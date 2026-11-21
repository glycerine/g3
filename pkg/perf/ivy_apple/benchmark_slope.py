"""Measure the incremental instruction cost of a Go benchmark at two iteration counts."""
import argparse
import csv
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


def run(binary, fixture, iterations, cpu, perf, counts):
    pattern = "/".join("^" + re.escape(part) + "$" for part in fixture.split("/"))
    text = subprocess.check_output(
        ["taskset", "-c", str(cpu), perf, "stat", "-e", "instructions:u", "-x", ",", "-o", counts, "--",
         binary, "-test.run=^$", "-test.bench=" + pattern, f"-test.benchtime={iterations}x", "-test.benchmem"],
        env=dict(os.environ, GOMAXPROCS="1"), text=True)
    rows = [line.split() for line in text.splitlines() if line.startswith("Benchmark")]
    if (len(rows) != 1 or rows[0][0].split("-")[0] != fixture or int(rows[0][1]) != iterations
            or not text.rstrip().endswith("PASS")):
        raise ValueError(f"unexpected output from {binary}: {text}")
    with counts.open() as source:
        events = [r for r in csv.reader(source) if len(r) > 2 and r[2] == "instructions:u"]
    if len(events) != 1 or float(events[0][4]) != 100:
        raise ValueError(f"missing or multiplexed instructions in {counts}")
    fields = rows[0]
    return (int(events[0][0]), *(fields[fields.index(k)-1] for k in ("ns/op", "B/op", "allocs/op")))


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", action="append", nargs=2, metavar=("LABEL", "TEST_BINARY"), required=True)
    p.add_argument("--fixture", action="append", required=True, help="one exact benchmark name, including subcase")
    p.add_argument("--iterations", type=int, nargs=2, default=(1_000_000, 3_000_000), metavar=("LOW", "HIGH"))
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=8)
    p.add_argument("--perf", default="perf")
    a = p.parse_args()
    low, high = a.iterations
    binaries = {name: Path(path).resolve(strict=True) for name, path in a.go}
    if not 0 < low < high or a.rounds < 1 or len(binaries) != len(a.go):
        p.error("use increasing positive iteration counts, distinct build names and a positive round count")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "cpu", "variant", "fixture", "low", "high", "low_instructions", "high_instructions",
                     "instructions_per_extra_op", "high_ns_per_op", "high_bytes_per_op", "high_allocs_per_op"])
    names = list(binaries)
    with tempfile.TemporaryDirectory() as directory:
        counts = Path(directory) / "perf.csv"
        for i in range(a.rounds):
            offset = i % len(names)
            order = names[offset:] + names[:offset]
            for fixture in a.fixture:
                for name in order:
                    results = {n: run(binaries[name], fixture, n, a.cpu, a.perf, counts)
                               for n in ((low, high) if i % 2 == 0 else (high, low))}
                    lo, hi = results[low][0], results[high][0]
                    writer.writerow([i+1, a.cpu, name, fixture, low, high, lo, hi,
                                     round((hi-lo)/(high-low), 3), *results[high][1:]])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
