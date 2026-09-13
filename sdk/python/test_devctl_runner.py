import errno
import io
import os
from pathlib import Path
import sys
import tempfile
import threading
import time
import unittest

from devctl_runner import Budget, Runner


class RunnerTest(unittest.TestCase):
    def test_dry_run_has_no_side_effect(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory, "marker")
            result = Runner(output=io.StringIO()).run(
                [sys.executable, "-c", f"open({str(marker)!r}, 'w').write('bad')"],
                cwd=directory,
                budget=Budget.from_deadline_ms(1000),
                dry_run=True,
            )
            self.assertTrue(result.dry_run)
            self.assertFalse(marker.exists())

    def test_budget_is_shared_across_steps(self):
        with tempfile.TemporaryDirectory() as directory:
            budget = Budget.from_deadline_ms(220)
            runner = Runner(output=io.StringIO(), terminate_grace=0.05)
            first = runner.run(
                [sys.executable, "-c", "import time; time.sleep(0.1)"],
                cwd=directory, budget=budget,
            )
            second = runner.run(
                [sys.executable, "-c", "import time; time.sleep(0.2)"],
                cwd=directory, budget=budget,
            )
            self.assertEqual(first.exit_code, 0)
            self.assertTrue(second.timed_out)
            self.assertLess(budget.remaining(), 0.02)

    def test_streams_output_and_bounds_diagnostic_tails(self):
        with tempfile.TemporaryDirectory() as directory:
            output = io.StringIO()
            result = Runner(output=output, tail_bytes=128).run(
                [sys.executable, "-c", "import sys; print('o' * 5000); print('e' * 5000, file=sys.stderr)"],
                cwd=directory, budget=Budget.from_deadline_ms(1000),
            )
            self.assertEqual(result.exit_code, 0)
            self.assertLessEqual(len(result.stdout_tail.encode()), 128)
            self.assertLessEqual(len(result.stderr_tail.encode()), 128)
            self.assertIn("ooo", output.getvalue())
            self.assertIn("eee", output.getvalue())

    def test_cancel_terminates_owned_process_group(self):
        with tempfile.TemporaryDirectory() as directory:
            runner = Runner(output=io.StringIO(), terminate_grace=0.05)
            timer = threading.Timer(0.08, runner.cancel)
            timer.start()
            try:
                result = runner.run(
                    [sys.executable, "-c", "import time; time.sleep(60)"],
                    cwd=directory, budget=Budget.from_deadline_ms(2000),
                )
            finally:
                timer.cancel()
            self.assertTrue(result.canceled)
            self.assertNotEqual(result.exit_code, 0)
            self.assertTrue(result.cleanup_confirmed)

    def test_cancel_removes_descendant(self):
        with tempfile.TemporaryDirectory() as directory:
            pid_file = Path(directory, "child.pid")
            code = (
                "import subprocess,time; "
                f"child=subprocess.Popen(['sleep','60']); open({str(pid_file)!r},'w').write(str(child.pid)); "
                "time.sleep(60)"
            )
            runner = Runner(output=io.StringIO(), terminate_grace=0.1)
            timer = threading.Timer(0.15, runner.cancel)
            timer.start()
            try:
                result = runner.run(
                    [sys.executable, "-c", code], cwd=directory,
                    budget=Budget.from_deadline_ms(2000),
                )
            finally:
                timer.cancel()
            self.assertTrue(result.canceled)
            child_pid = int(pid_file.read_text())
            deadline = time.monotonic() + 1.0
            while time.monotonic() < deadline:
                try:
                    os.kill(child_pid, 0)
                except OSError as error:
                    if error.errno == errno.ESRCH:
                        break
                time.sleep(0.01)
            else:
                self.fail(f"descendant {child_pid} remained alive")

    def test_nonexistent_executable_is_structured_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            result = Runner(output=io.StringIO()).run(
                ["definitely-not-a-devctl-test-executable"],
                cwd=directory, budget=Budget.from_deadline_ms(1000),
            )
            self.assertEqual(result.exit_code, 127)
            self.assertIn("failed to start", result.error)


if __name__ == "__main__":
    unittest.main()
