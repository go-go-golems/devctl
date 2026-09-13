#!/usr/bin/env bash
set -euo pipefail
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
if (($# == 0)); then
  set -- "$here/fixtures/minimal.json"
fi
python3 "$here/validate_report.py" "$@"
python3 "$here/test_report_model.py" -v
