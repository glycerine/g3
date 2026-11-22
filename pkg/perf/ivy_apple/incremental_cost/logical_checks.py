"""Measure each verification from the translated goal through the solver result."""
import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys

from mutation_costs import replace_once

HERE = Path(__file__).resolve().parent


def instrument(args, engine):
    directory = args.output / engine
    directory.mkdir()
    files = {}
    if engine == "g3" and args.go_overlay:
        files = json.loads(args.go_overlay.read_text())["Replace"]
    source = args.goivy / "z3bridge_solver_model.go"
    code = Path(files.get(str(source), source)).read_text()
    code = replace_once(code, "finalCheck := check", "finishInterval := startLogicalCost()\n finalCheck := check")
    code = replace_once(code, "if proveGoal(zCond) {",
                        'if proveGoal(zCond) {\n finishInterval("prepush", int(smt.Unsat))')
    check = "res, err := s.decideZ3With(z3solver, finalCheck)"
    code = replace_once(code, check, check + '\n finishInterval("scoped", int(res))')
    standalone = "result := s.checkZ3With(z3solver, check)"
    code = replace_once(code, standalone, """
    var finishInterval func(string, int)
    if !insideLogicalCost { finishInterval = startLogicalCost() }
    """ + standalone + '\n if finishInterval != nil { finishInterval("standalone", int(result)) }')
    target = directory / source.name
    target.write_text(code)
    files[str(source)] = str(target)

    helper = (HERE / "thread_counter.go.txt").read_text()
    helper = replace_once(helper, "func beginMeasuredCheck() func() {", """
var insideLogicalCost bool
func startLogicalCost() func(string, int) {
    if insideLogicalCost { panic("nested logical cost") }
    insideLogicalCost = true""")
    helper = replace_once(helper, "return func() {",
                          "return func(phase string, result int) {\n insideLogicalCost = false")
    helper = replace_once(helper, '"IVY-THREAD-CHECK", check', '"LOGICAL-COST", check, phase, result')
    target = directory / "logical_cost_measure.go"
    target.write_text('package goivy\nimport ("fmt";"os";"runtime";"strconv";"syscall";"time";"unsafe")\n' + helper)
    files[str(args.goivy / target.name)] = str(target)
    overlay = directory / "overlay.json"
    overlay.write_text(json.dumps({"Replace": files}))
    binary = directory / "checker"
    tags = "xtrace_off" if engine == "g3" else "xtrace_off,classic_z3"
    subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-tags=" + tags,
                    "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    return binary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("goivy", "spec", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--go-overlay", type=Path)
    parser.add_argument("--isolate")
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--pairs", type=int, default=3)
    args = parser.parse_args()
    if args.pairs < 1:
        parser.error("a positive pair count is required")
    args.goivy = args.goivy.resolve(strict=True)
    args.spec = args.spec.absolute()
    args.output = args.output.resolve()
    args.output.mkdir(parents=True)
    binaries = {name: instrument(args, name) for name in ("g3", "native")}
    flags = (["isolate=" + args.isolate] if args.isolate else []) + [args.spec]
    reference = None
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["pair", "engine", "check", "phase", "verdict", "instructions", "wall_ns"])
    for i in range(args.pairs):
        for engine in (("native", "g3") if i % 2 == 0 else ("g3", "native")):
            directory = args.output / f"pair-{i + 1}-{engine}"
            directory.mkdir()
            result = subprocess.run(["taskset", "-c", str(args.cpu), binaries[engine], *flags],
                                    env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1", IVY_THREAD_EVENT="1"),
                                    cwd=directory, capture_output=True)
            (directory / "stdout").write_bytes(result.stdout)
            (directory / "stderr").write_bytes(result.stderr)
            indices, answers = [], []
            for line in result.stderr.decode().splitlines():
                if line.startswith("LOGICAL-COST "):
                    _, index, phase, answer, count, empty, enabled, running, ns = line.split()
                    if int(enabled) <= 0 or enabled != running:
                        raise ValueError(f"counter did not run continuously in {directory}")
                    indices.append(int(index))
                    answers.append(answer)
                    writer.writerow([i + 1, engine, index, phase, answer, int(count) - int(empty), ns])
            if indices != list(range(1, len(indices) + 1)) or not indices:
                raise ValueError(f"incomplete verification intervals in {directory}")
            sample = (result.returncode, result.stdout, answers)
            if reference is None:
                reference = sample
            elif reference != sample:
                raise ValueError(f"solver output or status differs in {directory}")
            sys.stdout.flush()


if __name__ == "__main__":
    main()
