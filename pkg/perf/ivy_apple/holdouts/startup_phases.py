"""Measure individual default-tactic constructors on the thread that asserts the first formula."""

import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from startup import HERE, PKG
from native_benchmark import ROOT, make_variable
from phase_counters import replace_once


# Order of the Go branch factories. mk_default_tactic in C++ nests the same
# branches in reverse order, with plain SMT as the innermost alternative.
BRANCHES = ("qffplra", "qffp", "nra", "lira", "qfnia", "qfnra", "qflra", "qfauflia", "qflia", "qfaufbv", "qfbv")
PHASES = ("empty", *BRANCHES, "simplify", "smt", "whole")
CPP = ROOT / "src/test/perf"


def build_go(directory, overlay):
    directory.mkdir()
    paths = json.loads(overlay.read_text())["Replace"] if overlay else {}

    def source(name):
        path = PKG / name
        return Path(paths.get(str(path), path)).read_text()

    def write(name, contents):
        target = directory / Path(name).name
        target.write_text(contents)
        paths[str(PKG / name)] = str(target)

    helper = (PKG / "perf/scope_restoration/pop_phases/thread.go.txt").read_text()
    write("tactic/portfolio/strategic_study.go", helper.replace("package smtcore", "package portfolio") + """
func StudyStart() { startScopeCounter(); scopePhaseVisits=0; scopePhaseActive=true }
func StudyEvent(id int, begin bool) { scopePhaseEvent(id, begin) }
func StudyEnd() { scopePhaseActive=false; fmt.Printf("STARTUP-VISITS %d\\n",scopePhaseVisits); finishScopeCounter() }
""")
    name = "tactic/portfolio/smt_strategic_solver.go"
    code = source(name)
    start = code.index("func NewDefaultStrategicSolver(")
    end = code.index("func ", code.index("\n}", start))
    body = code[start:end]
    variables = ("qfFPLRA", "qfFP", "nra", "lira", "qfNIA", "qfNRA", "qfLRA", "qfAUFLIA", "qfLIA", "qfAUFBV", "qfBV", "simplify", "ordinaryValue")
    for phase, variable in enumerate(variables, 1):
        pattern = r"(?m)^\s*(" + variable + r", err := [^\n]+)"
        body, matches = re.subn(pattern, lambda m: f"\nscopePhaseEvent({phase},true)\n{m[1]}\nscopePhaseEvent({phase},false)", body)
        if matches != 1:
            raise ValueError(f"expected one constructor for {variable}")
    write(name, code[:start] + body + code[end:])
    name = "api/api_solver_startup_benchmark_test.go"
    code = replace_once(source(name), 'import "testing"', 'import "testing"\nimport "github.com/glycerine/g3/pkg/tactic/portfolio"')
    code = replace_once(code, "b.ResetTimer()", "b.ResetTimer()\nportfolio.StudyStart()")
    code = replace_once(code, "s, err := ctx.NewSolver()", "portfolio.StudyEvent(0,true);portfolio.StudyEvent(0,false)\nportfolio.StudyEvent(14,true)\ns, err := ctx.NewSolver()")
    code = replace_once(code, "\n\t\t\t}\n\t\t\tb.StopTimer()", "\nportfolio.StudyEvent(14,false)\n}\nportfolio.StudyEnd()\nb.StopTimer()")
    write(name, code)
    mapping = directory / "overlay.json"
    mapping.write_text(json.dumps({"Replace": paths}))
    subprocess.run(["go", "test", "-pgo=off", "-overlay=" + str(mapping), "-c", "-o", directory / "bench", "./api"], cwd=PKG, check=True)
    return directory / "bench"


