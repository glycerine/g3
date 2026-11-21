#!/usr/bin/env python3
"""Compare standalone g3 and Z3 on captured, ground-goal Apple verification conditions."""

import argparse
import csv
import gzip
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


COUNTERS = ("decisions", "conflicts", "quant-instantiations", "rlimit-count")
EVENTS = ("instructions:u", "cycles:u", "branches:u", "branch-misses:u")


def main():
    vectors = Path(__file__).resolve().parent / "test_vectors"
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--case", action="append", choices=sorted(x.name.split(".")[0] for x in vectors.glob("*.smt2.gz")))
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=5)
    p.add_argument("--perf", type=Path, help="optional Linux perf executable for hardware counters")
    p.add_argument("solvers", nargs="+", type=Path)
    args = p.parse_args()
    if args.rounds < 1:
        p.error("--rounds must be positive")
    solvers = [s.resolve(strict=True) for s in args.solvers]
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "case", "solver", "cpu", "wall_s", "user_s", "sys_s",
                     "rss_KiB", *COUNTERS, *(EVENTS if args.perf else ())])
    with tempfile.TemporaryDirectory() as tmp:
        directory = Path(tmp)
        for case in args.case or ["dramc_nb2-71", "dramc_nb2-63", "dramc_nb2-369", "cf_live-130"]:
            script = directory / "query.smt2"
            with gzip.open(vectors / (case + ".smt2.gz"), "rb") as source:
                script.write_bytes(source.read())
            for i in range(args.rounds):
                offset = i % len(solvers)
                for solver in solvers[offset:] + solvers[:offset]:
                    stats = directory / "resources"
                    perf_stats = directory / "hardware"
                    cmd = ["taskset", "-c", str(args.cpu)]
                    if args.perf:
                        cmd += [str(args.perf.resolve()), "stat", "-x", ",", "-e",
                                ",".join(EVENTS), "-o", str(perf_stats), "--"]
                    cmd += ["/usr/bin/time", "-f", "%e %U %S %M", "-o", str(stats), str(solver), str(script)]
                    run = subprocess.run(cmd, cwd=directory, env=dict(os.environ, GOMAXPROCS="1"),
                                         stdout=subprocess.PIPE, text=True, check=True)
                    if run.stdout.splitlines()[:1] != ["unsat"] or "(error" in run.stdout:
                        raise SystemExit(f"{solver} did not prove {case}: {run.stdout[:250]}")
                    numbers = dict(re.findall(r":([\w-]+)\s+(\d+)(?=[)\s])", run.stdout))
                    hardware = []
                    if args.perf:
                        with perf_stats.open() as source:
                            events = {r[2]: int(r[0]) for r in csv.reader(source) if len(r) > 2 and r[2] in EVENTS}
                        hardware = [events[e] for e in EVENTS]
                    writer.writerow([i + 1, case, solver.name, args.cpu, *stats.read_text().split(),
                                     *[int(numbers.get(k, 0)) for k in COUNTERS], *hardware])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
