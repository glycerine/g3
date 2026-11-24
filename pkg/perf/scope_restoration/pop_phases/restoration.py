"""Count instructions in individual operations that restore a learned clause or pop relevance marks."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from measure import HERE, PKG, ROOT, WARMUP, make_variable
from variables import go_fixture, native_fixture, run
from widths import go_width_fixture, native_width_fixture


# Source file, containing region, inclusive start, exclusive end.
GO = {
    "bindings": ("smt_clause_backtrack.go", "func (s *SMTInternalizerState) reinitializeLearnedClause",
                 "for saved.bindHead <", "literals := saved.clause.literals"),
    "atom": ("smt_clause_backtrack.go", "func (s *SMTInternalizerState) reinitializeLearnedClause",
             "variable, err := s.restoreLearnedAtom(expression)", "if err != nil {"),
    "publish": ("smt_internalizer_state.go", "func (s *SMTInternalizerState) publishBoolVar",
                "variable := int32(len(s.boolExpr))", "return variable, nil"),
    "queue": ("smt_internalizer_state.go", "func (s *SMTInternalizerState) publishBoolVar",
              "if err := s.basicActivityCreated", "return variable, nil"),
    "watches": ("smt_clause_backtrack.go", "func (s *SMTInternalizerState) reinitializeLearnedClause",
                "if err := s.selectRestoredWatches", "saved.keepReinit = true"),
    "release": ("smt_clause_backtrack.go", "if s.scopeLevel <= s.baseLevel || !saved.keepReinit {",
                "if err := saved.clause.ReleaseAtoms();", "saved.clause.reinit, saved.clause.reinternalizeAtoms"),
    "unmark": ("smt_relevancy.go", "func (p *SMTRelevancyPropagator) PopScopes",
               "for i := len(p.relevant);", "\n\t\t}\n\t\tif err := p.finishPop"),
    "owners": ("smt_relevancy.go", "func (p *SMTRelevancyPropagator) finishPop",
               "p.relevant, err = p.context.manager.ShrinkHeldRefs", "if err != nil {"),
    "filter": ("smt_relevancy.go", "func (p *SMTRelevancyPropagator) finishPop",
               "removals := p.context.exprs.relevanceRemovals", "if err := p.undoTrail"),
}
NATIVE = {
    "bindings": ("smt_context.cpp", "unsigned ilvl       = 0;",
                 "for (unsigned j = 0; j < num; j++) {", "SASSERT(ilvl <= m_scope_lvl);"),
    "atom": ("smt_context.cpp", "unsigned ilvl       = 0;",
             "bool gate_ctx = !m_manager.is_not(atom);", "DEBUG_CODE({"),
    "publish": ("smt_internalizer.cpp", "bool_var context::mk_bool_var(expr * n) {",
                "SASSERT(!b_internalized(n));", "return v;"),
    "queue": ("smt_internalizer.cpp", "bool_var context::mk_bool_var(expr * n) {",
              "m_case_split_queue->mk_var_eh(v);", "m_b_internalized_stack.push_back(n);"),
    "watches": ("smt_context.cpp", "SASSERT(ilvl <= m_scope_lvl);",
                "int w1_idx = select_watch_lit", "if (lit_occs_enabled())"),
    "release": ("smt_context.cpp", "// clause do not need to be in the reinit stack anymore,",
                "cls->release_atoms(m_manager);", "cls->m_reinit "),
    "unmark": ("smt_relevancy.cpp", "void unmark_relevant_exprs(unsigned old_lim)",
               "unsigned i = m_relevant_exprs.size();", "m_relevant_exprs.shrink(old_lim);"),
    "owners": ("smt_relevancy.cpp", "void unmark_relevant_exprs(unsigned old_lim)",
               "m_relevant_exprs.shrink(old_lim);", "m_qhead = m_relevant_exprs.size();"),
}
VISITS = {"bindings": 64, "atom": 192, "publish": 128, "queue": 128, "watches": 64,
          "release": 64, "unmark": 1, "owners": 1, "filter": 1}


def instrument(text, region, event, empty):
    _, anchor, start, stop = region
    left = text.index(start, text.index(anchor))
    right = left if empty else text.index(stop, left + len(start))
    return (text[:left] + event + "(3, true);\n" + text[left:right]
            + "\n" + event + "(3, false);\n" + text[right:])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cpu", required=True, type=int)
    parser.add_argument("--dir", required=True, type=Path, help="new output directory")
    parser.add_argument("--build", default=ROOT / "build", type=Path)
    parser.add_argument("--pairs", type=int, default=9)
    parser.add_argument("--iterations", type=int, help="pops per run, including eight warmups")
    parser.add_argument("--widths", type=int, nargs="+", help="repeat each phase at these even atom counts")
    parser.add_argument("--jobs", type=int, default=2, help="concurrent builds")
    parser.add_argument("--go-overlay", type=Path, help="Go source overlay to measure")
    parser.add_argument("--phase", action="append", choices=GO,
                        help="repeat to select individual operations (default: all)")
    args = parser.parse_args()
    if args.pairs < 1 or (args.iterations is not None and args.iterations <= WARMUP) or args.jobs < 1:
        parser.error("supply positive pairs and jobs, and more than eight iterations")
    if args.widths and any(width < 4 or width % 2 for width in args.widths):
        parser.error("use even widths of at least four atoms")
    phases = list(dict.fromkeys(args.phase or GO))
    replacements = json.loads(args.go_overlay.read_text())["Replace"] if args.go_overlay else {}
    output = args.dir.resolve()
    output.mkdir(parents=True)
    fixture = output / "smt_hotpath_benchmark_test.go"
    fixture.write_text(go_fixture(go_width_fixture(PKG / "smtcore" / fixture.name) if args.widths else None))
    build = args.build.resolve(strict=True)
    config = (build / "config.mk").read_text()
    makefile = (build / "Makefile").read_text()
    compiler = make_variable(config, "CXX")

    def flags(relative):
        name = re.search(r"\$\((includes_\d+)\).*\.\./src/" + re.escape(relative), makefile)[1]
        return make_variable(config, "CXXFLAGS") + make_variable(makefile, name)

    shared = native_fixture(output, build, compiler, flags("test/smt_scope_restoration.cpp"),
                            native_width_fixture() if args.widths else None)

    def prepare(key):
        phase, engine, empty = key
        dest = output / (phase + "-" + engine + ("-empty" if empty else ""))
        dest.mkdir()
        if engine == "g3":
            region = GO[phase]
            src = PKG / "smtcore" / region[0]
            changed = dest / src.name
            source = Path(replacements.get(str(src), src)).read_text()
            changed.write_text(instrument(source, region, "scopePhaseEvent", empty))
            overlay = dest / "overlay.json"
            overlay.write_text(json.dumps({"Replace": {**replacements,
                str(src): str(changed), str(PKG / "smtcore" / fixture.name): str(fixture),
                str(PKG / "smtcore/scope_phase_observer.go"): str(HERE / "thread.go.txt"),
            }}))
            subprocess.run(["go", "test", "-c", "-pgo=off", "-overlay=" + str(overlay),
                            "-o", dest / "bench", "./smtcore"], cwd=PKG, check=True)
        else:
            region = NATIVE[phase]
            src = ROOT / "src/smt" / region[0]
            changed = dest / src.name
            changed.write_text("extern void scope_phase_event(int, bool);\n"
                               + instrument(src.read_text(), region, "scope_phase_event", empty))
            obj = changed.with_suffix(".o")
            subprocess.run([*compiler, *flags("smt/" + src.name), "-o", obj, changed],
                           cwd=build, check=True)
            subprocess.run([*compiler, "-o", dest / "bench", shared, obj, build / "libz3.a",
                            *make_variable(config, "LINK_EXTRA_FLAGS")], cwd=build, check=True)
        return dest / "bench"

    keys = [(p, engine, empty) for p in phases for engine in ("g3", "native")
            if engine == "g3" or p in NATIVE for empty in (True, False)]
    with ThreadPoolExecutor(args.jobs) as pool:
        binaries = dict(zip(keys, pool.map(prepare, keys)))
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow((["width"] if args.widths else []) +
                    ["phase", "mode", "cpu", "pair", "engine", "pops", "intervals",
                     "instructions", "enabled_ns", "running_ns"])
    widths = args.widths or [128]
    fixed_iterations = args.iterations
    for pair in range(args.pairs):
        order = phases[pair % len(phases):] + phases[:pair % len(phases)]
        offset = pair % len(widths)
        for width in widths[offset:] + widths[:offset]:
            if args.widths:
                os.environ["SCOPE_ATOMS"] = str(width)
                args.iterations = fixed_iterations or WARMUP + max(24, min(512, 36864 // width))
            else:
                args.iterations = fixed_iterations or 408
            for phase in order:
                selected = [key for key in keys if key[0] == phase]
                visits = VISITS[phase] * width // 128 if VISITS[phase] > 1 else 1
                for key in (selected if pair % 2 == 0 else reversed(selected)):
                    _, engine, empty = key
                    row = run(binaries[key], engine, args, visits)
                    if row[-1] != row[-2]:
                        raise ValueError(f"multiplexed thread counter for {phase}/{engine}")
                    writer.writerow(([width] if args.widths else []) +
                                    [phase, "empty" if empty else "body", args.cpu, pair + 1,
                                     engine, args.iterations - WARMUP, *row])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
