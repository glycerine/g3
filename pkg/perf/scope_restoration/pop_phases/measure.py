"""Measure user instructions in the queue-enabled scope pop, or one part of it."""

import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys

HERE = Path(__file__).resolve().parent
PKG = HERE.parents[2]
sys.path.insert(0, str(PKG / "perf/ivy_apple"))
from native_benchmark import ROOT, make_variable
from phase_counters import EVENTS

PHASES = ("empty", "pop", "relevance", "undo", "restore")
CASES = ("ScopedAtoms", "SurvivingAtoms")
WARMUP = 8
HARDWARE_EVENT = {"instructions:u": 1, "cycles:u": 0, "branches:u": 4, "branch-misses:u": 5}


def replace_once(source, old, new):
    if source.count(old) != 1:
        raise ValueError(f"expected exactly one scope fixture fragment {old!r}")
    return source.replace(old, new)


def instrument_go(fixture, state):
    fixture = replace_once(fixture, "start := time.Now()", f"""openScopePhase()
                    scopePhaseActive = i >= {WARMUP}
                    scopePhaseEvent(0, true)
                    scopePhaseEvent(0, false)
                    scopePhaseEvent(1, true)
                    start := time.Now()""")
    fixture = replace_once(fixture, "elapsed += time.Since(start)", """elapsed += time.Since(start)
                    scopePhaseEvent(1, false)
                    scopePhaseActive = false""")
    report = 'b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N), "pop-ns/op")'
    fixture = replace_once(fixture, report,
                           'b.ReportMetric(float64(scopePhaseVisits), "intervals")\nscopePhaseVisits = 0\n' + report)
    for phase, call in [(2, "s.relevancy.PopScopes(relevancyPops)"), (4, "s.restoreLearnedClauses(pop.learned)")]:
        state = replace_once(state, f"if err := {call}; err != nil {{",
                             f"scopePhaseEvent({phase}, true)\nerr := {call}\nscopePhaseEvent({phase}, false)\nif err != nil {{")
    trail = "for len(s.coreTrail) > scope.coreTrailLimit {"
    state = replace_once(state, trail, "scopePhaseEvent(3, true)\n" + trail)
    after_trail = "if s.flushQuantifiersInPop {"
    state = replace_once(state, after_trail, "scopePhaseEvent(3, false)\n" + after_trail)
    return fixture, state


def build_go(directory, overlay, thread_event):
    directory.mkdir()
    paths = json.loads(overlay.read_text())["Replace"] if overlay else {}
    fixture = PKG / "smtcore/smt_hotpath_benchmark_test.go"
    state = PKG / "smtcore/smt_internalizer_state.go"
    originals = [Path(paths.get(str(path), path)).read_text() for path in (fixture, state)]
    fixture_text, state_text = instrument_go(*originals)
    if thread_event:
        fixture_text = replace_once(fixture_text, "openScopePhase()", "")
        start = "var elapsed time.Duration"
        fixture_text = replace_once(fixture_text, start, start + "\nstartScopeCounter()\ndefer finishScopeCounter()")
    for path, data in [(fixture, fixture_text), (state, state_text)]:
        changed = directory / path.name
        changed.write_text(data)
        paths[str(path)] = str(changed)
    paths[str(PKG / "smtcore/scope_phase_observer.go")] = str(HERE / ("thread.go.txt" if thread_event else "observer.go.txt"))
    overlay_file = directory / "overlay.json"
    overlay_file.write_text(json.dumps({"Replace": paths}))
    binary = directory / "bench"
    subprocess.run(["go", "test", "-c", "-pgo=off", "-overlay=" + str(overlay_file),
                    "-o", binary, "./smtcore"], cwd=PKG, check=True)
    return binary


def build_native(directory, build, thread_event, fixture=None):
    directory.mkdir()
    config = (build / "config.mk").read_text()
    makefile = (build / "Makefile").read_text()
    includes = re.search(r"\$\((includes_\d+)\).*\.\./src/test/smt_scope_restoration\.cpp", makefile)[1]
    compiler = make_variable(config, "CXX")
    flags = make_variable(config, "CXXFLAGS") + make_variable(makefile, includes)
    if thread_event:
        flags.append("-DG3_PHASE_THREAD")
    context = (ROOT / "src/smt/smt_context.cpp").read_text()
    for phase, call in [(2, "m_relevancy_propagator->pop(num_scopes);"),
                        (3, "undo_trail_stack(s.m_trail_stack_lim);"),
                        (4, "reinit_clauses(num_scopes, num_bool_vars);")]:
        context = replace_once(context, call, f"{{scope_phase_event({phase}, true); {call} scope_phase_event({phase}, false);}}")
    context_file = directory / "smt_context.cpp"
    context_file.write_text("extern void scope_phase_event(int, bool);\n" + context)
    if fixture is None:
        fixture = (ROOT / "src/test/smt_scope_restoration.cpp").read_text()
    fixture = replace_once(fixture, "auto start = clock::now();", f"""
        scope_phase_active = round >= {WARMUP};
        scope_phase_event(0, true);
        scope_phase_event(0, false);
        scope_phase_event(1, true);
        auto start = clock::now();""")
    fixture = replace_once(fixture, "elapsed += clock::now() - start;", """elapsed += clock::now() - start;
        scope_phase_event(1, false);
        scope_phase_active = false;""")
    (directory / "smt_scope_restoration.cpp").write_text(fixture)
    objects = []
    for source in (context_file, ROOT / "src/test/perf/scope_phase_main.cpp"):
        obj = directory / source.with_suffix(".o").name
        subprocess.run([*compiler, "-I", directory, *flags, "-o", obj, source], cwd=build, check=True)
        objects.append(obj)
    binary = directory / "bench"
    subprocess.run([*compiler, "-o", binary, *objects, build / "libz3.a", *make_variable(config, "LINK_EXTRA_FLAGS")],
                   cwd=build, check=True)
    return binary


