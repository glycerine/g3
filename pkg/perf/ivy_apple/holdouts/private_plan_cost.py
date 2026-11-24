"""Measure how much of each speculative GoIvy proof goes to setup, assertion, and search."""

import argparse
import csv
import gzip
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "incremental_cost"))
from first_push_rewrite_rows import G3, build_checker, stage_patch


def observe(args, binary, rd):
    rd.mkdir()
    command = ["taskset", "-c", str(args.cpu), binary]
    if args.isolate:
        command.append("isolate=" + args.isolate)
    env = dict(os.environ, XTRACE_OFF="1", GOMAXPROCS="1", GOGC="100",
               PLAN_CAPTURE="1" if args.capture_inputs else "")
    p = subprocess.run([*command, args.spec], env=env, cwd=rd, capture_output=True)
    goal, ordinary = ("0", "0", "none"), bytearray()
    with gzip.open(rd / "plans.csv.gz", "wt", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(["batch", "goal", "stage", "ordinal", "kind", "selected", "limit", "used",
                         "proved", "verdict", "total", "setup", "assertion", "check"])
        ordinal = 0
        for line in p.stderr.splitlines(keepends=True):
            if line.startswith(b"QUERY "):
                _, batch, condition, stage = line.decode().split()
                goal = batch, condition, stage
            elif line.startswith(b"PLAN "):
                _, number, kind, selected, limit, used, proved, verdict, total, *parts = line.decode().split()
                stages = [int(v.strip("[]")) for v in parts]
                if int(number) != ordinal + 1 or len(stages) != 4 or stages[3] != 0:
                    raise ValueError(f"unexpected plan record {line!r}")
                ordinal = int(number)
                writer.writerow([*goal, number, kind, selected, limit, used, proved, verdict, total, *stages[:3]])
            else:
                ordinary.extend(line)
    if (p.returncode != args.expect_exit or p.stdout != (args.reference / "stdout").read_bytes() or
            ordinary != (args.reference / "stderr").read_bytes()):
        (rd / "stdout").write_bytes(p.stdout)
        (rd / "stderr").write_bytes(p.stderr)
        raise RuntimeError(f"verification output differed in {rd}")
    return ordinal


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for key in ("goivy", "spec", "reference", "output"):
        parser.add_argument("--" + key, type=Path, required=True)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--abc", type=Path)
    parser.add_argument("--isolate", default="")
    parser.add_argument("--rounds", type=int, default=2)
    parser.add_argument("--expect-exit", type=int, default=0)
    parser.add_argument("--capture-inputs", action="store_true",
                        help="Write the selected quantified premises and goal as SMT-LIB for each short plan.")
    args = parser.parse_args()
    if args.rounds < 1:
        parser.error("--rounds must be positive")
    # Source locations in GoIvy output include the spelling of the input path.
    args.spec = args.spec.absolute()
    args.output = args.output.resolve()
    args.goivy = args.goivy.resolve()
    args.output.mkdir(parents=True)
    subprocess.run(["go", "work", "init", G3 / "pkg", args.goivy], cwd=args.output,
                   env=dict(os.environ, GOWORK="off"), check=True)
    os.environ["GOWORK"] = str(args.output / "go.work")
    stage = args.output / "build"
    stage.mkdir()
    diff = gzip.decompress((HERE / "nov24-private-plan-observer.patch.gz").read_bytes()).decode()
    files = stage_patch(diff, args.goivy, stage)
    binary = build_checker(args.goivy, stage, files, args.abc)
    (stage / "build-info.txt").write_bytes(subprocess.check_output(["go", "version", "-m", binary]))
    for i in range(1, args.rounds + 1):
        print(f"Run {i} measured {observe(args, binary, args.output / str(i))} private plans", flush=True)


if __name__ == "__main__":
    main()
