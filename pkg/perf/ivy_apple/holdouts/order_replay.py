"""Replay captured incremental Ivy order obligations in the g3 and Z3 shells."""
import argparse
import csv
import io
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import tempfile
import time


VECTORS = Path(__file__).with_name("nov23-order-inputs.tar.gz")
COUNTERS = ("conflicts", "decisions", "propagations", "quant-instantiations")


def run(binary, vector, checks, args, work):
    command = [str(binary), "-st", "-smt2", str(vector)]
    counters = work / "perf.csv"
    if args.perf:
        command = [args.perf, "stat", "-x,", "-e", "instructions:u,cycles:u",
                   "-o", str(counters), "--", *command]
    if args.cpu is not None:
        command = ["taskset", "-c", str(args.cpu), *command]
    start = time.perf_counter()
    result = subprocess.run(command, capture_output=True, text=True, check=True,
                            env=dict(os.environ, LC_ALL="C", GOMAXPROCS="1", GOGC="100"))
    elapsed = time.perf_counter() - start
    answers = re.findall(r"^(unsat|unknown|sat)$", result.stdout, re.MULTILINE)
    if answers != ["unsat"] * checks or result.stderr or "(error " in result.stdout:
        raise ValueError(f"{binary} produced unexpected results for {vector.name}: "
                         f"{answers!r} {result.stderr[:200]}")
    stats = dict(re.findall(r":([\w-]+)\s+(\d+)\b", result.stdout))
    hardware = {}
    if args.perf:
        for row in csv.reader(counters.read_text().splitlines()):
            if len(row) >= 3 and row[2] in ("instructions:u", "cycles:u"):
                hardware[row[2]] = int(row[0])
        if len(hardware) != 2:
            raise ValueError(f"incomplete hardware counters in {counters.read_text()}")
    return [f"{elapsed:.6f}", hardware.get("instructions:u", ""), hardware.get("cycles:u", ""),
            *(int(stats.get(name, 0)) for name in COUNTERS)]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--list", action="store_true", help="list the saved vectors")
    parser.add_argument("--case", default="13-38", help="vector name from --list")
    parser.add_argument("--g3", type=Path)
    parser.add_argument("--z3", type=Path)
    parser.add_argument("--cpu", type=int, help="pin both engines to this Linux CPU")
    parser.add_argument("--perf", help="optional path to Linux perf for hardware counts")
    parser.add_argument("--rounds", type=int, default=3)
    args = parser.parse_args()
    with tarfile.open(VECTORS, "r:gz") as archive:
        manifest = archive.extractfile("manifest.csv").read().decode()
        if args.list:
            print(manifest.rstrip())
            return
        if not args.g3 or not args.z3 or args.rounds < 1:
            parser.error("supply --g3 and --z3 and a positive --rounds")
        cases = {row["vector"]: int(row["checks"]) for row in csv.DictReader(io.StringIO(manifest))}
        if args.case not in cases:
            parser.error("unknown vector, see --list")
        data = archive.extractfile(args.case + ".smt2").read()
    binaries = [("g3", args.g3.resolve(strict=True)), ("z3", args.z3.resolve(strict=True))]
    writer = csv.writer(sys.stdout)
    writer.writerow(["vector", "pair", "engine", "checks", "wall_s", "instructions", "cycles", *COUNTERS])
    with tempfile.TemporaryDirectory() as scratch:
        work = Path(scratch)
        vector = work / (args.case + ".smt2")
        vector.write_bytes(data)
        for pair in range(1, args.rounds + 1):
            for name, binary in binaries if pair % 2 else reversed(binaries):
                metrics = run(binary, vector, cases[args.case], args, work)
                writer.writerow([args.case, pair, name, cases[args.case], *metrics])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
