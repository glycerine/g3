"""Compare every named proof decision in two completed negative Raft runs."""

import argparse
from pathlib import Path
import re


def decisions(stdout, stderr):
    rows = []
    action = obligation = ""
    lines = stdout.read_text().splitlines() + stderr.read_text().splitlines()
    for line in lines:
        line = line.strip()
        if line.startswith(("when ", "in action ")):
            action = line
        if line.endswith("..."):
            obligation = re.sub(r"\S*/examples/raft/", "", line)
        elif line in ("PASS", "FAIL"):
            if not obligation:
                raise ValueError(f"unnamed {line} in {stdout}")
            rows.append((action, obligation, line))
            obligation = ""
    failures = sum(row[2] == "FAIL" for row in rows)
    summaries = [line.strip() for line in lines if line.strip().startswith("error: failed checks:")]
    if len(rows) != 419 or failures == 0 or summaries != [f"error: failed checks: {failures}"]:
        raise ValueError(f"incomplete negative transcript in {stdout}")
    return rows


def compare(reference, actual):
    for number, (expected, observed) in enumerate(zip(reference, actual), 1):
        if expected != observed:
            raise ValueError(f"obligation {number} expected {expected}, observed {observed}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("reference_stdout", "reference_stderr", "candidate_stdout", "candidate_stderr"):
        parser.add_argument(name, type=Path)
    args = parser.parse_args()
    reference = decisions(args.reference_stdout, args.reference_stderr)
    compare(reference, decisions(args.candidate_stdout, args.candidate_stderr))
    print(f"{len(reference)} named outcomes agree, including {sum(x[2] == 'FAIL' for x in reference)} failures")
