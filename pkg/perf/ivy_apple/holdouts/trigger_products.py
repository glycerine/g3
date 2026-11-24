"""Measure the effect of linking two independently matched groups in a multi-pattern."""

import argparse
import csv
import os
from pathlib import Path
import re
import subprocess


def query(width, mode, satisfiable):
    base = ["(R x a)", "(R y b)", "(R x c)"]
    extra = {"original": [], "R": ["(R y c)"], "S": ["(S b a)"],
             "both": ["(S b a)", "(R y c)"]}
    body = "(=> (and (R x a) (R y b) (S b a) (R x c) (R y c)) false)"
    if mode != "auto":
        body = f"(! {body} :pattern ({' '.join(base + extra[mode])}))"
    sources = [f"t{i}" for i in range(width)]
    values = [f"v{i}" for i in range(width + 1)]
    lines = ["(set-option :smt.macro_finder true)", "(declare-sort T 0)", "(declare-sort V 0)",
             "(declare-fun R (T V) Bool)", "(declare-fun S (V V) Bool)"]
    lines.extend(f"(declare-const {t} T)" for t in sources)
    lines.extend(f"(declare-const {v} V)" for v in values)
    lines.extend(f"(assert (distinct {' '.join(items)}))" for items in (sources, values))
    lines.append(f"(assert (forall ((x T)(y T)(a V)(b V)(c V)) {body}))")
    for i, t in enumerate(sources):
        lines.extend([f"(assert (R {t} {values[i+1]}))", f"(assert (R {t} v0))"])
    lines.append("(push 1)")
    lines.append("(assert (forall ((a V)(b V)) (not (S a b))))" if satisfiable
                 else f"(assert (S v{width} v{width-1}))")
    return "\n".join([*lines, "(check-sat)", ""])


def measure(args, directory, engine, binary, source, answer):
    counter = directory / (engine + ".perf")
    cmd = ["taskset", "-c", str(args.cpu), args.perf, "stat", "-x,", "-e", "instructions:u",
           "-o", counter, "--", binary, "-in", "-smt2", "-st"]
    p = subprocess.run(cmd, input=source, text=True, capture_output=True, timeout=args.timeout,
                       env=dict(os.environ, GOMAXPROCS="1", GOGC="100"))
    (directory / (engine + ".stdout")).write_text(p.stdout)
    (directory / (engine + ".stderr")).write_text(p.stderr)
    if p.returncode or p.stderr or p.stdout.splitlines()[:1] != [answer]:
        raise RuntimeError(f"unexpected solver result in {directory} ({engine})")
    stats = dict(re.findall(r":([\w-]+)\s+(\d+)\b", p.stdout))
    with counter.open() as f:
        ins, = [int(row[0]) for row in csv.reader(f)
                if len(row) > 4 and row[2] == "instructions:u" and float(row[4]) == 100]
    return ins, *(int(stats.get(k, 0)) for k in ("quant-instantiations", "conflicts", "rlimit-count"))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("g3", "native", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--perf", default="perf")
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--width", type=int, action="append")
    parser.add_argument("--timeout", type=int, default=60)
    args = parser.parse_args()
    widths = args.width or [8, 24, 64, 128]
    if args.rounds < 1 or min(widths) < 2 or args.timeout < 1:
        parser.error("use positive rounds and timeout, and widths of at least two")
    engines = [("g3", args.g3.resolve(strict=True)), ("native", args.native.resolve(strict=True))]
    args.output.mkdir(parents=True)
    with (args.output / "runs.csv").open("w", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(["width", "answer", "mode", "round", "engine", "instructions",
                         "instances", "conflicts", "rlimit"])
        for width in widths:
            for answer in ("sat", "unsat"):
                for mode in ("auto", "original", "R", "S", "both"):
                    directory = args.output / f"{width}-{answer}-{mode}"
                    directory.mkdir()
                    source = query(width, mode, answer == "sat")
                    (directory / "input.smt2").write_text(source)
                    for i in range(args.rounds):
                        run = directory / str(i + 1)
                        run.mkdir()
                        for engine, binary in engines[::1 if i % 2 == 0 else -1]:
                            data = measure(args, run, engine, binary, source, answer)
                            writer.writerow([width, answer, mode, i+1, engine, *data])
                            f.flush()


if __name__ == "__main__":
    main()
