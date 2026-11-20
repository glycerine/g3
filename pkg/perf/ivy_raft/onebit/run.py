"""Replay 419 all-one-bit Raft obligations with general SAT and a checked CDCL prototype."""

import argparse
import csv
import gzip
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
sys.path.insert(0, str(HERE.parent / "order_models"))
from finite_model import Problem, evaluate


def build(work, truth_tables):
    order = ROOT / "api/internal/order"
    files = {}
    for source in HERE.glob("*.go.txt"):
        name = source.name.removesuffix(".txt")
        package = ROOT / "api" if name in {"replay_test.go", "residual_test.go"} else order
        files[str(package / ("onebit_study_" + name))] = str(source)
    overlay = work / "overlay.json"
    overlay.write_text(json.dumps({"Replace": files}))
    go = ["go", "test", f"-overlay={overlay}"]
    if truth_tables:
        subprocess.run([*go, "./api/internal/order", "-run", "^TestStudy", "-count=1"],
                       cwd=ROOT, check=True, stdout=sys.stderr)
    driver = work / "onebit.test"
    subprocess.run([*go, "-c", "-o", driver, "./api"], cwd=ROOT, check=True)
    return driver


def raft(work):
    folder = work / "raft"
    folder.mkdir()
    index = 0
    with gzip.open(ROOT / "api/test_vectors/raft_order/onebit.jsonl.gz", "rt") as source:
        for index, line in enumerate(source, 1):
            name, script = json.loads(line)
            if name != f"{index:03}.smt2":
                raise ValueError(f"unexpected obligation {name}")
            (folder / name).write_text(script)
    if index != 419:
        raise ValueError(f"expected 419 obligations, got {index}")
    return folder


def witnesses(work, start, count):
    guard = "(declare-fun witness_guard (A) Bool)\n(assert (forall ((x A)) (witness_guard x)))\n"
    clash = "(assert (exists ((x A)) (not (witness_guard x))))\n"
    sat, unsat = work / "witnesses", work / "contradictions"
    sat.mkdir()
    unsat.mkdir()
    for seed in range(start, start + count):
        model = Problem(seed, one_bit=True)
        if not all(evaluate(q, {}, model.widths, model.tables) for q in model.terms):
            raise ValueError(f"constructed model violates an assertion at seed {seed}")
        script = model.script() + "\n" + guard
        name = f"{seed:08}.smt2"
        (sat / name).write_text(script)
        (unsat / name).write_text(script + clash)
    return [(sat, 1, count), (unsat, -1, count)]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--rounds", type=int, default=1)
    parser.add_argument("--methods", nargs="+", choices=["sat", "positive", "phase", "restart"],
                        default=["sat", "positive"])
    parser.add_argument("--models", type=int, default=0, help="add this many SAT/UNSAT witness pairs")
    parser.add_argument("--start", type=int, default=0)
    parser.add_argument("--truth-tables", action="store_true", help="also check 70,000 small CNFs")
    parser.add_argument("--bench-residual", type=int, metavar="N",
                        help="time N full SAT searches over the precompiled Raft corpus")
    args = parser.parse_args()
    if args.bench_residual is not None and args.bench_residual < 1:
        parser.error("--bench-residual requires a positive iteration count")
    if args.rounds < 1 or args.models < 0 or args.start < 0:
        parser.error("invalid round, count or seed")
    with tempfile.TemporaryDirectory() as directory:
        work = Path(directory)
        driver = build(work, args.truth_tables)
        if args.bench_residual is not None:
            subprocess.run([driver, "-test.run=^$", "-test.bench=^BenchmarkRaftBooleanResidualSearch$",
                            f"-test.benchtime={args.bench_residual}x", "-test.cpu=1"],
                           cwd=ROOT / "api", check=True)
            return
        corpora = [(raft(work), -1, 419)]
        if args.models:
            corpora.extend(witnesses(work, args.start, args.models))
        writer = csv.writer(sys.stdout, lineterminator="\n")
        wrote_header = False
        for i in range(args.rounds):
            j = i % len(args.methods)
            for mode in args.methods[j:] + args.methods[:j]:
                for corpus, expected, count in corpora:
                    out = work / "measurements.csv"
                    env = dict(os.environ, GOMAXPROCS="1", ONEBIT_METHOD=mode,
                               ONEBIT_INPUT=str(corpus), ONEBIT_OUTPUT=str(out), ONEBIT_EXPECT=str(expected))
                    subprocess.run([driver, "-test.run=^TestReplayOneBit$", "-test.timeout=0"],
                                   check=True, env=env, stdout=sys.stderr)
                    with out.open() as results:
                        reader = csv.reader(results)
                        header = next(reader)
                        if not wrote_header:
                            wrote_header = True
                            writer.writerow(["round", "corpus", *header])
                        rows = 0
                        for row in reader:
                            writer.writerow([i + 1, corpus.name, *row])
                            rows += 1
                        if rows != count:
                            raise ValueError(f"{corpus.name} produced {rows} of {count} answers")


if __name__ == "__main__":
    main()
