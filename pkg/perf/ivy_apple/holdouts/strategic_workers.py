"""Measure live memory and peak process RSS with independent, initialized API solvers."""

import argparse
import csv
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

from startup import PKG


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", nargs=2, action="append", required=True, metavar=("LABEL", "API_TEST_BINARY"))
    p.add_argument("--incremental", action="append", default=[], metavar="GO_LABEL",
                   help="use NewSolverForIncrementalChecks for this --go label")
    p.add_argument("--library", type=Path, default=PKG.parent / "build/libz3.a")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=3)
    p.add_argument("--workers", nargs="+", choices=[1, 16, 128, 1024], type=int, default=[1, 16, 128])
    a = p.parse_args()
    engines = {name: Path(b).resolve(strict=True) for name, b in a.go}
    if len(engines) != len(a.go) or "native" in engines or a.rounds < 1 or len(set(a.workers)) != len(a.workers):
        p.error("supply distinct labels and worker counts and a positive round count")
    if set(a.incremental) - {name for name, _ in a.go}:
        p.error("--incremental requires a matching --go label")
    with tempfile.TemporaryDirectory() as tmp:
        directory = Path(tmp)
        native = directory / "native"
        subprocess.run([os.environ.get("CXX", "c++"), "-std=c++17", "-O3", "-DNDEBUG", "-I" + str(PKG.parent / "src/api"),
                        PKG.parent / "src/test/perf/strategic_solver_workers.cpp", a.library.resolve(strict=True),
                        "-fopenmp", "-lpthread", "-ldl", "-o", native], check=True)
        engines["native"] = native
        labels = list(engines)
        writer = csv.writer(sys.stdout, lineterminator="\n")
        writer.writerow(["round", "cpu", "workers", "engine", "initialization_ns", "live_bytes_per_worker", "peak_rss_KiB"])
        for i in range(a.rounds):
            j = i % len(labels)
            order = labels[j:] + labels[:j]
            for n in a.workers:
                for engine in order if i % 2 == 0 else order[::-1]:
                    if engine == "native":
                        cmd = [native, str(n)]
                    else:
                        prefix = "BenchmarkIncrementalStrategicSolver" if engine in a.incremental else "BenchmarkStrategicSolver"
                        cmd = [engines[engine], "-test.run", "^$", "-test.bench", f"^{prefix}Workers$/^Workers{n}$", "-test.benchtime=1x"]
                    rss = directory / "peak"
                    raw = subprocess.check_output(["taskset", "-c", str(a.cpu), "/usr/bin/time", "-f", "%M", "-o", rss, *cmd],
                                                  cwd=directory, text=True, env=dict(os.environ, GOMAXPROCS="1"))
                    row = re.search(rf"Workers{n}(?:-\d+)?\s+1\s+([.\d]+) ns/op\s+([.\d]+) live-B/worker", raw)
                    if row is None:
                        raise ValueError(f"missing measurement {engine}, {n}: {raw}")
                    writer.writerow([i+1, a.cpu, n, engine, *row.groups(), int(rss.read_text())])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
