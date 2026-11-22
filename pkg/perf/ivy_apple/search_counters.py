#!/usr/bin/env python3
"""Compare search work during Check or Push for complete, incremental GoIvy Apple isolates."""

import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[3]
COUNTERS = ("decisions quant-instantiations conflicts rlimit-count added-eqs binary-propagations "
            "del-clause max-generation minimized-lits mk-bool-var mk-clause propagations num-checks "
            "datatype-accessor-ax datatype-constructor-ax datatype-splits restarts final-checks").split()
GO_METHOD = "func (s *Z3Solver) checkVerificationCondition(prove func() (bool, error)) Z3CheckResult {"
GO_OBSERVER = """
    ivyCounterSequence++
    n := ivyCounterSequence
    before := ivyCounterStats(s)
    defer func() {
        fmt.Fprintln(os.Stderr, "IVY-STATS", n, strconv.Quote(before), strconv.Quote(ivyCounterStats(s)))
    }()
"""
GO_HELPER = """
var ivyCounterSequence int
func ivyCounterStats(s *Z3Solver) string {
    stats, err := s.c.Statistics()
    fail(err)
    text, err := stats.String()
    fail(err)
    fail(stats.Close())
    return text
}
"""


def original_scopes(code):
    """Observe search under the native sequence of assertions, pushes and pops."""
    method = "func (s *Z3Solver) BeginVerificationGoalChecks()"
    if method not in code:
        return code
    start = code.index("{", code.index(method)) + 1
    end = code.index("\n}\n", start)
    return code[:start] + "\n return nil, nil, nil" + code[end:]


def build(ivy, tmp, name, native, operation="check", go_overlay=None):
    common = ["go", "build", "-mod=readonly", "-buildvcs=false", "-pgo=off"]
    if native:
        obj = tmp / "observer.o"
        switch = ["-DIVY_OBSERVE_PUSH"] if operation == "push" else []
        subprocess.run(["c++", "-O2", "-fPIC", "-c", *switch, "-I", ROOT / "src/api",
                        ROOT / "src/test/perf/ivy_solver_statistics.cpp", "-o", obj], check=True)
        lib = ivy / "z3vendor/native_lib/libz3.a"
        flags = ["-tags=xtrace_off,classic_z3",
                 "-ldflags", f"-linkmode=external '-extldflags=-Wl,--wrap=Z3_solver_{operation} {obj} {lib} -lstdc++ -lm -lgomp'"]
    else:
        src = ivy / "smt/g3_solver.go"
        files = json.loads(go_overlay.read_text())["Replace"] if go_overlay else {}
        code = original_scopes(Path(files.get(str(src), src)).read_text())
        method = GO_METHOD if operation == "check" else "func (s *Z3Solver) Push() {"
        if code.count(method) != 1 or code.count('import (') != 1:
            raise ValueError(f"unrecognized GoIvy solver adapter {src}")
        extras = [s for s in ("fmt", "os") if f'"{s}"' not in code]
        code = code.replace('import (', 'import (\n' + '\n'.join(f'"{s}"' for s in extras), 1)
        patched = tmp / "g3_solver.go"
        patched.write_text(code.replace(method, method + GO_OBSERVER) + GO_HELPER)
        overlay = tmp / "overlay.json"
        files[str(src)] = str(patched)
        overlay.write_text(json.dumps({"Replace": files}))
        flags = ["-tags=xtrace_off", f"-overlay={overlay}"]
    binary = tmp / name
    subprocess.run([*common, *flags, "-o", binary, "./cmd/goivy_check"], cwd=ivy, check=True)
    return binary


def measurements(lines):
    before = {}
    after = {}
    decoder = json.JSONDecoder()
    for line in lines:
        if line.startswith("NATIVE-STATS "):
            header, *fields = line.split("|")
            _, num, side = header.split()
            values = {k.replace(" ", "-"): int(v) for k, v in (f.rsplit("=", 1) for f in fields)}
            (before if side == "before" else after)[int(num)] = values
        elif line.startswith("IVY-STATS "):
            _, num, pair = line.split(" ", 2)
            first, end = decoder.raw_decode(pair)
            last = decoder.decode(pair[end:].strip())
            for output, stats in [(before, first), (after, last)]:
                output[int(num)] = {k: int(v) for k, v in re.findall(r":([\w-]+)\s+(\d+)(?=[)\s])", stats)}
    if before.keys() != after.keys() or set(after) != set(range(1, len(after) + 1)):
        raise ValueError("incomplete sequence of solver statistics")
    return [[after[i].get(k, 0) - before[i].get(k, 0) for k in COUNTERS]
            for i in range(1, len(after) + 1)]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", type=Path, required=True, help="GoIvy directory with its local g3 replacement")
    p.add_argument("--spec", type=Path, required=True)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--operation", choices=("check", "push"), default="check")
    p.add_argument("--go-overlay", type=Path, help="optional absolute-path source overlay for the g3 build")
    p.add_argument("isolates", nargs="+")
    args = p.parse_args()
    spec = args.spec.resolve(strict=True)
    ivy = args.goivy.resolve(strict=True)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "engine", args.operation, *COUNTERS])
    with tempfile.TemporaryDirectory() as scratch:
        tmp = Path(scratch)
        engines = {name: build(ivy, tmp, name, name == "native", args.operation, args.go_overlay)
                   for name in ["g3", "native"]}
        for isolate in args.isolates:
            reference = None
            expected_rows = None
            for engine, checker in engines.items():
                got = subprocess.run(["taskset", "-c", str(args.cpu), checker, f"isolate={isolate}", spec],
                                     cwd=tmp, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"),
                                     capture_output=True, check=True)
                lines = got.stdout.splitlines()
                rows = measurements(got.stderr.decode().splitlines())
                if not rows or lines[-1:] != [b"OK"]:
                    raise ValueError(f"incomplete run of {isolate} on {engine}")
                if args.operation == "check" and len(rows) != lines.count(b"PASS"):
                    raise ValueError(f"missing checks for {isolate} on {engine}")
                if expected_rows is not None and len(rows) != expected_rows:
                    raise ValueError(f"{isolate} {args.operation} counts differ")
                expected_rows = len(rows)
                if reference is not None and got.stdout != reference:
                    raise ValueError(f"{isolate} checker transcripts differ")
                reference = got.stdout
                for i, counters in enumerate(rows, 1):
                    writer.writerow([isolate, engine, i, *counters])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
