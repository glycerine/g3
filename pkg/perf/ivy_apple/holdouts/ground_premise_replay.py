"""Reassert the original typed obligations independently in the linked Z3 backend."""
import argparse
import csv
import gzip
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from paired_verification import CASES

DEFAULT_CASES = ["tlb", "hybrid_reliable_broadcast_cisa", "cf_live", "dramc_nb2"]
HERE = Path(__file__).resolve().parent


def replace(source, old, new):
    if source.count(old) != 1:
        raise ValueError(f"expected one occurrence of {old!r}")
    return source.replace(old, new)


def build(goivy, dest, final_conditions=False):
    mapping = {}

    def overlay(name, source):
        path = dest / Path(name).name
        path.write_text(source)
        mapping[str(goivy / name)] = str(path)

    name = "smt/quantifier.go"
    code = (goivy / name).read_text()
    code = replace(code, "type Z3Solver struct {", """type Z3Solver struct {
        replayAssertions []Z3Expr
        replayScopes []int
        replaySolver *Z3Solver
        replayDetached bool""")
    operations = {
        "Assert(e Z3Expr)": "s.replayAssertions = append(s.replayAssertions, e)",
        "Push()": "s.replayScopes = append(s.replayScopes, len(s.replayAssertions))",
        "Pop()": """last := len(s.replayScopes)-1
            s.replayAssertions = s.replayAssertions[:s.replayScopes[last]]
            s.replayScopes = s.replayScopes[:last]""",
        "Reset()": "s.replayAssertions, s.replayScopes = nil, nil",
    }
    for method, record in operations.items():
        header = "func (s *Z3Solver) " + method + " {"
        code = replace(code, header, header + "\nif !s.replayDetached {\n" + record + "\n}")
    overlay(name, code)

    name = "smt/native_verification_condition.go"
    code = (goivy / name).read_text()
    code = replace(code, "package smt", 'package smt\nimport ("fmt"; "os"; "strconv"; "strings"; "time")')
    code = replace(code, "return s.Check()", "return s.replayGroundCheck()")
    code = replace(code, "return s.Check, func() {}", "return s.replayGroundCheck, func() {}")
    code += r'''
var replayOrdinal int
var replaySelected = func() map[int]bool {
    selected := make(map[int]bool)
    for _, field := range strings.Split(os.Getenv("IVY_REPLAY_CHECKS"), ",") {
        i, err := strconv.Atoi(field)
        if err != nil { panic(err) }
        selected[i] = true
    }
    return selected
}()
func (s *Z3Solver) replayGroundCheck() Z3CheckResult {
    replayOrdinal++
    if !replaySelected[replayOrdinal] { return s.Check() }
    start := time.Now()
    if s.replaySolver == nil {
        s.replaySolver = s.ctx.NewZ3Solver()
        s.replaySolver.replayDetached = true
    }
    fresh := s.replaySolver
    fresh.Reset()
    fresh.SetParam("macro_finder","true")
    fresh.SetParam("timeout","20000")
    fresh.Push()
    for _, term := range s.replayAssertions { fresh.Assert(term) }
    answer := fresh.Check()
    elapsed := time.Since(start).Microseconds()
    fresh.Pop()
    original := s.Check()
    fmt.Fprintf(os.Stderr,"GROUND-REPLAY %d %d %d %d %d\n",
        replayOrdinal,len(s.replayAssertions),original,answer,elapsed)
    return original
}
'''
    if final_conditions:
        # Number the frontend's final conditions regardless of the backend's
        # proof route. Other native check calls do not advance this counter.
        code = replace(code, "var replayOrdinal int", """var replayOrdinal int
var replayActive bool
func MarkReplayGoal() { replayOrdinal++; replayActive = true }""")
        code = replace(code, "    replayOrdinal++", """    if !replayActive { return s.Check() }
    replayActive = false""")
        bridge = "z3bridge_solver_model.go"
        text = (goivy / bridge).read_text()
        overlay(bridge, replace(text, "finalCheck := check", "smt.MarkReplayGoal()\n                finalCheck := check"))
    overlay(name, code)
    path = dest / "overlay.json"
    path.write_text(json.dumps({"Replace": mapping}))
    checker = dest / "checker"
    subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false",
                    "-tags=xtrace_off,classic_z3", "-overlay=" + str(path),
                    "-o", checker, "./cmd/goivy_check"], cwd=goivy, check=True)
    return checker


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", required=True, type=Path)
    parser.add_argument("--examples", required=True, type=Path)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--cases", nargs="+", choices=CASES, default=DEFAULT_CASES)
    parser.add_argument("--final-conditions", action="store_true", help="count frontend final conditions for the saved ordinals")
    parser.add_argument("--abc", type=Path)
    parser.add_argument("--observations", type=Path, default=HERE / "nov22b-ground-premises-ast-oracle.csv.gz")
    args = parser.parse_args()
    with gzip.open(args.observations, "rt") as file:
        observations = list(csv.DictReader(file))
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["case", "check", "asserted", "original_native", "fresh_native", "time_us"])
    with tempfile.TemporaryDirectory() as tmp:
        dest = Path(tmp)
        checker = build(args.goivy.resolve(strict=True), dest, args.final_conditions)
        if args.abc:
            (dest / "abc").symlink_to(args.abc.resolve(strict=True))
        for case in args.cases:
            rows = [r for r in observations if r["case"] == case]
            if not rows:
                raise ValueError(f"no saved ordinals for {case}")
            expected = [(r["check"], r["asserted"]) for r in rows]
            relative, isolate, _, failures, unknown = CASES[case]
            flags = [*(["isolate=" + isolate] if isolate else []), (args.examples / relative).resolve(strict=True)]
            proc = subprocess.run(["taskset", "-c", str(args.cpu), checker, *flags], cwd=dest,
                                  env=dict(os.environ, GOMAXPROCS="1", GOGC="100", XTRACE_OFF="1",
                                           IVY_REPLAY_CHECKS=",".join(k for k, _ in expected)),
                                  capture_output=True, text=True)
            if proc.returncode != int(bool(failures or unknown)):
                raise ValueError(f"{case} exited with {proc.returncode}: {proc.stderr[-1000:]}")
            seen = []
            for line in proc.stderr.splitlines():
                if line.startswith("GROUND-REPLAY "):
                    _, k, n, original, fresh, elapsed = line.split()
                    if fresh != "-1" or original != fresh:
                        raise ValueError(f"{case} check {k}: original {original}, fresh {fresh}")
                    seen.append((k, n))
                    writer.writerow([case, k, n, original, fresh, elapsed])
            if seen != expected:
                raise ValueError(f"{case} produced a different sequence of assertions")
            sys.stdout.flush()


if __name__ == "__main__":
    main()
