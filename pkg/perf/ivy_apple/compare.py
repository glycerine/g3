#!/usr/bin/env python3
"""Run two GoIvy builds on an Apple isolate and compare the entire transcript."""

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
        str(checker), "isolate=" + args.isolate, str(args.spec),
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
                  if len(row) > 2 and row[2] in EVENTS}
        counters = [values[event] for event in EVENTS]
    return result.returncode, (round(elapsed, 3), user, system, rss), counters


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--spec", type=Path, required=True)
    p.add_argument("--isolate", default="dramc_nb2")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--pairs", type=int, default=3)
    p.add_argument("--output", type=Path, required=True)
    p.add_argument("--reverse", action="store_true", help="start with the second checker")
    p.add_argument("--perf", metavar="EXECUTABLE", help="collect user-mode hardware counters with Linux perf")
    p.add_argument("checkers", nargs=2, type=Path, metavar="CHECKER")
    args = p.parse_args()
    if args.pairs < 1:
        p.error("--pairs must be positive")
    args.spec = args.spec.resolve(strict=True)
    checkers = [checker.resolve(strict=True) for checker in args.checkers]
    output = args.output.resolve()
    reference = None
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
            writer.writerow([pair, checker.name, args.isolate, args.cpu, status,
                             *metrics, lines.count(b"PASS"), int(same), *counters])
            sys.stdout.flush()
            if status or not lines or lines[-1].strip() != b"OK" or not same:
                raise SystemExit(f"{directory}: checker failed, output incomplete, or transcripts differ")


if __name__ == "__main__":
    main()
