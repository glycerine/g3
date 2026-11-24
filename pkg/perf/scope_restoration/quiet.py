"""Compare prebuilt scope or AST-lifetime benchmarks on a quiet Linux core."""

import argparse
import csv
import os
from pathlib import Path
import re
import resource
import subprocess
import sys
import time

PKG = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(PKG / "perf/ivy_apple/holdouts"))
from rotating_verification import idle, thread_siblings, ticks


def child_seconds():
    usage = resource.getrusage(resource.RUSAGE_CHILDREN)
    return usage.ru_utime + usage.ru_stime


def measurements(output, lifetime, native, n):
    if lifetime:
        pattern = r"^BenchmarkScopedBooleanLifetimes/(\d+)/(\w+)\s+(\d+)\s+([\d.]+) ns/op"
        rows = re.findall(pattern, output, re.M)
    else:
        rows = []
        for line in output.splitlines():
            m = re.match(r"^BenchmarkSMTScopeRestoration(WithQueue)?/(ScopedAtoms|SurvivingAtoms)\s+(\d+)", line)
            if m:
                metric = "pop-ns/op" if m[1] else "ns/op"
                value = re.search(r"([\d.]+) " + metric, line)[1]
                rows.append(("WithQueue" if m[1] or native else "make", m[2], m[3], value))
    if len(rows) != (12 if lifetime else 2 if native else 4) or any(int(r[2]) != n for r in rows):
        raise ValueError("missing cases or incorrect iteration count")
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", action="append", nargs=2, required=True, metavar=("LABEL", "SMTCORE_TEST"))
    parser.add_argument("--native", type=Path, help="in-tree test-z3 for the scope benchmark")
    parser.add_argument("--lifetimes", action="store_true")
    parser.add_argument("--cpu", type=int, action="append", help="candidate CPU (repeatable)")
    parser.add_argument("--output", required=True, type=Path, help="new output directory")
    parser.add_argument("--rounds", type=int, default=9)
    parser.add_argument("--iterations", type=int)
    parser.add_argument("--max-probes", type=int, default=180, help="maximum two-second idle checks")
    args = parser.parse_args()
    n = args.iterations if args.iterations is not None else 10 if args.lifetimes else 10_000
    binaries = {label: Path(path).resolve(strict=True) for label, path in args.go}
    if len(binaries) != len(args.go) or "native" in binaries or any(
            not re.fullmatch(r"[A-Za-z0-9_-]+", label) for label in binaries):
        parser.error("supply distinct Go labels composed of letters, digits, underscores or dashes")
    if args.lifetimes and args.native:
        parser.error("the lifetime fixture runs in the Go test binary")
    if args.native:
        binaries["native"] = args.native.resolve(strict=True)
    if min(args.rounds, n, args.max_probes) < 1:
        parser.error("round, iteration and probe counts must be positive")
    allowed = os.sched_getaffinity(0)
    cpus = args.cpu or sorted(c for c in allowed if c == min({c, *thread_siblings(c)} & allowed))
    if not set(cpus) <= allowed:
        parser.error("selected CPU is outside the process affinity mask")
    groups = {c: [c, *thread_siblings(c)] for c in cpus}
    all_cpus = set().union(*groups.values())
    args.output.mkdir(parents=True)
    names = list(binaries)
    completed = 0
    with (args.output / "attempts.csv").open("w") as f:
        writer = csv.writer(f)
        writer.writerow(["round", "probe", "engine", "cpu", "before_idle", "sibling_idle",
                         "occupancy", "wall_s", "fixture", "case", "iterations", "ns"])
        for probe in range(1, args.max_probes + 1):
            before = ticks(all_cpus)
            time.sleep(2)
            after = ticks(all_cpus)
            idle_rates = {c: min(idle(before, after, k) for k in group) for c, group in groups.items()}
            quiet = [c for c in cpus if idle_rates[c] >= .95]
            if not quiet:
                continue
            cpu = max(quiet, key=lambda c: (idle_rates[c], (c + probe) % (max(cpus) + 1)))
            offset = completed % len(names)
            for name in names[offset:] + names[:offset]:
                command = ["taskset", "-c", str(cpu), binaries[name]]
                if name == "native":
                    command += ["smt_scope_restoration", str(n), "Both"]
                else:
                    bench = "^BenchmarkScopedBooleanLifetimes$" if args.lifetimes else "^BenchmarkSMTScopeRestoration(WithQueue)?$"
                    command += ["-test.run=^$", "-test.cpu=1", "-test.bench=" + bench,
                                f"-test.benchtime={n}x", "-test.count=1"]
                first = ticks(groups[cpu])
                used = child_seconds()
                start = time.perf_counter()
                result = subprocess.run(command, cwd=PKG / "smtcore", check=True, text=True,
                                        capture_output=True, env=dict(os.environ, GOMAXPROCS="1", GOGC="100"))
                wall = time.perf_counter() - start
                occupancy = (child_seconds() - used) / wall
                last = ticks(groups[cpu])
                sibling = min((idle(first, last, c) for c in groups[cpu][1:]), default=1.)
                (args.output / f"{probe}-{name}.log").write_text(result.stdout + result.stderr)
                for row in measurements(result.stdout, args.lifetimes, name == "native", n):
                    writer.writerow([completed + 1, probe, name, cpu, idle_rates[cpu], sibling, occupancy, wall, *row])
                f.flush()
                if min(occupancy, sibling) < .95:
                    break
            else:
                completed += 1
                if completed == args.rounds:
                    return
    raise SystemExit(f"{completed} of {args.rounds} complete quiet rounds")


if __name__ == "__main__":
    main()
