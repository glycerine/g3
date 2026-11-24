"""Compare standalone SMT solvers on captured quantified verification goals."""

import argparse
import csv
import gzip
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time


FIXTURE = Path(__file__).resolve().parents[3] / "api/test_vectors/apple_enumeration_search.json.gz"


def query_text(fixture, index, form, budget):
    batch = fixture["Batches"][index - 1]
    query = batch["History"][-1] if form == "full" else batch["Core"]
    return (f"(set-option :rlimit {budget})\n"
            "(set-option :smt.macro_finder true)\n(set-option :smt.relevancy 1)\n"
            + fixture["Declarations"][query["Declarations"]]
            + fixture["Premises"][query["Premises"]]
            + "(push 1)\n" + query["Goal"] + "(check-sat)\n")


def measure(binary, text, cpu, perf, counters, answers=("unsat", "unknown")):
    command = ["taskset", "-c", str(cpu)] if cpu is not None else []
    if perf:
        command += [perf, "stat", "-e", "instructions:u,cycles:u", "-x,", "-o", counters, "--"]
    start = time.monotonic()
    result = subprocess.run(command + [binary, "-st", "-smt2", "-in"], input=text,
                            capture_output=True, text=True, env=dict(os.environ, GOMAXPROCS="1"),
                            check=True)
    wall = time.monotonic() - start
    lines = result.stdout.splitlines()
    if result.stderr or not lines or lines[0] not in answers or "(error " in result.stdout:
        raise ValueError((binary, result.stdout[:500], result.stderr[:500]))
    stats = {k: int(v) for k, v in re.findall(r":([\w-]+)\s+(\d+)(?=[\s)])", result.stdout)}
    hardware = {}
    if perf:
        with counters.open() as stream:
            for row in csv.reader(stream):
                if len(row) >= 5 and row[2] in ("instructions:u", "cycles:u") and float(row[4]) == 100:
                    hardware[row[2]] = int(row[0])
        if len(hardware) != 2:
            raise ValueError("performance counters missing or multiplexed")
    return (lines[0], stats.get("quant-instantiations", 0), stats.get("conflicts", 0),
            stats.get("decisions", 0), stats["rlimit-count"],
            hardware.get("instructions:u", ""), hardware.get("cycles:u", ""), wall)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--solver", action="append", nargs=2, metavar=("NAME", "BINARY"), required=True)
    parser.add_argument("--case", action="append", type=int, choices=range(1, 7), dest="cases")
    parser.add_argument("--form", choices=("full", "core"), default="full")
    parser.add_argument("--steps", type=int, default=6_000_000)
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--cpu", type=int)
    parser.add_argument("--perf", type=Path)
    parser.add_argument("--same-search", action="store_true",
                        help="require identical answer, instances, conflicts, decisions and resource count")
    args = parser.parse_args()
    if not 0 < args.steps <= 2**32 - 1 or args.rounds < 1:
        parser.error("steps must be a positive uint32 and rounds must be positive")
    with gzip.open(FIXTURE, "rt") as f:
        fixture = json.load(f)
    writer = csv.writer(sys.stdout)
    writer.writerow(["case", "form", "round", "solver", "answer", "instances", "conflicts",
                     "decisions", "resource", "instructions", "cycles", "seconds"])
    with tempfile.TemporaryDirectory() as directory:
        for index in args.cases or range(1, 7):
            text = query_text(fixture, index, args.form, args.steps)
            reference = None
            for i in range(args.rounds):
                offset = i % len(args.solver)
                for name, binary in args.solver[offset:] + args.solver[:offset]:
                    row = measure(binary, text, args.cpu, args.perf, Path(directory) / "counters")
                    if reference is None:
                        reference = row[:5]
                    if args.same_search and row[:5] != reference:
                        raise ValueError((index, name, row[:5], reference))
                    writer.writerow([index, args.form, i + 1, name, *row])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
