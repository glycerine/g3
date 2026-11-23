"""Count solver-thread instructions on the first physical push and its sibling checks."""
import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys

from mutation_costs import replace_once
from thread_checks import HERE, PACKAGES


def build(goivy, destination, native):
    source = goivy / "z3bridge_solver_model.go"
    before, marker, after = source.read_text().partition("func (s *Solver) GetSmallModelWithCond(")
    if not marker:
        raise ValueError("missing batched solver entry")
    after = replace_once(after, "if finalCond != nil {", """
    batchNumber++
    group := batchNumber
    ordinal, scope := 0, 0
    if finalCond != nil {""")
    after = replace_once(after, "finalCheck := check", """
        ordinal++
        logicalNumber++
        checkNumber := logicalNumber
        finalCheck := check""")
    after = replace_once(after, "if proveGoal(zCond) {", """
        stop := measureBatchOperation(checkNumber, group, ordinal, scope, "prepush")
        proved := proveGoal(zCond)
        stop()
        if proved {""")
    after = replace_once(after, "z3solver.Push()\n\t\t\t\t}\n\t\t\t\tz3solver.Assert(zCond)", """
        scope++
        stop := measureBatchOperation(checkNumber, group, ordinal, scope, "push")
        z3solver.Push()
        stop()
    }
    stop := measureBatchOperation(checkNumber, group, ordinal, scope, "assert")
    z3solver.Assert(zCond)
    stop()""")
    check = "res, err := s.decideZ3With(z3solver, finalCheck)"
    after = replace_once(after, check, """
        stop = measureBatchOperation(checkNumber, group, ordinal, scope, "check")
        insideBatchCheck = true
    """ + check + """
        insideBatchCheck = false
        stop()""")
    check = "result := s.checkZ3With(z3solver, check)"
    before = replace_once(before, check, """
        var stop func()
        if !insideBatchCheck {
            logicalNumber++
            stop = measureBatchOperation(logicalNumber, 0, 0, 0, "standalone")
        }
    """ + check + """
        if stop != nil { stop() }""")
    destination.mkdir()
    target = destination / source.name
    target.write_text(before + marker + after)
    helper = (HERE / "thread_counter.go.txt").read_text()
    helper = replace_once(helper, "func beginMeasuredCheck() func() {", """
var logicalNumber, batchNumber int
var insideBatchCheck bool
func measureBatchOperation(checkNumber, group, ordinal, scope int, operation string) func() {""")
    helper = replace_once(helper, '"IVY-THREAD-CHECK", check',
                          '"BATCH-OP", checkNumber, group, ordinal, scope, operation, check')
    counter = destination / "batch_counter.go"
    counter.write_text("package goivy\nimport (\n" + "\n".join(f'"{k}"' for k in PACKAGES) + "\n)\n" + helper)
    overlay = destination / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {str(source): str(target), str(goivy / counter.name): str(counter)}}))
    binary = destination / "checker"
    tags = "xtrace_off,classic_z3" if native else "xtrace_off"
    subprocess.run(["go", "build", "-buildvcs=false", "-pgo=off", "-mod=readonly", "-tags="+tags,
                    "-overlay="+str(overlay), "-o", binary, "./cmd/goivy_check"], cwd=goivy, check=True)
    return binary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", required=True, type=Path)
    parser.add_argument("--spec", required=True, type=Path)
    parser.add_argument("--isolate")
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--pairs", type=int, default=4)
    parser.add_argument("--expected-exit", type=int, default=0)
    args = parser.parse_args()
    if args.pairs < 1:
        parser.error("supply a positive pair count")
    directory = args.output.resolve()
    directory.mkdir(parents=True)
    binaries = {v: build(args.goivy.resolve(strict=True), directory / v, v == "native") for v in ("g3", "native")}
    flags = (["isolate=" + args.isolate] if args.isolate else []) + [args.spec.resolve(strict=True)]
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["pair", "engine", "check", "group", "ordinal", "scope", "operation", "sequence",
                     "instructions", "empty", "enabled", "running", "elapsed_ns"])
    reference = None
    for pair in range(1, args.pairs + 1):
        for engine in binaries if pair % 2 else reversed(binaries):
            run = directory / f"{pair}-{engine}"
            run.mkdir()
            result = subprocess.run(["taskset", "-c", str(args.cpu), binaries[engine], *flags], capture_output=True,
                                    cwd=run, env=dict(os.environ, GOMAXPROCS="1", GOGC="100", IVY_THREAD_EVENT="1"))
            (run / "out").write_bytes(result.stdout)
            (run / "err").write_bytes(result.stderr)
            if result.returncode != args.expected_exit or reference is not None and result.stdout != reference:
                raise ValueError(f"unexpected exit status or proof transcript in {run}")
            reference = result.stdout
            check, sequence = 0, 0
            for line in result.stderr.decode().splitlines():
                if line.startswith("BATCH-OP "):
                    _, index, group, ordinal, scope, operation, seq, count, empty, enabled, running, ns = line.split()
                    if (int(index) not in (check, check+1) or int(seq) != sequence+1 or
                            int(enabled) <= 0 or enabled != running or int(count) <= 0):
                        raise ValueError(f"incomplete counter sequence in {run}")
                    check, sequence = int(index), int(seq)
                    writer.writerow([pair, engine, index, group, ordinal, scope, operation, seq, count, empty,
                                     enabled, running, ns])
            if not sequence:
                raise ValueError(f"no measurement in {run}")
            sys.stdout.flush()


if __name__ == "__main__":
    main()
