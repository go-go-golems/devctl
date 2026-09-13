# devctl Python subprocess runner

`devctl_runner.py` is the supported bounded subprocess helper for repository Python plugins. Vendor this file beside the plugin (or put this directory on `PYTHONPATH`) and import `Budget` and `Runner`. The file is standalone and uses only the Python standard library.

```python
from devctl_runner import Budget, Runner

runner = Runner()
runner.install_signal_handlers()
budget = Budget.from_deadline_ms(request["ctx"]["deadline_ms"])
result = runner.run(
    ["go", "build", "-o", "dist/server", "./cmd/server"],
    cwd=request["ctx"]["repo_root"],
    budget=budget,
    dry_run=request["ctx"].get("dry_run", False),
)
```

Reuse one `Budget` for all steps in a request. Child stdout and stderr are copied to the plugin's stderr so protocol stdout remains clean. Treat any of these as failure:

- `result.exit_code != 0`
- `result.timed_out`
- `result.canceled`
- `not result.cleanup_confirmed`

`install_signal_handlers()` converts SIGTERM/SIGINT during an active run into cancellation. It must be called from the main thread. The plugin remains responsible for noticing cancellation while idle and exiting its request loop.

The runner supports Unix process groups, not Windows. It can clean descendants that remain in the child process group; a child that deliberately creates a new session is outside this ownership boundary.

Run the support suite directly:

```sh
cd sdk/python
python3 -m unittest -v test_devctl_runner.py
```

The Go runtime suite invokes this test as well, so `go test ./...` checks the shipped helper.
