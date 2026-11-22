"""Measure solver-thread work in complete independent Ivy specs, including expected failures."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
sys.path.insert(0, str(HERE.parent / "incremental_cost"))
from holdout_examples import EXAMPLES
import mutation_costs
import thread_checks


def measure(directory, checker, spec, failures, reference, cpu, phase):
    directory.mkdir()
    env = dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1", IVY_THREAD_EVENT="1")
    with (directory / "stdout").open("wb") as out, (directory / "stderr").open("wb") as err:
        result = subprocess.run(["taskset", "-c", str(cpu), checker, spec], stdout=out, stderr=err,
                                cwd=directory, env=env)
    if result.returncode != int(failures > 0) or (directory / "stdout").read_bytes() != reference:
        raise ValueError(f"{directory}: verification transcript differs from linked Z3")
    prefix = "IVY-THREAD-CHECK " if phase == "calls" else "IVY-MUTATION "
    rows = []
    lines = (directory / "stderr").read_text().splitlines()
    if failures and [s for s in lines if s.startswith("error: failed checks:")] != [f"error: failed checks: {failures}"]:
        raise ValueError(f"{directory}: unexpected failure diagnostic")
    for line in lines:
        if line.startswith(prefix):
            fields = line.removeprefix(prefix).split()
            op = "Check" if phase == "calls" else fields.pop(0)
            seq, count, empty, enabled, running, wall = map(int, fields)
            if running != enabled or running <= 0 or count <= 0:
                raise ValueError(f"{directory}: invalid or multiplexed solver-thread counter")
            rows.append((seq, op, count, empty, enabled, running, wall))
    rows.sort()
    if not rows or [r[0] for r in rows] != list(range(1, len(rows) + 1)):
        raise ValueError(f"{directory}: incomplete call sequence")
    return rows


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for option in ("goivy", "examples", "references", "output"):
        p.add_argument("--" + option, type=Path, required=True)
    p.add_argument("--example", nargs="+", choices=EXAMPLES, default=list(EXAMPLES))
    p.add_argument("--phase", choices=["calls", "mutations"], required=True)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--pairs", type=int, default=3)
    p.add_argument("--modfile", type=Path)
    p.add_argument("--go-overlay", type=Path, help="optional Go overlay for g3")
    args = p.parse_args()
    if args.pairs < 1 or len(set(args.example)) != len(args.example):
        p.error("select distinct examples and a positive pair count")
    args.goivy = args.goivy.resolve(strict=True)
    args.examples = args.examples.absolute()
    args.output = args.output.resolve()
    args.output.mkdir(parents=True)
    if args.go_overlay:
        args.go_overlay = args.go_overlay.resolve(strict=True)
    if args.phase == "calls":
        binaries = {}
        observer = (thread_checks.HERE / "thread_counter.go.txt").read_text()
        for engine in ("g3", "native"):
            directory = args.output / engine
            directory.mkdir()
            binaries[engine] = thread_checks.build(engine, args, directory, observer, thread_checks.PACKAGES,
                                                    go_overlay=args.go_overlay if engine == "g3" else None)
    else:
        binaries = {engine: mutation_costs.build(args, engine) for engine in ("g3", "native")}
    with (args.output / "counts.csv").open("w") as f:
        writer = csv.writer(f, lineterminator="\n")
        writer.writerow(["example", "round", "cpu", "engine", "sequence", "operation", "count", "empty",
                         "enabled_ns", "running_ns", "wall_ns"])
        for name in args.example:
            relative, passes, failures = EXAMPLES[name]
            reference = (args.references / (name + ".out")).read_bytes()
            lines = reference.splitlines()
            if (lines.count(b"PASS"), lines.count(b"FAIL")) != (passes, failures):
                raise ValueError(f"wrong reference for {name}")
            sequence = None
            for i in range(args.pairs):
                for engine in list(binaries)[::1 if i % 2 == 0 else -1]:
                    rows = measure(args.output / f"{name}-{i+1}-{engine}", binaries[engine],
                                   args.examples / relative, failures, reference, args.cpu, args.phase)
                    observed = [r[1] for r in rows]
                    if sequence is not None and observed != sequence:
                        raise ValueError(f"{name}: solver operations differ")
                    sequence = observed
                    writer.writerows((name, i+1, args.cpu, engine, *r) for r in rows)
                    f.flush()


if __name__ == "__main__":
    main()
