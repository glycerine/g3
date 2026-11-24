"""Measure both solvers on original premise subsets captured during full Apple verification."""

import argparse
import csv
import io
from pathlib import Path
import sys
import tarfile
import tempfile

from enumeration_budget import measure


FIXTURE = Path(__file__).with_name("nov24-selected-premise-replay.tar.xz")
PARAMETERS = {"match": ("true", "false"), "macro": ("true", "true"), "default": ("false", "true")}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--solver", action="append", nargs=2, metavar=("NAME", "BINARY"), required=True)
    parser.add_argument("--case", action="append", type=int, choices=range(1, 34), dest="cases")
    parser.add_argument("--mode", action="append", choices=PARAMETERS, dest="modes")
    parser.add_argument("--steps", action="append", type=int)
    parser.add_argument("--rounds", type=int, default=1)
    parser.add_argument("--cpu", type=int)
    parser.add_argument("--perf", type=Path)
    parser.add_argument("--same-search", action="store_true",
                        help="require equal answers, instances, conflicts, decisions and resource counts")
    args = parser.parse_args()
    limits = args.steps or [20000, 50000, 100000, 250000, 1000000]
    if args.rounds < 1 or any(n < 1 or n > 2**32 - 1 for n in limits):
        parser.error("rounds must be positive and steps must fit a positive uint32")
    writer = csv.writer(sys.stdout)
    writer.writerow(["case", "batch", "goal", "mode", "steps", "round", "solver", "oracle", "answer",
                     "instances", "conflicts", "decisions", "resource", "instructions", "cycles", "seconds"])
    with tarfile.open(FIXTURE) as archive, tempfile.TemporaryDirectory() as directory:
        with io.TextIOWrapper(archive.extractfile("goals.csv")) as stream:
            cases = list(csv.DictReader(stream))
        for case in cases:
            index = int(case["ordinal"])
            if args.cases and index not in args.cases:
                continue
            raw = archive.extractfile(f"queries/{index:04d}.smt2").read().decode()
            for mode in args.modes or PARAMETERS:
                macros, mbqi = PARAMETERS[mode]
                for limit in limits:
                    text = (f"(set-option :smt.macro_finder {macros})(set-option :smt.mbqi {mbqi})"
                            f"(set-option :rlimit {limit})\n" + raw)
                    reference = None
                    for i in range(args.rounds):
                        offset = i % len(args.solver)
                        for name, binary in args.solver[offset:] + args.solver[:offset]:
                            row = measure(binary, text, args.cpu, args.perf, Path(directory) / "counters",
                                          answers=("unknown", case["oracle"]))
                            if reference is None:
                                reference = row[:5]
                            if args.same_search and row[:5] != reference:
                                raise ValueError((index, mode, limit, name, row[:5], reference))
                            writer.writerow([index, case["batch"], case["goal"], mode, limit, i + 1,
                                             name, case["oracle"], *row])
                            sys.stdout.flush()


if __name__ == "__main__":
    main()
