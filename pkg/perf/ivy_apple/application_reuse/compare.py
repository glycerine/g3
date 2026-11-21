"""Compare short AST application construction in g3 and the in-tree C++ solver."""

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

ITERATIONS = 200_000
SINGLE = re.compile(r"BenchmarkASTCanonicalApplication/([1-4])(?:-\d+)?$")
ROTATING = re.compile(r"BenchmarkASTApplicationReuse/([1-4])/Resident(0|192|256)(?:-\d+)?$")


def measure(binary, native, cpu):
    cmd = ["taskset", "-c", str(cpu), binary]
    if not native:
        cmd += ["-test.run", "^$", "-test.bench", "^BenchmarkAST(CanonicalApplication|ApplicationReuse)$",
                f"-test.benchtime={ITERATIONS}x"]
    out = subprocess.check_output(cmd, env=dict(os.environ, GOMAXPROCS="1"), text=True)
    rows = {}
    if native:
        for r in csv.DictReader(io.StringIO(out)):
            key = r["workload"], int(r["arity"]), int(r["resident"])
            rows[key] = int(r["iterations"]), float(r["ns_per_op"])
    else:
        for line in out.splitlines():
            if not line.startswith("Benchmark"):
                continue
            name, n, time, unit, *_ = line.split()
            single, rotating = SINGLE.fullmatch(name), ROTATING.fullmatch(name)
            if single:
                key = "single", int(single[1]), 1
            elif rotating:
                key = "rotating", int(rotating[1]), int(rotating[2])
            else:
                raise ValueError(line)
            if unit != "ns/op":
                raise ValueError(line)
            rows[key] = int(n), float(time)
    expected = {("single", a, 1) for a in range(1, 5)}
    expected.update(("rotating", a, n) for a in range(1, 5) for n in (0, 192, 256))
    if rows.keys() != expected or any(n != ITERATIONS for n, _ in rows.values()):
        raise ValueError(f"incomplete application benchmark output from {binary}")
    return rows


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", action="append", nargs=2, required=True, metavar=("LABEL", "TEST_BINARY"))
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=9)
    p.add_argument("--build", type=Path, default=ROOT / "build")
    args = p.parse_args()
    binaries = {k: Path(v).resolve(strict=True) for k, v in args.go}
    if len(binaries) != len(args.go) or "native" in binaries or args.rounds < 1:
        p.error("choose unique Go labels and positive rounds")
    with tempfile.TemporaryDirectory() as scratch:
        native = Path(scratch) / "native"
        build_native(args.build.resolve(strict=True), native, ROOT / "src/test/perf/ast_application_reuse.cpp")
        binaries["native"] = native
        names = list(binaries)
        writer = csv.writer(sys.stdout, lineterminator="\n")
        writer.writerow(["round", "cpu", "variant", "workload", "arity", "resident", "iterations", "ns_per_op"])
        for i in range(args.rounds):
            offset = i % len(names)
            order = names[offset:] + names[:offset]
            if len(names) > 2 and i % 2:
                order.reverse()
            for name in order:
                for key, values in measure(binaries[name], name == "native", args.cpu).items():
                    writer.writerow([i+1, args.cpu, name, *key, *values])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
