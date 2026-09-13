"""Supported bounded subprocess execution for devctl Python plugins.

The runner never uses a shell, routes child output to plugin stderr, owns a
separate process group, shares one monotonic budget across calls, and reaps the
child after timeout or cancellation. It is Linux/Unix-only because devctl's
plugin ownership contract currently relies on process groups.
"""

from __future__ import annotations

from collections import deque
from dataclasses import dataclass
import ctypes
import os
from pathlib import Path
import signal
import subprocess
import sys
import threading
import time
from typing import IO, Mapping, Sequence


@dataclass(frozen=True)
class RunResult:
    argv: tuple[str, ...]
    exit_code: int
    duration_ms: int
    timed_out: bool = False
    canceled: bool = False
    dry_run: bool = False
    cleanup_confirmed: bool = True
    error: str = ""
    stdout_tail: str = ""
    stderr_tail: str = ""


class Budget:
    """One monotonic deadline shared by every step in a request."""

    def __init__(self, deadline: float) -> None:
        self.deadline = deadline

    @classmethod
    def from_deadline_ms(cls, deadline_ms: int) -> "Budget":
        return cls(time.monotonic() + max(0, deadline_ms) / 1000.0)

    def remaining(self) -> float:
        return max(0.0, self.deadline - time.monotonic())


class _ByteTail:
    def __init__(self, limit: int) -> None:
        self._limit = max(0, limit)
        self._chunks: deque[bytes] = deque()
        self._size = 0
        self._lock = threading.Lock()

    def append(self, data: bytes) -> None:
        if self._limit == 0:
            return
        with self._lock:
            self._chunks.append(data)
            self._size += len(data)
            while self._size > self._limit and self._chunks:
                excess = self._size - self._limit
                first = self._chunks[0]
                if len(first) <= excess:
                    self._chunks.popleft()
                    self._size -= len(first)
                else:
                    self._chunks[0] = first[excess:]
                    self._size -= excess

    def text(self) -> str:
        with self._lock:
            return b"".join(self._chunks).decode("utf-8", errors="replace")


class Runner:
    def __init__(
        self,
        *,
        output: IO[str] = sys.stderr,
        tail_bytes: int = 16 * 1024,
        terminate_grace: float = 1.0,
    ) -> None:
        self.output = output
        self.tail_bytes = tail_bytes
        self.terminate_grace = terminate_grace
        self._cancel = threading.Event()
        self._write_lock = threading.Lock()
        self._signal: int | None = None

    def cancel(self) -> None:
        self._cancel.set()

    def install_signal_handlers(self) -> None:
        """Convert TERM/INT into runner cancellation; call from the main thread."""

        def request_cancel(signum: int, _frame: object) -> None:
            self._signal = signum
            self._cancel.set()

        signal.signal(signal.SIGTERM, request_cancel)
        signal.signal(signal.SIGINT, request_cancel)

    def run(
        self,
        argv: Sequence[str],
        *,
        cwd: str | os.PathLike[str],
        budget: Budget,
        env: Mapping[str, str] | None = None,
        dry_run: bool = False,
    ) -> RunResult:
        if not argv or any(not isinstance(value, str) or not value for value in argv):
            raise ValueError("argv must contain nonempty strings")
        directory = Path(cwd)
        if not directory.is_dir():
            raise ValueError(f"cwd is not a directory: {directory}")
        started = time.monotonic()
        if dry_run:
            return RunResult(tuple(argv), 0, 0, dry_run=True)
        if budget.remaining() <= 0:
            return RunResult(tuple(argv), 124, 0, timed_out=True)

        self._enable_child_subreaper()
        child_env = os.environ.copy()
        if env:
            child_env.update(env)
        try:
            process = subprocess.Popen(
                list(argv),
                cwd=directory,
                env=child_env,
                stdin=subprocess.DEVNULL,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                start_new_session=True,
            )
        except OSError as error:
            message = f"failed to start {argv[0]}: {error}"
            with self._write_lock:
                self.output.write(message + "\n")
                self.output.flush()
            return RunResult(
                tuple(argv), 127, int((time.monotonic() - started) * 1000),
                error=message, stderr_tail=message,
            )
        stdout_tail = _ByteTail(self.tail_bytes)
        stderr_tail = _ByteTail(self.tail_bytes)
        pumps = [
            threading.Thread(target=self._pump, args=(process.stdout, stdout_tail), daemon=True),
            threading.Thread(target=self._pump, args=(process.stderr, stderr_tail), daemon=True),
        ]
        for pump in pumps:
            pump.start()

        timed_out = False
        canceled = False
        termination_started: float | None = None
        kill_sent = False
        kill_started: float | None = None
        cleanup_confirmed = True
        while True:
            leader_exited = process.poll() is not None
            if leader_exited:
                self._reap_group(process.pid)
            group_alive = self._group_alive(process.pid)
            if leader_exited and not group_alive:
                break
            now = time.monotonic()
            if termination_started is None:
                timed_out = budget.remaining() <= 0
                canceled = self._cancel.is_set()
                orphaned_descendants = leader_exited and group_alive
                if timed_out or canceled or orphaned_descendants:
                    self._signal_group(process.pid, signal.SIGTERM)
                    termination_started = now
            elif not kill_sent and now - termination_started >= self.terminate_grace:
                self._signal_group(process.pid, signal.SIGKILL)
                kill_sent = True
                kill_started = now
            elif kill_sent and kill_started is not None and now - kill_started >= self.terminate_grace:
                cleanup_confirmed = False
                break
            time.sleep(0.01)

        try:
            exit_code = process.wait(timeout=self.terminate_grace)
        except subprocess.TimeoutExpired:
            cleanup_confirmed = False
            exit_code = -1
        for pump in pumps:
            pump.join(timeout=self.terminate_grace)
        duration_ms = int((time.monotonic() - started) * 1000)
        if self._signal is not None:
            canceled = True
        return RunResult(
            tuple(argv), exit_code, duration_ms,
            timed_out=timed_out, canceled=canceled, cleanup_confirmed=cleanup_confirmed,
            stdout_tail=stdout_tail.text(), stderr_tail=stderr_tail.text(),
        )

    def _pump(self, pipe: IO[bytes] | None, tail: _ByteTail) -> None:
        if pipe is None:
            return
        try:
            for chunk in iter(lambda: pipe.read(4096), b""):
                tail.append(chunk)
                with self._write_lock:
                    self.output.write(chunk.decode("utf-8", errors="replace"))
                    self.output.flush()
        finally:
            pipe.close()

    @staticmethod
    def _enable_child_subreaper() -> None:
        """Adopt orphaned descendants so this process can reap their zombies."""
        if not sys.platform.startswith("linux"):
            return
        libc = ctypes.CDLL(None, use_errno=True)
        pr_set_child_subreaper = 36
        if libc.prctl(pr_set_child_subreaper, 1, 0, 0, 0) != 0:
            error_number = ctypes.get_errno()
            raise OSError(error_number, os.strerror(error_number))

    @staticmethod
    def _reap_group(pgid: int) -> None:
        while True:
            try:
                pid, _status = os.waitpid(-pgid, os.WNOHANG)
            except ChildProcessError:
                return
            except InterruptedError:
                continue
            if pid <= 0:
                return

    @staticmethod
    def _group_alive(pgid: int) -> bool:
        try:
            os.killpg(pgid, 0)
            return True
        except ProcessLookupError:
            return False
        except PermissionError:
            return True

    @staticmethod
    def _signal_group(pgid: int, signum: int) -> None:
        try:
            os.killpg(pgid, signum)
        except ProcessLookupError:
            pass
