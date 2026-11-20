"""Alternate two g3 executables on the scope study's SMT-LIB inputs."""

import argparse
import csv
import re
import subprocess
import sys
import tempfile
import time
from pathlib import Path

STUDY = Path(__file__).resolve().parent
VECTORS = STUDY.parents[1] / "cmd/g3/test_vectors"
COUNTERS = ["decisions", "conflicts", "propagations", "binary-propagations", "restarts"]


def inputs(directory):
    cases = {}
    for size in [8, 9]:
        source = VECTORS / f"verbose_progress_pigeon{size}.smt2"
        if size == 8:
            cases["pigeon8-default"] = source
        direct = directory / f"pigeon{size}-smt.smt2"
        direct.write_text(source.read_text().replace("(check-sat)", "(check-sat-using smt)"))
        cases[direct.stem] = direct

    random = STUDY / "random180.smt2"
    cases["random180"] = random
    batch = directory / "random180-batch.smt2"
    batch.write_text("\n(reset)\n".join([random.read_text()] * 20))
    cases[batch.stem] = batch
    return cases


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("before", type=Path)
    parser.add_argument("after", type=Path)
    parser.add_argument("--cpu", type=int, default=6, help="Linux CPU used by taskset")
    parser.add_argument("--runs", type=int, default=7)
    parser.add_argument("--batch", action="store_true", help="run only the 20-solve control")
    args = parser.parse_args()
    programs = [("before", args.before.resolve()), ("after", args.after.resolve())]
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["sample", "case", "variant", "wall-ms", "answers", "answer", *COUNTERS])
    reference = {}
    with tempfile.TemporaryDirectory() as temp:
        cases = inputs(Path(temp))
        for sample in range(args.runs):
            for case, path in cases.items():
                if args.batch != case.endswith("-batch"):
                    continue
                for name, program in (reversed(programs) if sample % 2 else programs):
                    cmd = ["taskset", "-c", str(args.cpu), str(program), "-st", str(path)]
                    start = time.perf_counter_ns()
                    run = subprocess.run(cmd, capture_output=True, text=True, check=True)
                    ms = (time.perf_counter_ns() - start) / 1e6
                    answers = re.findall(r"^(?:sat|unsat|unknown)$", run.stdout, re.M)
                    expected = "sat" if case.startswith("random180") else "unsat"
                    if answers != [expected] * (20 if args.batch else 1):
                        raise ValueError(f"{case} produced unexpected answers: {answers}")
                    stats = dict(re.findall(r":([\w-]+)\s+([\w.]+)", run.stdout))
                    counts = [stats.get(k, "") for k in COUNTERS]
                    if not counts[0] or not counts[1] or not counts[2]:
                        raise ValueError(f"{case} did not report SMT search counts")
                    if reference.setdefault(case, counts) != counts:
                        raise ValueError(f"{case} changed search counts: {counts}")
                    writer.writerow([sample, case, name, round(ms, 3), len(answers), expected, *counts])


if __name__ == "__main__":
    main()
