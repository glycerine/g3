#!/usr/bin/env python3
"""Check public and ground Boolean proof paths on the known models from verify.py."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cases", required=True, type=Path, help="output directory from verify.py")
    parser.add_argument("--output", required=True, type=Path, help="CSV results")
    args = parser.parse_args()
    here = Path(__file__).resolve().parent
    pkg = here.parents[3]
    with tempfile.TemporaryDirectory() as temp:
        overlay = Path(temp) / "overlay.json"
        replace = {str(pkg / "api" / "ground_boolean_finite_test.go"): str(here / "ground_boolean_test.go.txt")}
        overlay.write_text(json.dumps({"Replace": replace}))
        subprocess.run(["go", "test", "-pgo=off", "-overlay=" + str(overlay),
                        "./api", "-run", "^TestFiniteGroundBooleans$", "-count=1"],
                       cwd=pkg, check=True, env=dict(os.environ, G3_FINITE_CASES=str(args.cases.resolve()),
                                                   G3_FINITE_RESULTS=str(args.output.resolve())))


if __name__ == "__main__":
    main()