def build_cpp(directory, build):
    directory.mkdir()
    code = (ROOT / "src/tactic/portfolio/default_tactic.cpp").read_text()
    begin = code.index("tactic * mk_default_tactic(")
    declaration = """
extern void startup_event(int, bool);
template<typename F> static tactic* measure_startup(int id, F const& make) {
    startup_event(id, true); tactic* t=make(); startup_event(id, false); return t;
}
"""
    prefix, body = code[:begin] + declaration, code[begin:]
    for phase, name in enumerate((*BRANCHES, "simplify"), 1):
        pattern = r"\bmk_" + name + r"_tactic\(m(?:, p)?\)"
        body, matches = re.subn(pattern, lambda m: f"measure_startup({phase}, [&](){{return {m[0]};}})", body)
        if matches != 1:
            raise ValueError(f"expected one constructor for {name}")
    unit = directory / "default_tactic.cpp"
    unit.write_text(prefix + replace_once(body, "mk_smt_tactic()", "measure_startup(13, [&](){return mk_smt_tactic();})"))
    code = (CPP / "strategic_solver_startup.cpp").read_text()
    code = replace_once(code, "auto start =", 'open_counter(); phase=std::atoi(std::getenv("SCOPE_PHASE"));\n    auto start =')
    code = replace_once(code, "Z3_solver solver =", "startup_event(0,true); startup_event(0,false); startup_event(14,true);\n        Z3_solver solver =")
    code = replace_once(code, "Z3_solver_dec_ref(ctx, solver);", "Z3_solver_dec_ref(ctx, solver); startup_event(14,false);")
    code = replace_once(code, "auto elapsed =", 'std::printf("STARTUP-VISITS %ld\\n",visits); report_counter(); phase=-1;\n    auto elapsed =')
    fixture = directory / "fixture.cpp"
    fixture.write_text(f'#include "{CPP / "thread_counter.h"}"\n' + """
static int phase=-1;
static long visits;
void startup_event(int id,bool begin) { if(id==phase) { if(begin) ++visits; signal_counter(begin); } }
""" + code)
    config = (build / "config.mk").read_text()
    makefile = (build / "Makefile").read_text()
    cxx = make_variable(config, "CXX")
    include = re.search(r"\$\((includes_\d+)\).*\.\./src/tactic/portfolio/default_tactic.cpp", makefile)[1]
    obj = unit.with_suffix(".o")
    subprocess.run([*cxx, *make_variable(config, "CXXFLAGS"), *make_variable(makefile, include), "-o", obj, unit], cwd=build, check=True)
    exe = directory / "bench"
    subprocess.run([*cxx, "-std=c++17", "-O3", "-DNDEBUG", "-I" + str(ROOT / "src/api"), "-o", exe,
                    fixture, obj, build / "libz3.a", *make_variable(config, "LINK_EXTRA_FLAGS")], cwd=build, check=True)
    return exe


def measure(binary, native, phase, mode, args):
    n = str(args.iterations)
    cmd = [mode, n] if native else ["-test.run", "^$", "-test.bench", "^BenchmarkStrategicSolverFirstAssertion$/^" + mode + "$", "-test.benchtime=" + n + "x"]
    out = subprocess.check_output(["taskset", "-c", str(args.cpu), binary, *cmd], text=True,
                                  env=dict(os.environ, GOMAXPROCS="1", SCOPE_PHASE=str(phase), SCOPE_THREAD_EVENT="1"))
    values = re.findall(r"STARTUP-VISITS (\d+)\s+SCOPE-THREAD (\d+) (\d+) (\d+)", out)
    if not values or values[-1][0] != n or values[-1][2] != values[-1][3] or int(values[-1][3]) <= 0:
        raise ValueError(f"missing or multiplexed stage for {mode} {PHASES[phase]}\n{out}")
    return values[-1]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--dir", required=True, type=Path)
    p.add_argument("--build", type=Path, default=ROOT / "build")
    p.add_argument("--go", nargs=2, action="append", metavar=("LABEL", "OVERLAY"), help="use '-' for the current source")
    p.add_argument("--cpu", required=True, type=int)
    p.add_argument("--iterations", default=8, type=int)
    p.add_argument("--rounds", default=4, type=int)
    args = p.parse_args()
    variants = args.go or [("g3", "-")]
    labels = [v[0] for v in variants] + ["native"]
    if args.iterations < 2 or args.rounds < 1 or len(set(labels)) != len(labels) or any(not re.fullmatch(r"[\w-]+", v) for v in labels):
        p.error("supply distinct engine labels, at least two iterations and a positive round count")
    directory = args.dir.resolve()
    directory.mkdir(parents=True)
    engines = [(name, build_go(directory / name, None if path == "-" else Path(path).resolve(strict=True))) for name, path in variants]
    engines.append(("native", build_cpp(directory / "native", args.build.resolve(strict=True))))
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "cpu", "engine", "mode", "phase", "visits", "instructions", "enabled_ns", "running_ns"])
    for i in range(args.rounds):
        j = i % len(engines)
        order = engines[j:] + engines[:j]
        for phase, label in enumerate(PHASES):
            for mode in ("Default", "MacroFinder"):
                for engine, binary in order if i % 2 == 0 else order[::-1]:
                    writer.writerow([i+1, args.cpu, engine, mode, label, *measure(binary, engine == "native", phase, mode, args)])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
