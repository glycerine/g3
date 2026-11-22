"""Check every integer-region answer from the captured GoIvy model checks with Z3."""

import argparse
import gzip
import json
from pathlib import Path
import subprocess


def check_batch(binary, rows, start):
    commands = []
    expected = []
    for _, body, violated, witness in rows:
        commands.extend(["(reset)", "(set-logic QF_LIA)", "(declare-const audit_x Int)",
                         "(assert (not " + body + "))"])
        if violated:
            value = str(witness) if witness >= 0 else f"(- {-witness})"
            commands.append(f"(assert (= audit_x {value}))")
        commands.append("(check-sat)")
        expected.append("sat" if violated else "unsat")

    result = subprocess.run([binary, "-in", "-smt2"], input="\n".join(commands) + "\n",
                            text=True, capture_output=True, check=True)
    answers = result.stdout.splitlines()
    if len(answers) != len(expected):
        raise ValueError(f"batch starting at {start} returned {len(answers)} rows: {result.stdout[:200]}")
    for i, (got, want) in enumerate(zip(answers, expected), start):
        if got != want:
            raise ValueError(f"query {i}: {got}, expected {want}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--native", type=Path, required=True)
    parser.add_argument("--input", type=Path, default=Path(__file__).resolve().parents[1]
                        / "test_vectors/numeric_auxiliary/integer-regions.jsonl.gz")
    args = parser.parse_args()
    total = witnesses = 0
    batch = []
    with gzip.open(args.input, "rt") as stream:
        for line in stream:
            row = json.loads(line)
            batch.append(row)
            witnesses += row[2]
            total += 1
            if len(batch) == 32:
                check_batch(args.native.resolve(), batch, total - len(batch) + 1)
                batch.clear()
    if batch:
        check_batch(args.native.resolve(), batch, total - len(batch) + 1)
    print(f"Verified {total - witnesses} universal results and {witnesses} integer witnesses")


if __name__ == "__main__":
    main()
