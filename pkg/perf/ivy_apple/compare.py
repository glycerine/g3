#!/usr/bin/env python3
"""Compare two complete GoIvy checker runs on a specification or one isolate."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys
import time


EVENTS = ("instructions:u", "cycles:u", "branches:u", "branch-misses:u")


def measure(checker, args, directory):
    directory.mkdir(parents=True, exist_ok=False)
    resources = directory / "resources"
    checker_command = [
        "/usr/bin/time", "-f", "%U %S %M", "-o", str(resources),
        str(checker), *(["isolate=" + args.isolate] if args.isolate else []), str(args.spec),
    ]
    command = ["taskset", "-c", str(args.cpu)]
    if args.perf:
        counts = directory / "perf.csv"
        command.extend([args.perf, "stat", "-x", ",", "-e", ",".join(EVENTS), "-o", str(counts), "--"])
    command.extend(checker_command)
    with (directory / "stdout").open("wb") as out, (directory / "stderr").open("wb") as err:
        start = time.monotonic()
        result = subprocess.run(command, stdout=out, stderr=err, cwd=directory,
                                env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"))
        elapsed = time.monotonic() - start
    user, system, rss = resources.read_text().splitlines()[-1].split()
    counters = []
    if args.perf and result.returncode == 0:
        values = {row[2]: int(row[0]) for row in csv.reader(counts.read_text().splitlines())
                  if len(row) > 4 and row[2] in EVENTS and float(row[4]) == 100.0}
        if set(values) != set(EVENTS):
            raise ValueError(f"{directory}: missing or multiplexed hardware counter")
        counters = [values[event] for event in EVENTS]
    return result.returncode, (round(elapsed, 3), user, system, rss), counters


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--spec", type=Path, required=True)
    workload = p.add_mutually_exclusive_group()
    workload.add_argument("--isolate", default="dramc_nb2")
    workload.add_argument("--whole-spec", dest="isolate", action="store_const", const=None,
                          help="run GoIvy with its default isolate selection")
    p.add_argument("--reference", type=Path, help="require stdout to match this existing successful transcript")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--pairs", type=int, default=3)
    p.add_argument("--output", type=Path, required=True)
    p.add_argument("--reverse", action="store_true", help="start with the second checker")
    p.add_argument("--perf", metavar="EXECUTABLE", help="collect user-mode hardware counters with Linux perf")
    p.add_argument("--labels", nargs=2, metavar=("FIRST", "SECOND"), help="CSV names for the two checker builds")
    p.add_argument("checkers", nargs=2, type=Path, metavar="CHECKER")
    args = p.parse_args()
    if args.pairs < 1:
        p.error("--pairs must be positive")
    # GoIvy includes source filenames in the proof transcript. Keep the
    # caller's spelling of symlinked paths when comparing to a saved run.
    args.spec = args.spec.absolute() if args.reference else args.spec.resolve(strict=True)
    if not args.spec.is_file():
        p.error("the specification must be a file")
    checkers = [checker.resolve(strict=True) for checker in args.checkers]
    labels = args.labels or [checker.name for checker in checkers]
    if args.labels and len(set(labels)) != 2:
        p.error("--labels must be distinct")
    output = args.output.resolve()
    reference = (0, args.reference.read_bytes()) if args.reference else None
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["pair", "checker", "isolate", "cpu", "exit", "wall_s",
                     "user_s", "sys_s", "rss_KiB", "passes", "same_transcript", *(EVENTS if args.perf else ())])
    for pair in range(1, args.pairs + 1):
        order = (1, 0) if (pair % 2 == 0) != args.reverse else (0, 1)
        for idx in order:
            checker = checkers[idx]
            directory = output / f"{pair}-{idx}"
            status, metrics, counters = measure(checker, args, directory)
            transcript = (directory / "stdout").read_bytes()
            if reference is None:
                reference = (status, transcript)
            same = (status, transcript) == reference
            lines = transcript.splitlines()
            writer.writerow([pair, labels[idx], args.isolate or args.spec.stem, args.cpu, status,
                             *metrics, lines.count(b"PASS"), int(same), *counters])
            sys.stdout.flush()
            if status or not lines or lines[-1].strip() != b"OK" or not same:
                raise SystemExit(f"{directory}: checker failed, output incomplete, or transcripts differ")


if __name__ == "__main__":
    main()
