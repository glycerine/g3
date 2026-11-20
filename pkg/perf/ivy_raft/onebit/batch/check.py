"""Check public one-bit proof batches against generated, explicitly evaluated models."""

import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
sys.path.insert(0, str(HERE.parents[1] / "order_models"))
from finite_model import Problem, evaluate


GUARD = "(declare-fun witness_guard (A) Bool)(assert (forall ((x A)) (witness_guard x)))"
SQUARE = """(declare-const g3_witness_a Bool)(declare-const g3_witness_b Bool)
(assert (or g3_witness_a g3_witness_b))(assert (or g3_witness_a (not g3_witness_b)))
(assert (or (not g3_witness_a) g3_witness_b))(assert (or (not g3_witness_a) (not g3_witness_b)))"""


def run(args, work):
    overlay = work / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {
        str(ROOT / "api/study_boolean_witness_test.go"): str(HERE / "witness_test.go.txt")
    }}))
    driver = work / "witness.test"
    subprocess.run(["go", "test", "-c", f"-overlay={overlay}", "-o", driver, "./api"],
                   cwd=ROOT, check=True)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["first_seed", "models", "searched_refutations", "native_pairs"])
    for start in range(args.start, args.start + args.count, 250):
        rows = []
        native = 0
        for seed in range(start, min(start + 250, args.start + args.count)):
            m = Problem(seed, one_bit=True)
            if not all(evaluate(term, {}, m.widths, m.tables) for term in m.terms):
                raise ValueError(f"constructed model violates an assertion at seed {seed}")
            script = m.script() + "\n" + GUARD
            rows.append(json.dumps({"Seed": seed, "Script": script}))
            if args.native and seed % args.native_every == 0:
                check = subprocess.run([args.native, "-in", "-smt2"],
                                       input=script + "\n(check-sat)\n(push)\n" + SQUARE + "\n(check-sat)\n",
                                       text=True, capture_output=True, timeout=30, check=True)
                if check.stdout.splitlines() != ["sat", "unsat"]:
                    raise ValueError(f"native mismatch at seed {seed}: {check.stdout}")
                native += 1
        proc = subprocess.run([driver, "-test.run=^TestWitnessBooleanProofStream$", "-test.v"],
                              input="\n".join(rows), env=dict(os.environ, GOMAXPROCS="1"),
                              text=True, capture_output=True)
        result = re.search(r"verified (\d+) models and (\d+) searched refutations", proc.stdout)
        if proc.returncode or not result:
            raise RuntimeError(proc.stdout + proc.stderr)
        count, searched = map(int, result.groups())
        if count != len(rows) or searched != 2 * count:
            raise ValueError(f"incomplete scope check at seed {start}: {count}, {searched}")
        writer.writerow([start, count, searched, native])
        sys.stdout.flush()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--count", type=int, default=1000)
    parser.add_argument("--start", type=int, default=0)
    parser.add_argument("--native", type=Path, help="optional native Z3 executable")
    parser.add_argument("--native-every", type=int, default=523)
    args = parser.parse_args()
    if args.count < 1 or args.start < 0 or args.native_every < 1:
        parser.error("invalid count, start or sampling interval")
    with tempfile.TemporaryDirectory() as directory:
        run(args, Path(directory))


if __name__ == "__main__":
    main()
