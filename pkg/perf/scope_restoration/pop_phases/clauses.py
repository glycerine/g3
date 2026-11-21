"""Measure the work per surviving learned clause during the existing scoped-pop benchmark."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import csv
import json
from pathlib import Path
import re
import subprocess
import sys

from measure import HERE, PKG, ROOT, WARMUP, make_variable
from variables import go_fixture, native_fixture, run

PHASES = ("empty", "whole", "atoms", "choose-watches", "install-watches", "finish", "assignments", "retire-atoms")


def insert_events(source, function, end, phase, native):
    start = source.index(function)
    stop = source.index(end, start)
    body = source[start:stop]
    if native:
        first = body.index("SASSERT(cls->in_reinit_stack());")
        atoms = body.index("for (unsigned j = 0; j < num; j++)", body.index("(void)ilvl;"))
        choose = body.index("int w1_idx = select_watch_lit")
        install = body.index("add_watch_literal(cls, 0);", choose)
        finish = body.index("literal l1 = cls->get_literal(0);", install)
        last = body.index("\n            }\n            v.reset();", finish)
        schedule = body.index("if (keep && m_scope_lvl > m_base_lvl)", finish)
        release = body.index("cls->release_atoms(m_manager);", schedule)
        released = body.index("cls->m_reinternalize_atoms = false;", release)
        event = "scope_phase_event"
    else:
        first = body.index("if !saved.installed {")
        atoms = body.index("for saved.bindHead < len(atoms) {")
        choose = body.index("literals := saved.clause.literals", atoms)
        install = body.index("record := &s.lemmas[saved.recordIndex]", choose)
        finish = body.index("literals := saved.clause.literals", install)
        last = body.rindex("return nil")
        schedule = body.index("if err := s.finishLearnedAssignment(", finish)
        release = body.index("if err := saved.clause.ReleaseAtoms();", schedule)
        released = body.index("saved.clause.reinit, saved.clause.reinternalizeAtoms", release)
        event = "scopePhaseEvent"
    left, right = {"empty": (first, first), "whole": (first, last),
                   "atoms": (atoms, choose), "choose-watches": (choose, install),
                   "install-watches": (install, finish), "finish": (finish, last),
                   "assignments": (finish, schedule), "retire-atoms": (release, released)}[phase]
    begin = f"{event}(3, true);\n"
    done = f"\n{event}(3, false);\n"
    return source[:start] + body[:left] + begin + body[left:right] + done + body[right:] + source[stop:]


def build_go(directory, phase, fixture, paths):
    directory.mkdir()
    filename = PKG / "smtcore/smt_clause_backtrack.go"
    original = Path(paths.get(str(filename), filename)).read_text()
    code = insert_events(original, "func (s *SMTInternalizerState) reinitializeLearnedClause(",
                         "func (s *SMTInternalizerState) selectRestoredWatches(", phase, False)
    patched = directory / filename.name
    patched.write_text(code)
    overlay = directory / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {**paths, str(filename): str(patched),
                       str(PKG / "smtcore/smt_hotpath_benchmark_test.go"): str(fixture),
                       str(PKG / "smtcore/scope_phase_observer.go"): str(HERE / "thread.go.txt")}}))
    subprocess.run(["go", "test", "-c", "-pgo=off", "-overlay=" + str(overlay),
                    "-o", directory / "bench", "./smtcore"], cwd=PKG, check=True)


def build_cpp(directory, phase, build, compiler, flags, link, main):
    directory.mkdir()
    source = (ROOT / "src/smt/smt_context.cpp").read_text()
    code = insert_events(source, "void context::reinit_clauses(", "void context::reassert_units(", phase, True)
    patched = directory / "smt_context.cpp"
    patched.write_text("extern void scope_phase_event(int, bool);\n" + code)
    obj = directory / "smt_context.o"
    subprocess.run([*compiler, *flags, "-o", obj, patched], cwd=build, check=True)
    subprocess.run([*compiler, "-o", directory / "bench", main, obj, build / "libz3.a", *link],
                   cwd=build, check=True)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--dir", type=Path, required=True)
    p.add_argument("--build", type=Path, default=ROOT / "build")
    p.add_argument("--go-overlay", type=Path)
    p.add_argument("--iterations", type=int, default=1208)
    p.add_argument("--pairs", type=int, default=9)
    p.add_argument("--jobs", type=int, default=2)
    p.add_argument("--phases", nargs="+", choices=PHASES, default=PHASES)
    args = p.parse_args()
    if args.pairs < 1 or args.jobs < 1 or args.iterations <= WARMUP:
        p.error("supply positive pairs and jobs, and more than eight iterations")
    if "empty" not in args.phases or len(set(args.phases)) != len(args.phases):
        p.error("include the empty control and select each phase only once")
    out = args.dir.resolve()
    out.mkdir(parents=True)
    build = args.build.resolve(strict=True)
    config = (build / "config.mk").read_text()
    makefile = (build / "Makefile").read_text()
    compiler = make_variable(config, "CXX")
    link = make_variable(config, "LINK_EXTRA_FLAGS")

    def flags(source):
        key = re.search(r"\$\((includes_\d+)\).*\.\./src/" + re.escape(source), makefile)[1]
        return make_variable(config, "CXXFLAGS") + make_variable(makefile, key)

    shared = native_fixture(out, build, compiler, flags("test/smt_scope_restoration.cpp"))
    cppflags = flags("smt/smt_context.cpp")
    fixture = out / "fixture.go"
    fixture.write_text(go_fixture())
    paths = json.loads(args.go_overlay.read_text())["Replace"] if args.go_overlay else {}
    with ThreadPoolExecutor(args.jobs) as pool:
        futures = []
        for phase in args.phases:
            futures.append(pool.submit(build_go, out / (phase + "-g3"), phase, fixture, paths))
            futures.append(pool.submit(build_cpp, out / (phase + "-native"), phase, build, compiler,
                                       cppflags, link, shared))
        for job in futures:
            job.result()
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "cpu", "engine", "phase", "intervals", "instructions", "enabled_ns", "running_ns"])
    for i in range(args.pairs):
        offset = i % len(args.phases)
        phases = args.phases[offset:] + args.phases[:offset]
        for phase in phases:
            for engine in (["native", "g3"] if i % 2 else ["g3", "native"]):
                values = run(out / (phase + "-" + engine) / "bench", engine, args, visits_per_pop=64)
                if values[-2] != values[-1]:
                    raise ValueError(f"multiplexed counter in {phase}/{engine}")
                writer.writerow([i + 1, args.cpu, engine, phase, *values])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
