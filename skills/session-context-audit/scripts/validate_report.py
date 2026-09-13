#!/usr/bin/env python3
"""Validate one or more session-context-audit JSON models without rendering."""

import argparse
import json
from pathlib import Path

from render_report import validate


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("reports", nargs="+", type=Path)
    args = parser.parse_args()
    for report in args.reports:
        model = json.loads(report.read_text(encoding="utf-8"))
        validate(model)
        print(f"JSON report valid: {report}")


if __name__ == "__main__":
    main()
