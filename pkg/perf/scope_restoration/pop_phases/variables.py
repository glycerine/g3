"""Count instructions per retired Boolean in the queue-enabled scoped pop on Linux/amd64."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from measure import HERE, PKG, ROOT, WARMUP, make_variable, replace_once


PHASES = ("empty", "whole", "queue", "binding", "slots", "release", "rest")
VARIABLES = 128


def instrument(source, first, last, boundaries, phase, event):
    """Place just two calls in the chosen region, so other probes cost nothing."""
    start = source.index(first)
    end = source.index(last, start) + len(last)
    body = source[start:end]
    left, right = boundaries(body)[phase]
    return (source[:start] + body[:left] + event + "(3, true);\n" + body[left:right]
            + "\n" + event + "(3, false);\n" + body[right:] + source[end:])


def go_boundaries(body):
    queue = body.index("if (s.cachedActivity")
    binding = body.index("if s.quantifiers != nil")
    slots = body.index("s.boolExpr = s.boolExpr[:variable]")
    retry = body.index("s.retiringBoolean, s.retiringBooleanRoot = true")
    release = body.index("if !s.retiringBooleanRoot.IsNull()")
    done = body.index("s.retiringBoolean = false")
    return {"empty": (0, 0), "whole": (0, len(body)), "queue": (queue, binding),
            "binding": (binding, slots), "slots": (slots, retry),
            "release": (release, done), "rest": (binding, len(body))}


def native_boundaries(body):
    call = "m_case_split_queue->del_var_eh(v);"
    queue = body.index(call)
    binding = queue + len(call)
    release = body.index("m_b_internalized_stack.pop_back();")
    return {"empty": (0, 0), "whole": (0, len(body)), "queue": (queue, binding),
            "binding": (binding, release), "slots": (release, release),
            "release": (release, len(body)), "rest": (binding, len(body))}


def go_fixture():
    source = (PKG / "smtcore/smt_hotpath_benchmark_test.go").read_text()
    source = replace_once(source, "var elapsed time.Duration",
                          "var elapsed time.Duration\nstartScopeCounter()\ndefer finishScopeCounter()")
    source = replace_once(source, "start := time.Now()",
                          f"scopePhaseActive = i >= {WARMUP}\nstart := time.Now()")
    source = replace_once(source, "elapsed += time.Since(start)",
                          "elapsed += time.Since(start)\nscopePhaseActive = false")
    return replace_once(source, "b.ReportMetric(float64(elapsed.Nanoseconds())",
                        'b.ReportMetric(float64(scopePhaseVisits), "intervals")\nscopePhaseVisits = 0\n'
                        'b.ReportMetric(float64(elapsed.Nanoseconds())')


def build_go(output, phase, fixture):
    output.mkdir()
    state = PKG / "smtcore/smt_internalizer_state.go"
    code = state.read_text()
    start = code.index("for len(s.coreTrail) > scope.coreTrailLimit")
    patched = instrument(code[start:], "expression := s.booleanStack[len(s.booleanStack)-1]",
                         "s.statistics.NumDelBoolVar++", go_boundaries, phase, "scopePhaseEvent")
    (output / state.name).write_text(code[:start] + patched)
    paths = {str(state): str(output / state.name),
             str(PKG / "smtcore/smt_hotpath_benchmark_test.go"): str(fixture),
             str(PKG / "smtcore/scope_phase_observer.go"): str(HERE / "thread.go.txt")}
    overlay = output / "overlay.json"
    overlay.write_text(json.dumps({"Replace": paths}))
    subprocess.run(["go", "test", "-c", "-pgo=off", "-overlay=" + str(overlay),
                    "-o", output / "bench", "./smtcore"], cwd=PKG, check=True)


def native_fixture(output, build, compiler, flags):
    fixture = (ROOT / "src/test/smt_scope_restoration.cpp").read_text()
    fixture = replace_once(fixture, "auto start = clock::now();",
                           f"scope_phase_active = round >= {WARMUP};\nauto start = clock::now();")
    fixture = replace_once(fixture, "elapsed += clock::now() - start;",
                           "elapsed += clock::now() - start;\nscope_phase_active = false;")
    (output / "smt_scope_restoration.cpp").write_text(fixture)
    obj = output / "scope_phase_main.o"
    subprocess.run([*compiler, "-DG3_PHASE_THREAD", "-I", output, *flags, "-o", obj,
                    ROOT / "src/test/perf/scope_phase_main.cpp"], cwd=build, check=True)
    return obj


def build_native(output, phase, build, compiler, flags, link, main):
    output.mkdir()
    source = (ROOT / "src/smt/smt_internalizer.cpp").read_text()
    start = source.index("void context::undo_mk_bool_var()")
    patched = instrument(source[start:], "SASSERT(", "m_b_internalized_stack.pop_back();",
                         native_boundaries, phase, "scope_phase_event")
    file = output / "smt_internalizer.cpp"
    file.write_text("extern void scope_phase_event(int, bool);\n" + source[:start] + patched)
    obj = file.with_suffix(".o")
    subprocess.run([*compiler, *flags, "-o", obj, file], cwd=build, check=True)
    subprocess.run([*compiler, "-o", output / "bench", main, obj,
                    build / "libz3.a", *link], cwd=build, check=True)


def run(binary, engine, args):
    command = ["taskset", "-c", str(args.cpu), binary]
    if engine == "g3":
        command.extend(["-test.run", "^$", "-test.bench",
                        "^BenchmarkSMTScopeRestorationWithQueue$/^ScopedAtoms$",
                        f"-test.benchtime={args.iterations}x"])
    else:
        command.extend([str(args.iterations), "ScopedAtoms"])
    env = dict(os.environ, SCOPE_THREAD_EVENT="1", SCOPE_PHASE="3", GOMAXPROCS="1")
    result = subprocess.check_output(command, cwd=binary.parent, env=env, text=True)
    matches = re.findall(r"(\d+) intervals" if engine == "g3" else r"intervals (\d+)", result)
    expected = VARIABLES * (args.iterations - WARMUP)
    if not matches or int(matches[-1]) != expected:
        raise ValueError(f"expected {expected} variable intervals from {binary}")
    counters = re.findall(r"SCOPE-THREAD (\d+) (\d+) (\d+)$", result, re.MULTILINE)
    if not counters:
        raise ValueError(f"missing thread counter from {binary}")
    count, enabled, running = map(int, counters[-1])
    if not 0 < running <= enabled:
        raise ValueError(f"invalid thread counter running time in {binary}")
    return expected, count, enabled, running


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--dir", type=Path, required=True, help="new output directory")
    parser.add_argument("--build", type=Path, default=ROOT / "build")
    parser.add_argument("--pairs", type=int, default=9)
    parser.add_argument("--iterations", type=int, default=1208)
    parser.add_argument("--jobs", type=int, default=2, help="concurrent builds before measurements")
    args = parser.parse_args()
    if args.pairs < 1 or args.iterations <= WARMUP or args.jobs < 1:
        parser.error("supply positive pairs and jobs, and more than eight iterations")
    output = args.dir.resolve()
    output.mkdir(parents=True)
    build = args.build.resolve(strict=True)
    config = (build / "config.mk").read_text()
    makefile = (build / "Makefile").read_text()
    compiler = make_variable(config, "CXX")
    common = make_variable(config, "CXXFLAGS")
    link = make_variable(config, "LINK_EXTRA_FLAGS")

    def flags_for(relative):
        includes = re.search(r"\$\((includes_\d+)\).*\.\./src/" + re.escape(relative), makefile)[1]
        return common + make_variable(makefile, includes)

    shared = native_fixture(output, build, compiler, flags_for("test/smt_scope_restoration.cpp"))
    flags = flags_for("smt/smt_internalizer.cpp")
    fixture = output / "smt_hotpath_benchmark_test.go"
    fixture.write_text(go_fixture())
    with ThreadPoolExecutor(args.jobs) as pool:
        jobs = []
        for phase in PHASES:
            jobs.append(pool.submit(build_go, output / (phase + "-g3"), phase, fixture))
            jobs.append(pool.submit(build_native, output / (phase + "-native"),
                                    phase, build, compiler, flags, link, shared))
        for job in jobs:
            job.result()

    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["phase", "cpu", "pair", "engine", "scoped_pops", "variable_intervals",
                     "instructions", "enabled_ns", "running_ns"])
    for i in range(args.pairs):
        offset = i % len(PHASES)
        for phase in PHASES[offset:] + PHASES[:offset]:
            for engine in (("g3", "native") if i % 2 == 0 else ("native", "g3")):
                binary = output / f"{phase}-{engine}/bench"
                writer.writerow([phase, args.cpu, i+1, engine, args.iterations-WARMUP,
                                 *run(binary, engine, args)])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
