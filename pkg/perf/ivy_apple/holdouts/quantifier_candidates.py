"""Count E-matching candidates by quantified formula, generation and verification goal."""

import argparse
from collections import Counter
import csv
import gzip
import os
from pathlib import Path
import re
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "incremental_cost"))
from first_push_rewrite_rows import G3, build_checker, native_library, stage_patch


def prepare(args, engine, stage):
    stage.mkdir()
    experiment = "data256" if engine == "conditional" else "control"
    patch = gzip.decompress((HERE / f"nov24-quantifier-{experiment}.patch.gz").read_bytes()).decode()
    files = stage_patch(patch, args.goivy, stage)
    if engine == "native":
        native_library(args, stage, files, unit="qi_queue", counter=None)
    binary = build_checker(args.goivy, stage, files, args.abc, native=engine == "native")
    (stage / "build-info.txt").write_bytes(subprocess.check_output(["go", "version", "-m", binary]))
    return binary


def observe(args, binary, directory):
    directory.mkdir()
    command = ["taskset", "-c", str(args.cpu), binary]
    if args.isolate:
        command.append("isolate=" + args.isolate)
    command.append(args.spec)
    env = dict(os.environ, GOMAXPROCS="1", GOGC="100", XTRACE_OFF="1",
               G3_PERF_QUANTIFIER_GOALS=",".join(args.goal))
    p = subprocess.run(command, cwd=directory, capture_output=True, env=env)
    counters, names, ordinary = Counter(), {}, bytearray()
    for line in p.stderr.splitlines(keepends=True):
        if line.startswith(b"QUANT-NAME "):
            _, goal, number, formula = line.decode().rstrip("\r\n").split(" ", 3)
            names[goal, number] = formula
        elif line.startswith(b"QUANT-COUNT "):
            _, goal, number, stage, generation = line.decode().split()
            counters[goal, number, names[goal, number], stage, int(generation)] += 1
        else:
            ordinary.extend(line)
    if (p.returncode != args.expect_exit or p.stdout != (args.reference / "stdout").read_bytes()
            or ordinary != (args.reference / "stderr").read_bytes()):
        (directory / "stdout").write_bytes(p.stdout)
        (directory / "stderr").write_bytes(p.stderr)
        raise RuntimeError(f"verification output differed in {directory}")
    with gzip.open(directory / "observations.csv.gz", "wt", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(["goal", "ast_id", "quantifier", "stage", "generation", "count"])
        for key, n in sorted(counters.items()):
            writer.writerow([*key, n])
    for target in args.goal:
        by_stage = Counter()
        for (goal, _, _, stage, _), count in counters.items():
            if goal == target:
                by_stage[stage] += count
        if by_stage["candidate"] != sum(by_stage[s] for s in ("satisfied", "true", "instance")):
            raise RuntimeError(f"incomplete candidate counts for {target} in {directory}")
        print(directory.name, target, dict(by_stage), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", type=Path, required=True)
    parser.add_argument("--spec", type=Path, required=True)
    parser.add_argument("--reference", type=Path, required=True, help="Directory with expected stdout and stderr")
    parser.add_argument("--goal", required=True, action="append", help="One-based batch and condition, e.g. 5-69")
    parser.add_argument("--engine", action="append", choices=["g3", "conditional", "native"])
    parser.add_argument("--cpp-build", type=Path, default=G3 / "build")
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--rounds", type=int, default=2)
    parser.add_argument("--expect-exit", type=int, default=0)
    parser.add_argument("--isolate", default="")
    parser.add_argument("--abc", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.rounds < 1 or not all(re.fullmatch(r"[1-9][0-9]*-[1-9][0-9]*", g) for g in args.goal):
        parser.error("use positive rounds and one-based batch-condition pairs")
    args.spec = args.spec.absolute()
    for name in ("goivy", "reference", "cpp_build", "output"):
        setattr(args, name, getattr(args, name).resolve())
    args.output.mkdir(parents=True)
    subprocess.run(["go", "work", "init", G3 / "pkg", args.goivy], cwd=args.output,
                   env=dict(os.environ, GOWORK="off"), check=True)
    os.environ["GOWORK"] = str(args.output / "go.work")
    binaries = {e: prepare(args, e, args.output / ("build-" + e))
                for e in dict.fromkeys(args.engine or ["g3", "conditional", "native"])}
    for i in range(args.rounds):
        order = list(binaries)
        for e in order[i % len(order):] + order[:i % len(order)]:
            observe(args, binaries[e], args.output / f"{i+1}-{e}")


if __name__ == "__main__":
    main()
