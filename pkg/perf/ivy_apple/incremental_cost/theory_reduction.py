"""Compare the Go and C++ theory reducers with many calls per hardware reading.

Supply a Go test executable built from pkg/ast/rewriter and the native
src/test/perf/theory_reduction.cpp executable linked to the in-tree libz3.a.
The CSV includes complete process counters at two iteration counts. Their
difference removes fixed process setup and gives an uninstrumented loop slope.
"""

import argparse
import csv
import io
import os
from pathlib import Path
import re
import subprocess
import tempfile


CASES = ("UF", "Not", "DoubleNot", "Or", "OrDuplicate", "Iff")
MODES = ("Core", "Blank")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", type=Path, required=True)
    parser.add_argument("--native", type=Path, required=True)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--perf", default="perf")
    parser.add_argument("--rounds", type=int, default=5)
    parser.add_argument("--low", type=int, default=100000)
    parser.add_argument("--high", type=int, default=600000)
    parser.add_argument("--output", required=True, type=Path)
    a = parser.parse_args()
    if not 0 < a.low < a.high or a.rounds < 1:
        parser.error("supply positive rounds and 0 < low < high")
    env = dict(os.environ, GOMAXPROCS="1", GOGC="100", LC_ALL="C")
    with tempfile.TemporaryDirectory() as tmp, a.output.open("x", newline="") as out:
        counter = Path(tmp) / "counter.csv"
        writer = csv.writer(out)
        writer.writerow(("round", "cpu", "engine", "case", "mode", "iterations",
                         "ns_per_op", "instructions", "counter_running_percent"))
        for i in range(a.rounds):
            cases = CASES[i % len(CASES):] + CASES[:i % len(CASES)]
            for case in cases:
                for mode in (MODES if i % 2 == 0 else MODES[::-1]):
                    engines = ("g3", "native") if i % 2 == 0 else ("native", "g3")
                    for engine in engines:
                        for n in ((a.low, a.high) if i % 2 == 0 else (a.high, a.low)):
                            if engine == "g3":
                                cmd = [a.go.resolve(), "-test.run=^$", "-test.cpu=1",
                                       f"-test.bench=^BenchmarkTheoryReduction$/^{case}$/^{mode}$",
                                       f"-test.benchtime={n}x"]
                            else:
                                cmd = [a.native.resolve(), case, mode, str(n)]
                            perf_cmd = ["taskset", "-c", str(a.cpu), a.perf, "stat",
                                        "-e", "instructions:u", "-x,", "-o", counter, "--"]
                            res = subprocess.run(perf_cmd + cmd, check=True, env=env,
                                                 text=True, stdout=subprocess.PIPE)
                            if engine == "g3":
                                pattern = rf"^BenchmarkTheoryReduction/{case}/{mode}\s+{n}\s+([.\d]+) ns/op"
                                ns = re.search(pattern, res.stdout, re.MULTILINE)
                                if not ns:
                                    raise ValueError(res.stdout)
                                elapsed = ns[1]
                            else:
                                row, = csv.DictReader(io.StringIO(res.stdout))
                                if (row["case"], row["mode"], row["iterations"]) != (case, mode, str(n)):
                                    raise ValueError(res.stdout)
                                elapsed = row["ns_per_op"]
                            with counter.open() as f:
                                event, = [r for r in csv.reader(f) if len(r) >= 5 and r[2] == "instructions:u"]
                            writer.writerow((i + 1, a.cpu, engine, case, mode, n, elapsed,
                                             int(event[0]), float(event[4])))
                            out.flush()


if __name__ == "__main__":
    main()
