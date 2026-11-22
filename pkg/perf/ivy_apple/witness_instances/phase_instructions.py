"""Count Linux/amd64 solver-thread instructions in each bounded ground proof phase."""

import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
from phase_counters import build, replace_once

CHECK_ORDINAL = """
var witnessCheckOrdinal int
func beginMeasuredCheck() func() {
    witnessCheckOrdinal++
    fmt.Fprintln(os.Stderr, "WITNESS-CHECK", witnessCheckOrdinal)
    return func() {}
}
"""


def instrument(pkg, module, destination):
    paths = {}
    def install(name, source):
        target = destination / name
        target.write_text(source)
        for base in (pkg, module):
            paths[str(base / "api" / name)] = str(target)

    counter = (HERE.parent / "incremental_cost/thread_counter.go.txt").read_text()
    counter = replace_once(counter, "func beginMeasuredCheck() func()", "func beginGroundPhase(label string) func()")
    counter = replace_once(counter, "ivyThreadCheck++\n\tcheck := ivyThreadCheck", "")
    counter = replace_once(counter, '"IVY-THREAD-CHECK", check,', '"WITNESS-PHASE", label,')
    install("study_ground_phase_clock.go",
            'package api\nimport("fmt";"os";"runtime";"strconv";"syscall";"time";"unsafe")\n' + counter)

    name = "api_witness_check.go"
    source = (pkg / "api" / name).read_text()
    start = "tr, err := ast.NewASTTranslation(parent, m, true)"
    source = replace_once(source, start, """
    phase := beginGroundPhase("translation_setup")
    next := func(label string) { phase(); phase = beginGroundPhase(label) }
    defer func() { phase() }()
    """ + start)
    for label, line in (("input", "if ok, err := s.acceptsInput"), ("translate", "for _, sort := range p.sorts"),
                        ("lower", "base := s.lowerAssertions(input)"), ("forced", "for _, root := range base"),
                        ("instantiate", "s.generate(p.width, p.rounds)"), ("params", "params, err :="),
                        ("assert", "for _, root := range append"), ("release_input", "return true, nil")):
        source = replace_once(source, line, f'next("{label}")\n\t{line}')
    install(name, source)

    name = "api_goal_instances_check.go"
    source = (pkg / "api" / name).read_text()
    source = replace_once(source, "import (", 'import (\n"fmt"\n"os"')
    start = "complete, err := plan.assertIn(parent, m, aux.child)"
    source = replace_once(source, start, """
    if witness, ok := plan.(*witnessInstancePlan); ok {
        fmt.Fprintln(os.Stderr, "WITNESS-PLAN", len(witness.assertions), witness.width, witness.rounds)
    } else { fmt.Fprintln(os.Stderr, "WITNESS-PLAN", 0, 0, 0) }
    """ + start)
    start = "answer, err := aux.child.Check()"
    source = replace_once(source, start, 'finish := beginGroundPhase("search")\n\t' + start +
                          '\n\tfinish()\n\tfmt.Fprintln(os.Stderr, "WITNESS-RESULT", answer)')
    install(name, source)
    overlay = destination / "phases.json"
    overlay.write_text(json.dumps({"Replace": paths}))
    return overlay


def records(log, expected):
    ordinal = attempt = 0
    plan = []
    for line in log.splitlines():
        fields = line.split()
        if not fields:
            continue
        if fields[0] == "WITNESS-CHECK":
            if int(fields[1]) != ordinal + 1:
                raise ValueError("unexpected verification sequence")
            ordinal, attempt = int(fields[1]), 0
        elif fields[0] == "WITNESS-PLAN":
            plan = list(map(int, fields[1:]))
            attempt += 1
        elif fields[0] == "WITNESS-PHASE":
            phase = fields[1]
            count, empty, enabled, running, wall = map(int, fields[2:])
            if attempt < 1 or running <= 0 or running != enabled:
                raise ValueError(f"incomplete counter in check {ordinal} phase {phase}")
            yield [ordinal, attempt, *plan, phase, count, empty, enabled, running, wall]
    if ordinal != expected:
        raise ValueError(f"observed {ordinal} of {expected} checks")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", type=Path, required=True)
    parser.add_argument("--modfile", type=Path)
    parser.add_argument("--spec", type=Path, required=True)
    parser.add_argument("--isolate", required=True)
    parser.add_argument("--reference", type=Path, required=True, help="linked Z3 stdout for the same spec path")
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--output", type=Path, required=True, help="new directory")
    args = parser.parse_args()
    args.goivy = args.goivy.resolve(strict=True)
    spec = args.spec.absolute()
    reference = args.reference.read_bytes()
    expected = reference.splitlines().count(b"PASS")
    if not expected or not reference.rstrip().endswith(b"\nOK") or args.rounds < 1:
        parser.error("supply a complete reference and a positive number of rounds")
    directory = args.output.resolve()
    directory.mkdir(parents=True)
    # The replacement can use a symlink spelling that differs from __file__.
    cmd = ["go", "list", "-mod=readonly"]
    if args.modfile:
        cmd.append("-modfile=" + str(args.modfile.resolve(strict=True)))
    module = Path(subprocess.check_output([*cmd, "-m", "-f", "{{.Dir}}", "github.com/glycerine/g3/pkg"],
                                          cwd=args.goivy, text=True).strip())
    pkg = HERE.parents[2]
    if not module.samefile(pkg):
        parser.error("the GoIvy module must resolve g3 to the checkout under study")
    overlay = instrument(pkg, module, directory)
    checker = build("g3", args, directory, CHECK_ORDINAL, ("fmt", "os"), go_overlay=overlay)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "round", "cpu", "check", "attempt", "assertions", "width", "rounds", "phase",
                     "instructions", "empty", "enabled_ns", "running_ns", "wall_ns"])
    for i in range(1, args.rounds + 1):
        loc = directory / str(i)
        loc.mkdir()
        with (loc / "out").open("wb") as out, (loc / "err").open("wb") as err:
            subprocess.run(["taskset", "-c", str(args.cpu), checker, "isolate=" + args.isolate, spec],
                           cwd=loc, env=dict(os.environ, GOMAXPROCS="1", IVY_THREAD_EVENT="1", XTRACE_OFF="1"),
                           stdout=out, stderr=err, check=True)
        if (loc / "out").read_bytes() != reference:
            raise ValueError(f"verification differs from the linked solver in round {i}")
        writer.writerows([args.isolate, i, args.cpu, *row] for row in records((loc / "err").read_text(), expected))


if __name__ == "__main__":
    main()
