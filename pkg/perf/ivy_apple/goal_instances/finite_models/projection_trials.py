#!/usr/bin/env python3
"""Check public and selected ground proof paths on the known models from verify.py."""
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
    goals = parser.add_mutually_exclusive_group()
    goals.add_argument("--constant-goals", action="store_true", help="exercise the short universal premise selection")
    goals.add_argument("--ground-relations", action="store_true", help="exercise the single-sort relation plan")
    args = parser.parse_args()
    here = Path(__file__).resolve().parent
    pkg = here.parents[3]
    with tempfile.TemporaryDirectory() as temp:
        overlay = Path(temp) / "overlay.json"
        if args.constant_goals:
            source, test = "short_premise_test.go.txt", "TestFiniteShortPremises"
        elif args.ground_relations:
            source, test = "ground_relations_test.go.txt", "TestFiniteGroundRelations"
        else:
            source, test = "ground_boolean_test.go.txt", "TestFiniteGroundBooleans"
        replace = {str(pkg / "api" / "ground_finite_test.go"): str(here / source)}
        overlay.write_text(json.dumps({"Replace": replace}))
        subprocess.run(["go", "test", "-pgo=off", "-overlay=" + str(overlay),
                        "./api", "-run", "^" + test + "$", "-count=1"],
                       cwd=pkg, check=True, env=dict(os.environ, G3_FINITE_CASES=str(args.cases.resolve()),
                                                   G3_FINITE_RESULTS=str(args.output.resolve())))


if __name__ == "__main__":
    main()