def measure(directory, binary, variant, case, phase, args):
    directory.mkdir()
    control, ack = directory / "control", directory / "ack"
    env = dict(os.environ, GOMAXPROCS="1", SCOPE_PHASE=str(PHASES.index(phase)))
    command = ["taskset", "-c", str(args.cpu)]
    counts = directory / "perf.csv"
    if args.thread_event:
        env["SCOPE_THREAD_EVENT"] = str(HARDWARE_EVENT[args.thread_event])
    else:
        os.mkfifo(control, 0o600)
        os.mkfifo(ack, 0o600)
        env.update(SCOPE_CONTROL=str(control), SCOPE_ACK=str(ack))
        command.extend([args.perf, "stat", "-x", ",", "-o", counts, "-D", "-1",
                        "--control", f"fifo:{control},{ack}", "-e", ",".join(EVENTS), "--"])
    command.append(binary)
    if variant == "native":
        command.extend([str(args.iterations), case])
    else:
        command.extend(["-test.run", "^$", "-test.bench", f"^BenchmarkSMTScopeRestorationWithQueue$/^{case}$",
                        f"-test.benchtime={args.iterations}x"])
    try:
        with (directory / "out").open("w") as out, (directory / "err").open("w") as err:
            subprocess.run(command, cwd=directory, env=env, stdout=out, stderr=err, check=True, timeout=args.timeout)
    finally:
        if not args.thread_event:
            control.unlink()
            ack.unlink()
    result = (directory / "out").read_text()
    match = re.search(r"intervals (\d+)" if variant == "native" else r"(?<![\d.])(\d+)(?:\.0+)? intervals", result)
    if not match or int(match[1]) != args.iterations - WARMUP or "BenchmarkSMTScopeRestoration" not in result:
        raise ValueError(f"unexpected interval count or missing benchmark in {directory / 'out'}")
    if args.thread_event:
        # Go writes an extra row for its one-iteration benchmark calibration.
        counters = re.findall(r"SCOPE-THREAD (\d+) (\d+) (\d+)$", result, re.MULTILINE)
        if not counters:
            raise ValueError(f"missing thread counter in {directory / 'out'}")
        count, enabled, running = map(int, counters[-1])
        if not 0 < running <= enabled:
            raise ValueError(f"invalid thread counter running time in {directory / 'out'}")
        return [int(match[1]), count, enabled, running]
    with counts.open() as source:
        values = {row[2]: int(row[0]) for row in csv.reader(source) if len(row) > 2 and row[2] in EVENTS}
    return [int(match[1]), *[values[key] for key in EVENTS]]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--pairs", type=int, default=7)
    parser.add_argument("--iterations", type=int, default=3008)
    parser.add_argument("--timeout", type=int, default=180, help="maximum seconds per measurement")
    parser.add_argument("--thread-event", choices=HARDWARE_EVENT,
                        help="count only the solver's OS thread with Linux/amd64 perf_event_open")
    parser.add_argument("--perf", default="perf")
    parser.add_argument("--build", type=Path, default=ROOT / "build")
    parser.add_argument("--dir", type=Path, required=True, help="new directory for binaries, counters and logs")
    parser.add_argument("--go-overlay", nargs=2, action="append", default=[], metavar=("LABEL", "PATH"))
    parser.add_argument("--case", choices=CASES, default=CASES[0])
    parser.add_argument("--phases", nargs="+", choices=PHASES, default=PHASES)
    args = parser.parse_args()
    if args.iterations <= WARMUP or args.pairs < 1 or args.timeout < 1:
        parser.error("supply positive pairs and timeout, and more than eight iterations")
    if args.case == "SurvivingAtoms" and "restore" in args.phases:
        parser.error("choose phases executed by the surviving case, for example --phases empty pop")
    names = ["g3", "native", *[label for label, _ in args.go_overlay]]
    if len(set(names)) != len(names) or any(not re.fullmatch(r"[A-Za-z0-9_-]+", name) for name in names):
        parser.error("choose distinct alphanumeric Go labels, optionally containing '_' or '-'")
    directory = args.dir.resolve()
    directory.mkdir(parents=True)
    binaries = {"g3": build_go(directory / "g3", None, args.thread_event),
                "native": build_native(directory / "native", args.build.resolve(strict=True), args.thread_event)}
    for name, overlay in args.go_overlay:
        binaries[name] = build_go(directory / name, Path(overlay).resolve(strict=True), args.thread_event)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    fields = [args.thread_event, "enabled_ns", "running_ns"] if args.thread_event else EVENTS
    writer.writerow(["case", "phase", "cpu", "pair", "engine", "intervals", *fields])
    for pair in range(args.pairs):
        offset = pair % len(names)
        order = names[offset:] + names[:offset]
        if pair % 2:
            order.reverse()
        for phase in args.phases:
            for name in order:
                sample = directory / f"{args.case}-{phase}-{pair+1}-{name}"
                values = measure(sample, binaries[name], name, args.case, phase, args)
                writer.writerow([args.case, phase, args.cpu, pair+1, name, *values])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
