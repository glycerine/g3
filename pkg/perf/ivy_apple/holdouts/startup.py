"""Compare the first assertion on fresh strategic solvers, at two iteration counts."""

import argparse
import csv
import os
from pathlib import Path
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
PKG = HERE.parents[2]
sys.path.insert(0, str(HERE.parent))
import benchmark_slope


def native_run(binary, mode, count, a, counts):
    text = subprocess.check_output(["taskset", "-c", str(a.cpu), a.perf, "stat", "-x", ",", "-e", "instructions:u",
                                    "-o", counts, "--", binary, mode, str(count)], text=True)
    fields = text.split()
    if fields[:2] != ["BenchmarkStrategicSolverFirstAssertion/" + mode, str(count)] or fields[3:] != ["ns/op"]:
        raise ValueError(f"unexpected native output: {text}")
    values = [r for r in csv.reader(counts.read_text().splitlines()) if len(r) > 4 and r[2] == "instructions:u"]
    if len(values) != 1 or float(values[0][4]) != 100:
        raise ValueError("missing or multiplexed native counter")
    return int(values[0][0]), fields[2], "", ""


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", action="append", nargs=2, metavar=("LABEL", "API_TEST_BINARY"), required=True)
    p.add_argument("--library", type=Path, default=PKG.parent / "build/libz3.a")
    p.add_argument("--perf", default="perf")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=7)
    p.add_argument("--iterations", nargs=2, type=int, default=[3, 13], metavar=("LOW", "HIGH"))
    a = p.parse_args()
    low, high = a.iterations
    binaries = {n: Path(f).resolve(strict=True) for n, f in a.go}
    if len(binaries) != len(a.go) or "native" in binaries or not 0 < low < high or a.rounds < 1:
        p.error("select distinct Go builds, a positive round count and increasing positive iteration counts")
    library = a.library.resolve(strict=True)
    with tempfile.TemporaryDirectory() as tmp:
        native = Path(tmp) / "native"
        subprocess.run([os.environ.get("CXX", "c++"), "-std=c++17", "-O3", "-DNDEBUG",
                        "-I" + str(PKG.parent / "src/api"), PKG.parent / "src/test/perf/strategic_solver_startup.cpp",
                        library, "-fopenmp", "-lpthread", "-ldl", "-o", native], check=True)
        runs = [*binaries, "native"]
        counts = Path(tmp) / "counter"
        writer = csv.writer(sys.stdout, lineterminator="\n")
        writer.writerow(["round", "cpu", "mode", "engine", "low", "high", "low_instructions", "high_instructions",
                         "instructions_per_extra_op", "high_ns_per_op", "high_bytes_per_op", "high_allocs_per_op"])
        for i in range(a.rounds):
            start = i % len(runs)
            order = runs[start:] + runs[:start]
            for mode in ("Default", "MacroFinder"):
                for name in order if i % 2 == 0 else order[::-1]:
                    results = {}
                    for n in (low, high) if i % 2 == 0 else (high, low):
                        if name == "native":
                            results[n] = native_run(native, mode, n, a, counts)
                        else:
                            results[n] = benchmark_slope.run(binaries[name], "BenchmarkStrategicSolverFirstAssertion/" + mode,
                                                             n, a.cpu, a.perf, counts)
                    l, h = results[low][0], results[high][0]
                    writer.writerow([i+1, a.cpu, mode, name, low, high, l, h, round((h-l)/(high-low), 2),
                                     *results[high][1:]])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
