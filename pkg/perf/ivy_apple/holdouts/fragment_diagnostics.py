"""Compare two complete GoIvy fragment checks after permuting source variable names."""

import argparse
import csv
import os
from pathlib import Path
import random
import re
import subprocess
import sys
import tempfile

CASES = [
    "doc/examples/liveness/1queue_bounded.ivy", "doc/examples/liveness/1queuelive.ivy",
    "doc/examples/liveness/2queuelive.ivy", "doc/examples/liveness/2queuelive_bounded.ivy",
    "doc/examples/liveness/2queuelive_noncomp.ivy", "doc/examples/liveness/liveness_refinement.ivy",
    "doc/examples/liveness/llqueue1.ivy", "doc/examples/liveness/llqueue2_bounded.ivy",
    "examples/ivy/bigger.ivy",
]
TOKEN = re.compile(r'"(?:[^"\\]|\\.)*"|#[^\n]*|\b_?[A-Z][a-zA-Z0-9_]*\b')
NAMES = ["Z", "A", "A_a", "V", "M", "A_b", "V_a", "Y", "Z_z0", "W", "B", "A_a_a", "X1", "X0"]


def permute(source, index):
    # Preserve the connection between X and its _X Skolem spelling.
    original = sorted({token for token in TOKEN.findall(source) if token[0].isupper()})
    choices = NAMES + [f"V_{i}" for i in range(max(0, len(original) - len(NAMES)))]
    random.Random(177107 + index).shuffle(choices)
    names = dict(zip(original, choices))

    def replace(match):
        token = match[0]
        if token.startswith("_"):
            return "_" + names.get(token[1:], token[1:])
        return names.get(token, token)

    return TOKEN.sub(replace, source)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("examples", "before", "after"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--rounds", type=int, default=12)
    args = parser.parse_args()
    if args.rounds < 1:
        parser.error("supply a positive number of rounds")
    examples = args.examples.resolve(strict=True)
    engines = [p.resolve(strict=True) for p in (args.before, args.after)]
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["input", "round", "before_status", "after_status", "stdout_equal", "stderr_equal", "fragment_error"])
    for case in CASES:
        spec = examples / case
        source = spec.read_text()
        with tempfile.TemporaryDirectory() as work:
            root = Path(work)
            for sibling in spec.parent.glob("*.ivy"):
                if sibling != spec:
                    (root / sibling.name).symlink_to(sibling)
            dirs = [root / label for label in ("before", "after")]
            for path in dirs:
                path.mkdir()
            edited = root / spec.name
            for i in range(args.rounds):
                edited.write_text(permute(source, i))
                before, after = [subprocess.run([exe, edited], cwd=directory, capture_output=True,
                                                env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"))
                                 for exe, directory in zip(engines, dirs)]
                same = before.stdout == after.stdout, before.stderr == after.stderr
                fragment_error = b"not in the fragment" in before.stderr
                writer.writerow([case, i + 1, before.returncode, after.returncode, *(int(x) for x in same),
                                 int(fragment_error)])
                if before.returncode != after.returncode or before.returncode == 0 or not all(same) or not fragment_error:
                    raise ValueError(f"changed fragment check in {case}, permutation {i + 1}")


if __name__ == "__main__":
    main()
