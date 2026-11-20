"""Cross-check direct instance proofs after deleting, reordering or negating Raft assertions."""

import argparse
import csv
import gzip
import itertools
import json
import os
from pathlib import Path
import random
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
ONE_BIT = ROOT / "api/test_vectors/raft_order/onebit.jsonl.gz"
# Percent of premises retained, whether to negate the goal, and whether to shuffle.
TRIALS = [(100, False, False), (100, True, False), (90, False, True),
          (65, False, True), (30, False, True), (30, True, True),
          (8, False, True), (8, True, False), (0, False, False), (0, True, False)]


def commands(text):
    depth = start = 0
    quoted = ""
    comment = False
    i = 0
    while i < len(text):
        ch = text[i]
        i += 1
        if comment:
            comment = ch != "\n"
        elif quoted:
            if ch == quoted:
                if quoted == '"' and text[i:i+1] == '"':
                    i += 1
                else:
                    quoted = ""
        elif ch == ";":
            comment = True
        elif ch in {'"', '|'}:
            quoted = ch
        elif ch == "(":
            if depth == 0:
                start = i - 1
            depth += 1
        elif ch == ")":
            depth -= 1
            if depth < 0:
                raise ValueError("unbalanced closing parenthesis")
            if depth == 0:
                yield text[start:i]
    if depth or quoted:
        raise ValueError("unterminated SMT-LIB command")


def corpus(path):
    if path.is_dir():
        for f in sorted(path.glob("*.smt2")):
            yield f.name, f.read_text()
    else:
        with gzip.open(path, "rt") as source:
            for line in source:
                yield json.loads(line)


def variants(path, limit):
    for idx, (name, text) in enumerate(itertools.islice(corpus(path), limit), 1):
        declarations, assertions = [], []
        for form in commands(text):
            if form.startswith("(declare-"):
                declarations.append(form)
            elif form.startswith("(assert") and form[7].isspace():
                assertions.append(form)
            else:
                raise ValueError(f"{name}: unexpected command {form[:40]}")
        if not assertions:
            raise ValueError(f"{name}: missing assertions")
        for trial, (percent, negate, shuffle) in enumerate(TRIALS):
            rng = random.Random(212291 * idx + trial)
            premises = [v for v in assertions[:-1] if rng.randrange(100) < percent]
            if shuffle:
                rng.shuffle(premises)
            goal = assertions[-1]
            if negate:
                goal = "(assert (not " + goal[len("(assert"):-1].strip() + "))"
            yield idx, trial, len(premises), "\n".join([*declarations, *premises, goal])


def native_check(binary, script, milliseconds):
    query = f"(set-option :timeout {milliseconds})\n{script}\n(check-sat)\n"
    try:
        result = subprocess.run([binary, "-in", "-smt2"], input=query, capture_output=True,
                                text=True, timeout=milliseconds / 1000 + 12)
    except subprocess.TimeoutExpired:
        return "process_timeout"
    if result.returncode < 0:
        return f"native_signal_{-result.returncode}"
    result.check_returncode()
    lines = result.stdout.splitlines()
    if len(lines) != 1 or lines[0] not in {"sat", "unsat", "unknown"}:
        raise ValueError(f"unexpected native output {lines[-3:]}")
    return lines[0]


def replay(args, work):
    overlay = work / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {
        str(ROOT / "api/study_instance_perturb_test.go"): str(HERE / "perturb_test.go.txt")
    }}))
    driver = work / "perturb.test"
    subprocess.run(["go", "test", "-c", f"-overlay={overlay}", "-o", driver, "./api"],
                   cwd=ROOT, check=True)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["obligation", "trial", "premises", "applicable", "direct", "native", "retries"])
    rows = variants(args.input, args.limit)
    while batch := list(itertools.islice(rows, 32)):
        chunks = [json.dumps({"ID": f"{i}-{trial}", "Text": text}) for i, trial, _, text in batch]
        proc = subprocess.run([driver, "-test.run=^TestPerturbedInstanceProofs$", "-test.v"],
                              input="\n".join(chunks), env=dict(os.environ, GOMAXPROCS="1"),
                              text=True, capture_output=True, check=True)
        answers = dict((key, (app, direct)) for _, key, app, direct in
                       (s.split() for s in proc.stdout.splitlines() if s.startswith("PROBE ")))
        for i, trial, n, script in batch:
            app, direct = answers[f"{i}-{trial}"]
            result = native_check(args.native, script, 600)
            retry = direct == "true" and result not in {"sat", "unsat"}
            if retry:
                result = native_check(args.native, script, 10000)
            if direct == "true" and result != "unsat":
                raise ValueError(f"unconfirmed direct proof on {i}, trial {trial}: {result}\n{script}")
            writer.writerow([i, trial, n, app, direct, result, int(retry)])
        sys.stdout.flush()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", type=Path, default=ONE_BIT,
                        help="directory of numbered SMT2 files or gzip-compressed JSON lines")
    parser.add_argument("--native", type=Path, required=True)
    parser.add_argument("--limit", type=int, help="number of original obligations to transform")
    args = parser.parse_args()
    if args.limit is not None and args.limit < 1:
        parser.error("--limit must be positive")
    args.native = args.native.resolve()
    with tempfile.TemporaryDirectory() as tmp:
        replay(args, Path(tmp))


if __name__ == "__main__":
    main()
