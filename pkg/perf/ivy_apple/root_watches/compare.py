"""Time the same root-disjunction assignment callback in this fork and the Go port."""

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

GO_CASE = re.compile(r"BenchmarkSMTAssertedOrWitness/Width(\d+)/Child(\d+)(?:-\d+)?$")


def run(binary, native, cpu):
    command = ["taskset", "-c", str(cpu), binary]
    if not native:
        command.extend(["-test.run", "^$", "-test.bench", "^BenchmarkSMTAssertedOrWitness$",
                        "-test.benchtime=500000x"])
    output = subprocess.check_output(command, env=dict(os.environ, GOMAXPROCS="1"), text=True)
    rows = []
    if native:
        for r in csv.DictReader(io.StringIO(output)):
            rows.append((int(r["width"]), int(r["child"]), int(r["iterations"]), float(r["ns_per_op"])))
    else:
        for line in output.splitlines():
            fields = line.split()
            if fields and (name := GO_CASE.fullmatch(fields[0])):
                rows.append((int(name[1]), int(name[2]), int(fields[1]), float(fields[2])))
    if [(w, i, n) for w, i, n, _ in rows] != [(w, i, 500000) for w in [4, 16, 64] for i in [0, w-1]]:
        raise ValueError(f"{binary} returned incomplete benchmark results")
    return rows


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--build", type=Path, default=ROOT / "build", help="configured native Makefile build")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=12)
    p.add_argument("--go-binary", action="append", metavar="LABEL=PATH",
                   help="prebuilt smtcore test binary, repeatable. By default build the current checkout.")
    args = p.parse_args()
    if args.rounds < 1:
        p.error("round count must be positive")
    with tempfile.TemporaryDirectory() as scratch:
        native = Path(scratch) / "native"
        build_native(args.build.resolve(strict=True), native, ROOT / "src/test/perf/asserted_or_witness.cpp")
        binaries = {"native": native}
        for spec in args.go_binary or []:
            name, sep, path = spec.partition("=")
            if not sep or not name or name in binaries:
                p.error("provide distinct Go labels as LABEL=PATH")
            binaries[name] = Path(path).resolve(strict=True)
        if len(binaries) == 1:
            binary = Path(scratch) / "g3"
            subprocess.run(["go", "test", "-c", "-o", binary, "./smtcore"], cwd=ROOT / "pkg", check=True)
            binaries["g3"] = binary
        writer = csv.writer(sys.stdout, lineterminator="\n")
        writer.writerow(["round", "cpu", "engine", "width", "child", "iterations", "ns_per_op"])
        names = list(binaries)
        for i in range(args.rounds):
            start = i % len(names)
            order = names[start:] + names[:start]
            if i // len(names) % 2:
                order.reverse()
            for name in order:
                for row in run(binaries[name], name == "native", args.cpu):
                    writer.writerow([i+1, args.cpu, name, *row])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
