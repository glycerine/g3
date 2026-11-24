"""Time complete DIMACS solver processes on a quiet CPU, rotating the engine order."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

from wide_clauses import clauses, check_output

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "ivy_apple" / "holdouts"))
from rotating_verification import idle, thread_siblings, ticks


def trial(binary, cpu, siblings, source, directory):
    stdout, stderr = directory / "stdout", directory / "stderr"
    before = ticks(siblings)
    with stdout.open("wb") as out, stderr.open("wb") as err:
        start = time.monotonic()
        with subprocess.Popen(["taskset", "-c", str(cpu), binary, "-st", "-dimacs", source],
                              stdout=out, stderr=err, env=dict(os.environ, GOMAXPROCS="1", GOGC="100")) as p:
            _, status, usage = os.wait4(p.pid, 0)
            p.returncode = os.waitstatus_to_exitcode(status)
        wall = time.monotonic() - start
    after = ticks(siblings)
    if p.returncode not in (0, 10) or stderr.stat().st_size:
        raise ValueError((binary, p.returncode, stderr.read_text()[:500]))
    sibling_idle = min((idle(before, after, c) for c in siblings), default=1)
    return stdout.read_text(), sibling_idle, wall, usage


def measure(width, count, solvers, cpus, rounds, max_probes, writer):
    models = set()
    reference = None
    observed = sorted({other for c, siblings in cpus.items() for other in [c, *siblings]})
    with tempfile.TemporaryDirectory() as tmp:
        directory = Path(tmp)
        source = directory / "input.cnf"
        with source.open("w") as f:
            print("p cnf", 1024, count, file=f)
            for row in clauses(1024, width, count):
                print(*row, 0, file=f)
        accepted = attempt = 0
        for _ in range(max_probes):
            before = ticks(observed)
            time.sleep(0.7)
            after = ticks(observed)
            free = {c: min(idle(before, after, member) for member in [c, *siblings])
                    for c, siblings in cpus.items()}
            cpu = max(free, key=free.get)
            if free[cpu] < 0.95:
                continue
            attempt += 1
            offset = accepted % len(solvers)
            for name, binary in solvers[offset:] + solvers[:offset]:
                text, sibling_idle, wall, usage = trial(binary, cpu, cpus[cpu], source, directory)
                lines = text.splitlines()
                if len(lines) < 2 or lines[0] != "sat":
                    raise ValueError((name, lines[:1]))
                model = lines[1]
                stats = check_output(text, "sat", 1024, () if model in models else clauses(1024, width, count))
                models.add(model)
                counters = stats["decisions"], stats.get("conflicts", 0), stats["mk-clause"]
                if reference is None:
                    reference = counters
                if counters != reference or counters[2] != count:
                    raise ValueError((name, counters, reference, count))
                occupancy = (usage.ru_utime + usage.ru_stime) / wall
                writer.writerow([width, count, accepted + 1, attempt, cpu, name, free[cpu], sibling_idle,
                                 occupancy, wall, usage.ru_utime, usage.ru_stime, usage.ru_maxrss, *counters])
                sys.stdout.flush()
                if min(sibling_idle, occupancy) < 0.95:
                    break
            else:
                accepted += 1
                if accepted == rounds:
                    return
        raise RuntimeError(f"only {accepted} of {rounds} complete quiet rotations for {width} x {count}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--solver", nargs=2, action="append", required=True, metavar=("NAME", "PATH"))
    parser.add_argument("--case", type=int, nargs=2, action="append", metavar=("WIDTH", "CLAUSES"))
    parser.add_argument("--cpu", type=int, action="append", help="candidate logical CPU, repeatable")
    parser.add_argument("--rounds", type=int, default=7)
    parser.add_argument("--max-probes", type=int, default=360)
    args = parser.parse_args()
    solvers = [(name, Path(path).resolve(strict=True)) for name, path in args.solver]
    cases = args.case or [(512, 16384), (64, 131072), (128, 32768), (16, 65536)]
    allowed = os.sched_getaffinity(0)
    cpus = {c: thread_siblings(c) for c in sorted(args.cpu or allowed)}
    if not cpus or cpus.keys() - allowed or len(dict(solvers)) != len(solvers):
        parser.error("supply available CPUs and distinct solver labels")
    if args.rounds < 1 or args.max_probes < 1 or any(not 1 <= w <= 1024 or n < 1 for w, n in cases):
        parser.error("use positive counts and clause widths between 1 and 1024")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow("width count round attempt cpu engine pre_idle sibling_idle occupancy wall user sys rss "
                    "decisions conflicts mk_clause".split())
    for width, count in cases:
        measure(width, count, solvers, cpus, args.rounds, args.max_probes, writer)


if __name__ == "__main__":
    main()
