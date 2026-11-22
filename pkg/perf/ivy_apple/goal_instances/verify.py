"""Verify every private refutation with the original incremental solver and linked Z3."""
import argparse
import csv
import json
import os
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import search_counters as counters
from phase_counters import replace_once


def compile_go(args):
    tmp = args.output / "g3"
    tmp.mkdir()
    mapping = {}

    def patch(original, code):
        path = tmp / original.name
        path.write_text(code)
        mapping[str(original)] = mapping[str(original.resolve())] = str(path)

    original = args.goivy / "smt/g3_solver.go"
    code = counters.original_scopes(original.read_text())
    extras = [p for p in ("fmt", "os") if f'"{p}"' not in code]
    code = replace_once(code, "import (", "import (\n" + "\n".join(f'"{p}"' for p in extras))
    code = replace_once(code, counters.GO_METHOD, counters.GO_METHOD + counters.GO_OBSERVER +
                        '\n fmt.Fprintln(os.Stderr,"GROUND-BEGIN",n)')
    short_circuit = """if proved, err := prove(); err == nil && proved {
			result = Unsat
			return
		}"""
    code = replace_once(code, short_circuit, """proved, err := prove()
        if err != nil { panic(err) }
        if proved { fmt.Fprintln(os.Stderr, "GROUND-PROVED") }""")
    anchor = "r, err := s.c.Check()\n\t\tfail(err)"
    code = replace_once(code, anchor, anchor + '\n fmt.Fprintln(os.Stderr,"GROUND-RESULT",n,int(r))')
    patch(original, code + counters.GO_HELPER)
    overlay = tmp / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    binary = tmp / "checker"
    subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-tags=xtrace_off",
                    "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    return binary


def optimized_checker(args):
    """Observe actual refutations while the live verifier can still avoid first Push."""
    model = args.goivy / "z3bridge_solver_model.go"
    code = model.read_text()
    if "proveGoal(zCond)" not in code:
        return None
    directory = args.output / "optimized"
    directory.mkdir()
    source = args.goivy / "smt/g3_solver.go"
    smt = source.read_text()
    imports = [p for p in ("fmt", "os") if f'"{p}"' not in smt]
    smt = replace_once(smt, "import (", "import (\n" + "\n".join(f'"{p}"' for p in imports))
    smt += """
var ivyGoalSequence int
func ObserveBeginGoal() { ivyGoalSequence++ }
func ObserveGoal(phase string, result Z3CheckResult) {
    fmt.Fprintln(os.Stderr, "IVY-GOAL", ivyGoalSequence, phase, int(result))
}
"""
    success = "if proved, err := prove(); err == nil && proved {"
    smt = replace_once(smt, success, success + '\n ObserveGoal("scoped-proof", Unsat)')
    setup = "finalCheck := check"
    code = replace_once(code, setup, "smt.ObserveBeginGoal()\n" + setup)
    immediate = "if proveGoal(zCond) {"
    code = replace_once(code, immediate, immediate + '\n smt.ObserveGoal("prepush", smt.Unsat)')
    finish = "res, err := s.decideZ3With(z3solver, finalCheck)"
    code = replace_once(code, finish, finish + '\n smt.ObserveGoal("scoped", res)')
    mapping = {}
    for src, content in ((source, smt), (model, code)):
        dest = directory / src.name
        dest.write_text(content)
        mapping[str(src)] = str(dest)
    overlay = directory / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    binary = directory / "checker"
    subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-tags=xtrace_off",
                    "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    return binary


def optimized_results(args, binary, iso, cpu, reference, concrete):
    directory = args.output / (iso + "-optimized")
    directory.mkdir()
    with (directory / "stderr").open("wb") as err:
        run = subprocess.run(["taskset", "-c", cpu, binary, "isolate=" + iso, args.spec], cwd=directory,
                             env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"), check=True,
                             stdout=subprocess.PIPE, stderr=err)
    if run.stdout != reference:
        raise ValueError(f"optimized verifier changed the transcript on {iso}")
    outcomes, private = {}, {}
    for line in (directory / "stderr").read_text().splitlines():
        if not line.startswith("IVY-GOAL "):
            continue
        _, number, phase, value = line.split()
        number, value = int(number), int(value)
        if phase in ("prepush", "scoped-proof"):
            if number in private or value != -1:
                raise ValueError(f"invalid private proof at {iso} check {number}")
            private[number] = phase
        if phase in ("prepush", "scoped"):
            if number in outcomes:
                raise ValueError(f"duplicate optimized result at {iso} check {number}")
            outcomes[number] = value
    if outcomes != concrete or any(concrete.get(i) != -1 for i in private):
        raise ValueError(f"private or concrete result differs from the forced-scope run on {iso}")
    return private


