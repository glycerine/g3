#!/usr/bin/env python3
"""Verify ground refutations of two Raft obligations and time fresh SMT-LIB checks."""

import argparse
import csv
import gzip
import json
import os
from pathlib import Path
import resource
import re
import subprocess
import sys
import time


def check(exe, commands, cpu=None):
    cmd = [str(exe), "-in", "-smt2"]
    if cpu is not None:
        cmd = ["taskset", "-c", str(cpu), *cmd]
    result = subprocess.run(cmd, input=commands, text=True, capture_output=True,
                            env=dict(os.environ, GOMAXPROCS="1"), check=True, timeout=180)
    return result.stdout.splitlines()


def parts(case):
    lines = case["problem"].splitlines()
    declarations = "\n".join(s for s in lines if s.startswith("(declare-")) + "\n"
    assertions = [s for s in lines if s.startswith("(assert ")]
    return declarations, assertions


def verify(native, case):
    declarations, assertions = parts(case)
    # Each implication uses just its one originating assertion. A stronger
    # contradictory premise set could make this check pass vacuously.
    preamble = "(set-option :smt.macro_finder true)\n" + declarations
    checks = []
    for index, term in case["instances"]:
        if not 0 <= index < len(assertions) or re.search(r"\(\s*(forall|exists)\b", term):
            raise ValueError(f"invalid ground instance for parent {index}")
        checks.extend(["(push)", assertions[index], f"(assert (not {term}))",
                       "(check-sat)", "(pop)"])
    answers = check(native, preamble + "\n".join(checks))
    if answers != ["unsat"] * len(case["instances"]):
        raise ValueError(f"{case['obligation']} has an unproved ground instance: {answers}")
    ground = [f"(assert {t})" for _, t in case["instances"]]
    background = [a for a, (i, _) in zip(ground, case["instances"]) if i != len(assertions) - 1]
    for subset, expected in [(background, "sat"), (ground, "unsat")]:
        if check(native, preamble + "\n".join([*subset, "(check-sat)"])) != [expected]:
            raise ValueError(f"{case['obligation']} failed its {expected} control")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--native", type=Path, required=True)
    parser.add_argument("--g3", type=Path, help="also benchmark both engines after verification")
    parser.add_argument("--cpu", type=int)
    parser.add_argument("--rounds", type=int, default=8)
    parser.add_argument("--checks", type=int, default=20, help="resets and fresh checks per process")
    args = parser.parse_args()
    if args.checks < 1 or args.rounds < 1:
        parser.error("--checks and --rounds must be positive")
    native = args.native.resolve(strict=True)
    with gzip.open(Path(__file__).with_name("cases.json.gz"), "rt") as f:
        cases = json.load(f)
    for case in cases:
        verify(native, case)
        print(f"Verified Raft {case['obligation']} with {len(case['instances'])} ground assertions.",
              file=sys.stderr)
    if args.g3 is None:
        return
    if args.cpu is None:
        parser.error("--cpu is required for benchmarking")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "obligation", "engine", "problem", "cpu", "checks", "elapsed_s", "user_s", "sys_s"])
    for case in cases:
        decl, _ = parts(case)
        ground = decl + "".join(f"(assert {term})\n" for _, term in case["instances"])
        runs = [(name, exe, kind, problem) for name, exe in [("g3", args.g3.resolve(strict=True)), ("z3", native)]
                for kind, problem in [("full", case["problem"]), ("ground", ground), ("declarations", decl)]]
        for i in range(args.rounds):
            start = i % len(runs)
            order = runs[start:] + runs[:start]
            for name, exe, kind, problem in order:
                query = "(set-option :smt.macro_finder true)\n" + problem + "(check-sat)\n"
                batch = "(reset)\n".join([query] * args.checks)
                before = resource.getrusage(resource.RUSAGE_CHILDREN)
                t0 = time.monotonic()
                results = check(exe, batch, args.cpu)
                elapsed = time.monotonic() - t0
                after = resource.getrusage(resource.RUSAGE_CHILDREN)
                expected = "sat" if kind == "declarations" else "unsat"
                if results != [expected] * args.checks:
                    raise ValueError(f"{name} {kind} {case['obligation']}: {results[:5]}")
                writer.writerow([i + 1, case['obligation'], name, kind, args.cpu, args.checks,
                                 f"{elapsed:.6f}", f"{after.ru_utime-before.ru_utime:.6f}",
                                 f"{after.ru_stime-before.ru_stime:.6f}"])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
