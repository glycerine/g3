"""Record instruction stacks only while the Ivy adapter pushes a solver scope, outside checks."""
import argparse
from collections import Counter
import csv
import json
from pathlib import Path
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from phase_counters import G3_METHOD, NATIVE_METHOD, OBSERVER, replace_once
from phase_stacks import record
from stack_costs import samples


def build(a, directory):
    smt = a.goivy.resolve() / "smt"
    helper = OBSERVER[:OBSERVER.index("func beginMeasuredCheck()")] + """
var pushProfileDepth int
func noteCheck() func() {
    phaseSequence++
    n := phaseSequence
    pushProfileDepth++
    return func() {
        pushProfileDepth--
        fmt.Fprintln(os.Stderr, "IVY-PERF-CHECK", n)
    }
}
func samplePush() func() {
    if pushProfileDepth != 0 { return func() {} }
    phaseSignal("enable")
    return func() {
        phaseSignal("disable")
        fmt.Fprintln(os.Stderr, "IVY-PUSH-PROFILE")
    }
}
"""
    push = "func (s *Z3Solver) Push() {"
    mapping = {}
    files = ["g3_solver.go"] if a.engine == "g3" else ["native_verification_condition.go", "quantifier.go"]
    for filename in files:
        source = smt / filename
        code = source.read_text()
        if filename in ("g3_solver.go", "quantifier.go"):
            code = replace_once(code, push, push + "\n defer samplePush()();")
        if filename != "quantifier.go":
            if a.engine == "g3":
                code = replace_once(code, G3_METHOD, G3_METHOD + "\n defer noteCheck()()")
            else:
                code = replace_once(code, NATIVE_METHOD, NATIVE_METHOD.replace("return s.Check()",
                                    "defer noteCheck()(); return s.Check()"))
                code = replace_once(code, "return s.Check, func() {}", "return s.CheckVerificationCondition, func() {}")
            imports = [f'"{m}"' for m in ("io", "os", "fmt") if f'"{m}"' not in code]
            code = code.replace("package smt", "package smt\nimport (\n" + "\n".join(imports) + "\n)", 1)
            code += helper
        patched = directory / filename
        patched.write_text(code)
        mapping[str(source)] = str(patched)
    overlay = directory / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    binary = directory / a.engine
    tags = "xtrace_off" if a.engine == "g3" else "xtrace_off,classic_z3"
    cmd = ["go", "build", "-mod=readonly", "-buildvcs=false", "-pgo=off", "-tags=" + tags, "-overlay=" + str(overlay)]
    if a.modfile:
        cmd.append("-modfile=" + str(a.modfile.resolve(strict=True)))
    subprocess.run([*cmd, "-o", binary, "./cmd/goivy_check"], cwd=a.goivy, check=True)
    return binary


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for arg in ("spec", "reference", "goivy", "output"):
        p.add_argument("--" + arg, type=Path, required=True)
    p.add_argument("--modfile", type=Path)
    p.add_argument("--engine", choices=("g3", "native"), required=True)
    p.add_argument("--isolate", required=True)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--pairs", type=int, default=2)
    p.add_argument("--period", type=int, default=10000019)
    p.add_argument("--perf", default="perf")
    p.add_argument("--call-graph", choices=("fp", "dwarf,8192"), default="dwarf,8192")
    a = p.parse_args()
    if a.period < 1 or a.pairs < 1:
        p.error("supply positive rounds and sampling period")
    a.output = a.output.resolve()
    a.output.mkdir(parents=True)
    a.spec = a.spec.absolute()
    a.by_check = a.inline = a.keep_raw = False
    reference = a.reference.read_bytes()
    if not reference.endswith(b"OK\n"):
        raise ValueError("incomplete reference transcript")
    binary = build(a, a.output)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "round", "engine", "cpu", "period", "pushes", "all_samples", "attribution", "symbol", "samples"])
    for i in range(a.pairs):
        run = a.output / str(i + 1)
        record(a, binary, "inside", run, reference)
        pushes = (run / "stderr").read_bytes().splitlines().count(b"IVY-PUSH-PROFILE")
        flat, inclusive = Counter(), Counter()
        for stack in samples(run / "stacks.gz"):
            flat[stack[0] if stack else "(unknown)"] += 1
            inclusive.update(set(stack))
        if not pushes or not flat:
            raise ValueError("no pushes or sampled instructions")
        for label, counts in [("leaf", flat), ("inclusive", inclusive)]:
            for symbol, n in counts.most_common():
                writer.writerow([a.isolate, i+1, a.engine, a.cpu, a.period, pushes, flat.total(), label, symbol, n])
        sys.stdout.flush()


if __name__ == "__main__":
    main()