def run(args, engine, binary, iso, cpu):
    path = args.output / (iso + "-" + engine)
    path.mkdir()
    with (path / "stdout").open("wb") as so, (path / "stderr").open("wb") as se:
        subprocess.run(["taskset", "-c", cpu, binary, "isolate=" + iso, args.spec], cwd=path, stdout=so, stderr=se,
                       env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"), check=True)
    lines = (path / "stderr").read_text().splitlines()
    stats = counters.measurements(lines)
    if not stats:
        raise ValueError(f"missing incremental search counts for {path}")
    proven, results = set(), {}
    current = None
    for line in lines:
        if line.startswith("GROUND-BEGIN "):
            current = int(line.split()[1])
        elif line == "GROUND-PROVED":
            if current is None or current in proven:
                raise ValueError(f"unexpected private result in {path}")
            proven.add(current)
        elif line.startswith("GROUND-RESULT "):
            _, n, result = line.split()
            results[int(n)] = int(result)
    if any(results.get(n) != -1 for n in proven):
        raise ValueError(f"the concrete solver did not confirm a private refutation in {path}")
    return (path / "stdout").read_bytes(), stats, proven, results


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for key in ("goivy", "spec", "output"):
        p.add_argument("--" + key, type=Path, required=True)
    p.add_argument("--lane", nargs="+", action="append", required=True, metavar="CPU_OR_ISOLATE",
                   help="one CPU number followed by its isolates, in execution order")
    args = p.parse_args()
    args.goivy, args.spec = args.goivy.resolve(strict=True), args.spec.absolute()
    args.output = args.output.resolve()
    args.output.mkdir(parents=True)
    native = args.output / "native"
    native.mkdir()
    with ThreadPoolExecutor(3) as pool:
        optimized = pool.submit(optimized_checker, args)
        go = pool.submit(compile_go, args)
        z3 = pool.submit(counters.build, args.goivy, native, "checker", True)
        binaries = (go.result(), z3.result())

    fast = optimized.result()

    def lane(items):
        cpu, *isolates = items
        if not isolates:
            raise ValueError("each lane needs an isolate")
        for iso in isolates:
            go = run(args, "g3", binaries[0], iso, cpu)
            z3 = run(args, "native", binaries[1], iso, cpu)
            if go[0] != z3[0] or not go[0].endswith(b"OK\n"):
                raise ValueError(f"the two engines produced different proof transcripts for {iso}")
            if len(go[1]) != len(z3[1]):
                raise ValueError(f"different check counts for {iso}")
            private = optimized_results(args, fast, iso, cpu, go[0], go[3]) if fast else {}
            with (args.output / (iso + ".csv")).open("w") as stream:
                out = csv.writer(stream, lineterminator="\n")
                out.writerow(["isolate", "check", "private_proof", *(["proof_phase", "forced_scoped_proof"] if fast else []), *counters.COUNTERS])
                for i, (a, b) in enumerate(zip(go[1], z3[1]), 1):
                    if a != b:
                        diffs = {k: (v, w) for k, v, w in zip(counters.COUNTERS, a, b) if v != w}
                        raise ValueError(f"incremental search changed on {iso} check {i}: {diffs}")
                    if fast:
                        out.writerow([iso, i, int(i in private), private.get(i, ""), int(i in go[2]), *a])
                    else:
                        out.writerow([iso, i, int(i in go[2]), *a])

    with ThreadPoolExecutor(len(args.lane)) as pool:
        list(pool.map(lane, args.lane))


if __name__ == "__main__":
    main()
