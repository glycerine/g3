"""Time GoIvy backends and optionally Python Ivy on the same Raft proof transcript."""

import argparse
import csv
import io
from pathlib import Path
import subprocess
import sys

from measure import EVENTS


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--g3", type=Path, required=True, help="GoIvy built with g3")
    parser.add_argument("--native", type=Path, required=True, help="GoIvy built with classic_z3")
    parser.add_argument("--python-ivy", type=Path, help="also run this ivy_check with the active Python")
    parser.add_argument("--reference", type=Path, required=True, help="Python Ivy Raft transcript")
    parser.add_argument("--output", type=Path, required=True, help="new directory for per-run logs")
    parser.add_argument("--rounds", type=int, default=5, help="number of runs of each backend")
    parser.add_argument("--cpu", type=int, help="pin both checkers and use one Go CPU")
    parser.add_argument("--perf", metavar="EXECUTABLE", help="collect user-mode hardware counters")
    parser.add_argument("specification", type=Path)
    args = parser.parse_args()
    if args.rounds < 1:
        parser.error("--rounds must be positive")
    args.output.mkdir(parents=True, exist_ok=False)
    command = [sys.executable, Path(__file__).with_name("measure.py"), "--reference", args.reference]
    if args.cpu is not None:
        command.extend(["--cpu", str(args.cpu)])
    if args.perf:
        command.extend(["--perf", args.perf])
    engines = [("g3", [args.g3.resolve()]), ("native", [args.native.resolve()])]
    if args.python_ivy:
        engines.append(("python", [sys.executable, "-O", args.python_ivy.resolve()]))
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "engine", "cpu", "wall_s", "user_s", "sys_s", "rss_KiB", "passes",
                     *(EVENTS if args.perf else ())])
    for i in range(args.rounds):
        start = i % len(engines)
        order = engines[start:] + engines[:start]
        if len(engines) > 2 and i // len(engines) % 2:
            order.reverse()
        for name, checker in order:
            log = args.output / f"{i+1:02}-{name}.log"
            result = subprocess.run([*command, "--sample", str(i + 1), "--label", name,
                                     "--output", log, "--", *checker, args.specification],
                                    stdout=subprocess.PIPE, text=True, check=True)
            row, = csv.reader(io.StringIO(result.stdout))
            writer.writerow(row)
            sys.stdout.flush()


if __name__ == "__main__":
    main()
