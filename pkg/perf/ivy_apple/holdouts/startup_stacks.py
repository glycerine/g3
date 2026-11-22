"""Sample instructions during repeated initialization and release of fresh strategic solvers."""
import argparse
from collections import Counter
import csv
import gzip
import os
from pathlib import Path
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
PKG = HERE.parents[2]
sys.path.insert(0, str(HERE.parent))
from stack_costs import samples


def profile(engine, mode, binary, root, a):
    fixture = "BenchmarkStrategicSolverFirstAssertion/" + mode
    if engine == "native":
        command = [binary, mode, str(a.iterations)]
        graph = "dwarf,8192"
    else:
        command = [binary, "-test.run=^$", "-test.bench=^BenchmarkStrategicSolverFirstAssertion$/^" + mode + "$",
                   "-test.benchtime=" + str(a.iterations) + "x"]
        graph = "fp"
    data = root / "perf.data"
    record = subprocess.run(["taskset", "-c", str(a.cpu), a.perf, "record", "--no-buildid", "-m", str(a.pages),
                             "-e", "instructions:u", "-c", str(a.period), "--call-graph", graph, "-o", data,
                             "--", *command], capture_output=True, text=True, env=dict(os.environ, GOMAXPROCS="1"))
    if record.returncode or fixture not in record.stdout:
        raise RuntimeError(f"{engine} {fixture} failed\n{record.stdout}\n{record.stderr}")
    trace = root / "stacks.gz"
    decoded = subprocess.run([a.perf, "script", "--no-inline", "-i", data, "-F", "event,ip,sym,dso"],
                             capture_output=True, check=True)
    if b"lost" in decoded.stderr.lower() or "lost" in record.stderr.lower():
        raise ValueError(f"{engine} {mode} lost samples; increase --pages or --period")
    with gzip.open(trace, "wb", compresslevel=1) as stream:
        stream.write(decoded.stdout)
    leaf, inclusive = Counter(), Counter()
    for stack in samples(trace):
        leaf[stack[0] if stack else "[no stack]"] += 1
        inclusive.update(set(stack))
    if not leaf:
        raise ValueError(f"no instruction samples for {engine} {mode}")
    return leaf, inclusive


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", type=Path, required=True, help="compiled g3 api test binary")
    p.add_argument("--library", type=Path, default=PKG.parent / "build/libz3.a")
    p.add_argument("--perf", default="perf")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=3)
    p.add_argument("--iterations", type=int, default=80)
    p.add_argument("--period", type=int, default=1200011)
    p.add_argument("--pages", type=int, default=512, help="power-of-two perf ring size in pages")
    a = p.parse_args()
    if min(a.rounds, a.iterations, a.period, a.pages) < 1 or a.pages & (a.pages - 1):
        p.error("counts must be positive and --pages must be a power of two")
    go = a.go.resolve(strict=True)
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        native = root / "native"
        subprocess.run([os.environ.get("CXX", "c++"), "-O3", "-std=c++17", "-DNDEBUG", "-I" + str(PKG.parent / "src/api"),
                        PKG.parent / "src/test/perf/strategic_solver_startup.cpp", a.library.resolve(strict=True),
                        "-fopenmp", "-lpthread", "-ldl", "-o", native], check=True)
        writer = csv.writer(sys.stdout, lineterminator="\n")
        writer.writerow(["round", "mode", "engine", "cpu", "period", "samples", "attribution", "symbol", "count"])
        engines = [("g3", go), ("native", native)]
        for i in range(a.rounds):
            for mode in ("Default", "MacroFinder"):
                for engine, binary in engines if i % 2 == 0 else engines[::-1]:
                    leaf, inclusive = profile(engine, mode, binary, root, a)
                    count = sum(leaf.values())
                    for kind, values in (("leaf", leaf), ("inclusive", inclusive)):
                        writer.writerows([i + 1, mode, engine, a.cpu, a.period, count, kind, name, n]
                                         for name, n in values.most_common())
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
