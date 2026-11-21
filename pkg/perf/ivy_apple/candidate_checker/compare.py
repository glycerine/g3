"""Compare quantified candidate checks in this C++ fork and chosen g3 test binaries."""

import argparse
import csv
import io
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from native_benchmark import ROOT, build_native

GO_ROW = re.compile(r"BenchmarkSMTQuantifierCandidate/Width(4|16)/(First|Last)/(Checker|RegisteredChecker|Queue)(?:-\d+)?$")
ITERATIONS = 100_000


def measure(binary, native, cpu):
    cmd = ["taskset", "-c", str(cpu), binary]
    if not native:
        cmd.extend(["-test.run", "^$", "-test.bench", "^BenchmarkSMTQuantifierCandidate$",
                    f"-test.benchtime={ITERATIONS}x"])
    out = subprocess.check_output(cmd, env=dict(os.environ, GOMAXPROCS="1"), text=True)
    rows = {}
    if native:
        for r in csv.DictReader(io.StringIO(out)):
            key = int(r["width"]), r["position"], "Checker"
            rows[key] = int(r["iterations"]), float(r["ns_per_op"])
    else:
        for line in out.splitlines():
            if not line.startswith("Benchmark"):
                continue
            name, n, ns, unit, *rest = line.split()
            match = GO_ROW.fullmatch(name)
            if not match or unit != "ns/op":
                raise ValueError(line)
            rows[int(match[1]), match[2], match[3]] = int(n), float(ns)
    entries = {"Checker"} if native else {"Checker", "Queue"}
    if not native and any(entry == "RegisteredChecker" for _, _, entry in rows):
        entries.add("RegisteredChecker")
    expected = {(width, pos, entry) for width in (4, 16) for pos in ("First", "Last") for entry in entries}
    if rows.keys() != expected or any(n != ITERATIONS for n, _ in rows.values()):
        raise ValueError(f"incomplete benchmark output from {binary}")
    return rows


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", action="append", nargs=2, required=True, metavar=("LABEL", "TEST_BINARY"))
    p.add_argument("--build", type=Path, default=ROOT / "build", help="in-tree native Makefile build")
    p.add_argument("--cpu", required=True, type=int)
    p.add_argument("--rounds", type=int, default=11)
    args = p.parse_args()
    variants = {name: Path(binary).resolve(strict=True) for name, binary in args.go}
    if args.rounds < 1 or "native" in variants or len(variants) != len(args.go):
        p.error("supply distinct Go labels and a positive round count")
    with tempfile.TemporaryDirectory() as scratch:
        native = Path(scratch) / "native"
        build_native(args.build.resolve(strict=True), native, ROOT / "src/test/perf/quantifier_candidate.cpp")
        variants["native"] = native
        names = list(variants)
        writer = csv.writer(sys.stdout, lineterminator="\n")
        writer.writerow(["round", "cpu", "variant", "width", "position", "entry", "iterations", "ns_per_op"])
        for i in range(args.rounds):
            offset = i % len(names)
            order = names[offset:] + names[:offset]
            if len(names) > 2 and i % 2:
                order.reverse()
            for name in order:
                for key, timing in measure(variants[name], name == "native", args.cpu).items():
                    writer.writerow([i+1, args.cpu, name, *key, *timing])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
