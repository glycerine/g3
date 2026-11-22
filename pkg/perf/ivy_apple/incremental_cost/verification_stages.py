"""Measure disjoint solver-thread intervals per GoIvy solver on one Apple isolate."""
import argparse
import csv
import json
from pathlib import Path
import subprocess
import sys

from mutation_costs import build, measure, replace_once


def instrument(args):
    build(args, "g3")
    directory = args.output / "g3"
    overlay = directory / "overlay.json"
    mapping = json.loads(overlay.read_text())["Replace"]
    source = args.goivy / "smt/g3_solver.go"
    target = Path(mapping[str(source)])
    code = target.read_text()
    code = replace_once(code, "import (", 'import (\n"fmt"')
    code = replace_once(code, "type Z3Solver struct {", "type Z3Solver struct {\n measuredOwner int")
    code = code.replace('defer ivyMeasureMutation("', 'defer s.measureStage("')
    code = replace_once(code, "defer ivyInsideCheck()()", 'defer s.measureStage("Check")()\n defer ivyInsideCheck()()')
    direct = "func (s *Z3Solver) Check() Z3CheckResult {"
    code = replace_once(code, direct, direct + ' defer s.measureStage("DirectCheck")();')
    if "func (s *Z3Solver) BeginVerificationGoalChecks()" in code:
        code = replace_once(code, "func(goal Z3Expr) bool {",
                            'func(goal Z3Expr) bool {\n defer s.measureStage("Preflight")()')
    code += """
var nextMeasuredOwner int
func (s *Z3Solver) measureStage(stage string) func() {
    if s.measuredOwner == 0 {
        nextMeasuredOwner++
        s.measuredOwner = nextMeasuredOwner
    }
    return ivyMeasureMutation(fmt.Sprintf("%d-%s", s.measuredOwner, stage))
}
"""
    target.write_text(code)
    binary = directory / "checker"
    flags = [f"-modfile={args.modfile.resolve(strict=True)}"] if args.modfile else []
    subprocess.run(["go", "build", "-mod=readonly", "-buildvcs=false", "-pgo=off", "-tags=xtrace_off",
                    *flags, "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"],
                   cwd=args.goivy, check=True)
    return binary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ["goivy", "spec", "reference", "output"]:
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--modfile", type=Path)
    parser.add_argument("--go-overlay", type=Path)
    parser.add_argument("--isolate", default="rfn2")
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--rounds", type=int, default=5)
    args = parser.parse_args()
    args.goivy = args.goivy.resolve(strict=True)
    args.spec = args.spec.absolute()
    args.output = args.output.resolve()
    reference = args.reference.read_bytes()
    if not args.spec.is_file() or args.rounds < 1 or not reference.rstrip().endswith(b"\nOK"):
        parser.error("supply a successful complete reference and positive round count")
    args.output.mkdir(parents=True)
    binary = instrument(args)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "round", "sequence", "owner", "stage", "instructions", "elapsed_ns"])
    for pair in range(1, args.rounds + 1):
        for seq, operation, count, empty, _, _, ns in measure(args, "g3", binary, args.isolate, reference, pair):
            owner, stage = operation.split("-")
            writer.writerow([args.isolate, pair, seq, owner, stage, count - empty, ns])
        sys.stdout.flush()


if __name__ == "__main__":
    main()
