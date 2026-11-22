"""Replay bounded unit sweeps on copies of each Boolean problem emitted by GoIvy."""
import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
from phase_counters import G3_METHOD, replace_once

OBSERVER = """
func (k *BooleanProblem) StudyUnitSchedules(check int) {
    limit, err := control.NewReslimit(nil)
    if err != nil { panic(err) }
    defer limit.Close()
    rows := make([]BooleanClause, len(k.Clauses))
    for _, schedule := range []string{"forward", "reverse", "alternate", "reverse-first"} {
        for _, rounds := range []int{1, 4, 8, 16, 32, 64} {
            copy(rows, k.Clauses)
            trial := BooleanProblem{Vars: k.Vars, Clauses: rows}
            start := limit.Count()
            conflict, complete := trial.studyReduce(rounds, limit, schedule)
            if !complete { panic("unbounded diagnostic reduction was interrupted") }
            fmt.Fprintln(os.Stderr, "IVY-UNIT", check, schedule, rounds, k.Vars,
                len(k.Clauses), trial.Vars, len(trial.Clauses), conflict, limit.Count()-start)
        }
    }
}
"""


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for key in ("goivy", "spec", "reference", "output"):
        parser.add_argument("--" + key, type=Path, required=True)
    parser.add_argument("--isolate", help="Apple isolate, or omit for Raft")
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--unrestricted", action="store_true", help="also try flat multi-quantifier goals")
    args = parser.parse_args()
    ivy, out = args.goivy.resolve(strict=True), args.output.resolve()
    out.mkdir(parents=True)
    module = Path(subprocess.check_output(
        ["go", "list", "-mod=readonly", "-m", "-f", "{{.Dir}}", "github.com/glycerine/g3/pkg"],
        cwd=ivy, text=True).strip())
    paths = {}

    def patch(original, code):
        target = out / original.name
        target.write_text(code)
        for path in {original, original.resolve()}:
            paths[str(path)] = str(target)

    traversal = module / "api/internal/order/boolean_units.go"
    code = replace_once(traversal.read_text(), 'import "github.com/glycerine/g3/pkg/util/control"',
                        'import ("fmt"; "os"; "github.com/glycerine/g3/pkg/util/control")')
    signature = "func (k *BooleanProblem) Reduce(rounds int, limit *control.Reslimit) (conflict, complete bool) {"
    code = replace_once(code, signature, signature + '\n return k.studyReduce(rounds, limit, "reverse-first")\n}\n'
                        + signature.replace("Reduce(", "studyReduce(").replace("*control.Reslimit)", "*control.Reslimit, schedule string)"))
    code = replace_once(code, "backward := rnd%2 == 0",
                        'backward := schedule=="reverse" || schedule=="reverse-first" && rnd%2==0 || schedule=="alternate" && rnd%2==1')
    patch(traversal, code + OBSERVER)
    api = module / "api/api_order_boolean.go"
    code = replace_once(api.read_text(), "if p.bits == nil {", "k.StudyUnitSchedules(StudyUnitCheck)\n if p.bits == nil {")
    patch(api, code + "\nvar StudyUnitCheck int\n")
    bridge = ivy / "smt/g3_solver.go"
    patch(bridge, replace_once(bridge.read_text(), G3_METHOD, G3_METHOD + "\n api.StudyUnitCheck++"))
    if args.unrestricted:
        goal = module / "api/internal/order/quantified_goal.go"
        patch(goal, replace_once(goal.read_text(), "projectGoalShape(m, assertions[len(assertions)-1])",
                                 "m.HasQuantifiers(assertions[len(assertions)-1])"))
    overlay = out / "overlay.json"
    overlay.write_text(json.dumps({"Replace": paths}))
    binary = out / "checker"
    subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-tags=xtrace_off",
                    "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"], cwd=ivy, check=True)
    command = ["taskset", "-c", str(args.cpu), binary]
    if args.isolate:
        command.append("isolate=" + args.isolate)
    # GoIvy echoes the input path in its transcript. Preserve symlink spelling
    # so a reference generated with the same command compares byte for byte.
    command.append(args.spec.absolute())
    with (out / "stdout").open("wb") as so, (out / "stderr").open("wb") as se:
        subprocess.run(command, cwd=out, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"),
                       stdout=so, stderr=se, check=True)
    if (out / "stdout").read_bytes() != args.reference.read_bytes():
        raise ValueError("verification transcript changed")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["case", "check", "schedule", "rounds", "variables", "clauses", "residual_variables",
                     "residual_clauses", "conflict", "resources"])
    with (out / "stderr").open() as source:
        for line in source:
            if line.startswith("IVY-UNIT "):
                writer.writerow([args.isolate or "raft", *line.split()[1:]])


if __name__ == "__main__":
    main()
