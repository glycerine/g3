"""Exercise the public and forced private proof on previously certified finite models."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import csv
import json
import os
from pathlib import Path
import subprocess
from tempfile import TemporaryDirectory

HERE = Path(__file__).resolve().parent
PKG = HERE.parents[2]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("models", type=Path, help="output directory from finite_models/verify.py")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--jobs", type=int, default=2)
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error("--jobs must be positive")
    models = args.models.resolve()
    with (models / "results.csv").open() as f:
        rows = list(csv.DictReader(f))
    with TemporaryDirectory() as tmp:
        work = Path(tmp)
        target = PKG / "api" / "witness_finite_trial_test.go"
        overlay = work / "overlay.json"
        overlay.write_text(json.dumps({"Replace": {str(target): str(HERE / "finite_trial_test.go.txt")}}))
        binary = work / "finite.test"
        subprocess.run(["go", "test", "-pgo=off", "-overlay=" + str(overlay), "-c", "-o", binary, "./api"],
                       cwd=PKG, check=True)

        def run(row):
            case, seed, width = (row[k] for k in ("case", "seed", "sort_size"))
            if row["g3_certificate"] != "sat" or row["z3_certificate"] != "sat":
                raise ValueError(f"missing finite model for {case} {seed} {width}")
            src = models / f"{case}-{seed}-{width}" / "source.smt2"
            proc = subprocess.run([binary, "-test.v", "-test.run=^TestKnownFiniteWitnessModel$"],
                                  cwd=PKG / "api", env={**os.environ, "G3_WITNESS_MODEL": str(src)},
                                  stdout=subprocess.PIPE, text=True, timeout=120)
            if proc.returncode:
                raise ValueError(f"{src}\n{proc.stdout}")
            values = next(line.split()[1:] for line in proc.stdout.splitlines() if line.startswith("FINITE "))
            return case, seed, width, *values

        with ThreadPoolExecutor(args.jobs) as pool, args.output.open("x") as f:
            writer = csv.writer(f, lineterminator="\n")
            writer.writerow(["case", "seed", "sort_size", "assertions", "binder_sorts", "admitted", "forced_refutation"])
            writer.writerows(pool.map(run, rows))


if __name__ == "__main__":
    main()
