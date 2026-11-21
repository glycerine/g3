"""Count rewriter and SMT operations during a specified GoIvy verification call or external push."""
import argparse
import csv
import json
import os
import shutil
import subprocess
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

from measure import go_overlay, instrument_queue, replace_once
from phase_counters import G3_METHOD, NATIVE_METHOD

EVENTS = ("visits", "app_resumes", "quantifier_resumes", "cache_find", "cache_insert", "bool_lookup",
          "result_push", "result_drop", "internalize", "mk_clause", "expr_table_read")
PHASES = ("setup", "candidate_check", "substitution", "rewrite", "lemma", "internalize", "cleanup", "outside_candidate")
GO_COUNTER = "github.com/glycerine/g3/pkg/util/lbool"


def native_library(args, out):
    # Template and inline-method probes need recompilation of all their consumers.
    src, build = out / "src", out / "build"
    for original, private in ((args.g3.parent / "src", src), (args.build, build)):
        subprocess.run(["cp", "-a", "--reflink=auto", original, private], check=True)
    header = "util/qi_operation_measurement.h"
    shutil.copyfile(src / "test/perf/qi_operation_measurement.h", src / header)

    def patch(filename, transform):
        file = src / filename
        file.write_text(f'#include "{header}"\n' + transform(file.read_text()))

    def events(filename, methods):
        def transform(code):
            for method, body in methods:
                code = replace_once(code, method, method + "\n" + body)
            return code
        patch(filename, transform)

    def rewriter(code):
        for method, call in (
                ("visit(expr * t, unsigned max_depth) {", "qi_profile::event(0); qi_profile::visit(t);"),
                ("process_app(app * t, frame & fr) {", "qi_profile::event(1);"),
                ("process_quantifier(quantifier * q, frame & fr) {", "qi_profile::event(2);")):
            anchor = "rewriter_tpl<Config>::" + method
            code = replace_once(code, anchor, anchor + "\n" + call)
        for method, column in (("push_back", 6), ("pop_back", 7)):
            anchor = f"result_stack().{method}("
            if anchor not in code:
                raise ValueError(f"missing result-stack {method}")
            code = code.replace(anchor, f"(qi_profile::event({column}), result_stack()).{method}(")
        return code.replace("result_stack().shrink(fr.m_spos)",
                            "result_stack().shrink(qi_profile::shrink(result_stack().size(),fr.m_spos))")

    patch("ast/rewriter/rewriter_def.h", rewriter)
    events("ast/act_cache.cpp", [("void act_cache::insert(expr * k, expr * v) {", "qi_profile::event(4);"),
                                 ("expr * act_cache::find(expr * k) {", "qi_profile::event(3);")])
    events("smt/smt_context.h", [(signature, "qi_profile::event(5);") for signature in (
        "bool_var get_bool_var_of_id_option(unsigned id) const {", "bool_var get_bool_var_of_id(unsigned id) const {",
        "bool_var get_bool_var(expr const * n) const {")])
    events("smt/smt_internalizer.cpp", [
        ("void context::internalize(expr * n, bool gate_ctx) {", "qi_profile::event(8);"),
        ("clause * context::mk_clause(unsigned num_lits, literal * lits, justification * j, clause_kind k, clause_del_eh * del_eh) {",
         "qi_profile::event(9);")])
    events("api/api_solver.cpp", [(signature, "qi_profile::window window;") for signature in (
        "Z3_lbool Z3_API Z3_solver_check(Z3_context c, Z3_solver s) {",
        "Z3_lbool Z3_API Z3_solver_check_assumptions(Z3_context c, Z3_solver s, unsigned num_assumptions, Z3_ast const assumptions[]) {",
        "void Z3_API Z3_solver_push(Z3_context c, Z3_solver s) {")])
    events("smt/asserted_formulas.cpp", [("if (!s.should_apply()) return true;", "qi_profile::pass pass(s.id());")])
    # Retire *each* deleted address in the cascade. An allocator can recycle both
    # an address and an AST ID within one simplification pass.
    events("ast/ast.cpp", [("n = worklist.back();\n        worklist.pop_back();", "qi_profile::retire(n);")])
    patch("smt/qi_queue.cpp", lambda code: '#include "test/perf/qi_operation_measurement.cpp"\n'
          "using qi_observation = qi_profile::candidate;\n" + instrument_queue(code))
    with (out / "make.log").open("w") as log:
        subprocess.run(["make", "-j", str(args.jobs), "libz3.a"], cwd=build, stdout=log, stderr=log, check=True)
    return build / "libz3.a"


def write_overlay(mapping, original, directory, code):
    dest = directory / original.name
    dest.write_text(code)
    mapping[str(original)] = mapping[str(original.resolve())] = str(dest)


def instrument_go(args, out):
    mapping = go_overlay(args, out)
    local = Path(subprocess.check_output(["go", "list", "-mod=readonly", "-m", "-f", "{{.Dir}}",
                                          "github.com/glycerine/g3/pkg"], cwd=args.goivy, text=True).strip())

    def save(path, code):
        write_overlay(mapping, local / path, out, code)

    save("smtcore/qi_candidate_measurement.go", """
package smtcore
import ops "github.com/glycerine/g3/pkg/util/lbool"
type qiOperation struct {outcome string; previous *qiOperation; prior int}
var qiObserved *qiOperation
func qiStartEntry(q *SMTQuantifierManager, e SMTQIQueueEntry) {
    if ops.IvyPhase != 0 { qiObserved = &qiOperation{previous:qiObserved, prior:ops.IvyPhase}; ops.IvyPhase=1 }
}
func qiNextPhase(p int) { if qiObserved!=nil {ops.IvyPhase=p+1} }
func qiEndCounter() { if qiObserved!=nil {ops.IvyPhase=qiObserved.prior; qiObserved=qiObserved.previous} }
""")
    save("util/lbool/qi_operation_counts.go", """
package lbool
import ("fmt"; "os")
var IvyPhase, ivyPassOrdinal int
var IvyOps [9][11]uint64
type ivyHandle struct {origin uint64; slot, generation uint32}
type ivyPass struct {name string; ordinal, visits int; distinct map[ivyHandle]struct{}}
var ivyPasses []ivyPass
func IvyEvent(column, count int) {if IvyPhase!=0 {IvyOps[IvyPhase][column]+=uint64(count)}}
func IvyVisit(origin uint64, slot, generation uint32) {
    if len(ivyPasses)>0 {p:=&ivyPasses[len(ivyPasses)-1];p.visits++;p.distinct[ivyHandle{origin,slot,generation}]=struct{}{}}
}
func IvyPass(name string) func() {
    if IvyPhase==0 {return func(){}}
    ivyPassOrdinal++
    ivyPasses=append(ivyPasses,ivyPass{name:name,ordinal:ivyPassOrdinal,distinct:make(map[ivyHandle]struct{})})
    return func(){
        i:=len(ivyPasses)-1;p:=ivyPasses[i]
        fmt.Fprintln(os.Stderr,"IVY-PASS",p.ordinal,p.name,p.visits,len(p.distinct))
        ivyPasses[i]=ivyPass{};ivyPasses=ivyPasses[:i]
    }
}
""")
    methods = {
        "ast/rewriter/rewriter_engine.go": [
            ("visit(t ast.ASTHandle, depth uint32) (bool, error) {", "IvyEvent(0,1); PKG.IvyVisit(t.Origin(),t.Slot(),t.Generation())"),
            ("processApp() (err error) {", "IvyEvent(1,1)"), ("processQuantifier() (err error) {", "IvyEvent(2,1)")],
        "ast/ast_activity_cache.go": [("Find(k ASTHandle) (ASTHandle, error) {", "IvyEvent(3,1)"),
                                      ("Insert(k, v ASTHandle) (err error) {", "IvyEvent(4,1)")],
        "smtcore/smt_expr_table.go": [
            ("func (b smtBoolVarTable) lookup(h ast.ASTHandle) (int32, bool) {", "IvyEvent(5,1)"),
            ("func (t *smtExprTable) read(h ast.ASTHandle) *smtExprRecord {", "IvyEvent(10,1)")],
        "ast/rewriter/rewriter_roots.go": [
            ("func appendRewriterRoot(m *ast.ASTManager, roots *[]ast.ASTHandle, h ast.ASTHandle) error {", "IvyEvent(6,1)"),
            ("func shrinkRewriterRoots(m *ast.ASTManager, roots *[]ast.ASTHandle, n uint32) error {", "IvyEvent(7,len(*roots)-int(n))")],
        "smtcore/smt_internalizer.go": [
            ("Internalize(expression ast.ASTHandle, gateContext bool) error {", "IvyEvent(8,1)"),
            ("addClauseKindWithJustificationAndDeleteHandler(literals []SMTLiteral, root bool, kind SMTClauseKind, proof ast.ASTHandle, justification SMTJustification, deleteHandler SMTClauseDeleteHandler) error {",
             "IvyEvent(9,1)")],
        "smtcore/smt_asserted_driver.go": [('if err := assertedVerbose(10, "(smt."+pass.sourceName()', None)],
    }
    for name, hooks in methods.items():
        file = local / name
        code = file.read_text()
        pkg = "lbool"
        if f'"{GO_COUNTER}"' not in code:
            pkg = "qiops"
            package = "package " + file.parent.name + "\n"
            code = replace_once(code, package, package + f'import qiops "{GO_COUNTER}"\n')
        for anchor, statement in hooks:
            if statement is None:
                code = replace_once(code, anchor, f"defer {pkg}.IvyPass(pass.sourceName())()\n" + anchor)
            else:
                code = replace_once(code, anchor, anchor + "\n" + pkg + "." + statement.replace("PKG", pkg))
        save(name, code)
    return mapping


WATCH = """
var qiOrdinal, qiInside int
func qiWatch(kind string) func() {
    if kind=="push" && qiInside>0 {return func(){}}
    if kind=="check" {qiInside++}
    unnest:=func(){if kind=="check"{qiInside--}}
    if os.Getenv("IVY_QI_TARGET")!=kind {return unnest}
    qiOrdinal++
    if strconv.Itoa(qiOrdinal)!=os.Getenv("IVY_QI_CHECK"){return unnest}
    runtime.LockOSThread()
    if err:=os.Setenv("IVY_QI_ACTIVE","yes");err!=nil{panic(err)}
    START
    return func(){
        END
        if err:=os.Unsetenv("IVY_QI_ACTIVE");err!=nil{panic(err)}
        runtime.UnlockOSThread()
        unnest()
    }
}
"""


def compile_checker(args, engine):
    out = args.output / engine
    out.mkdir()
    native = engine == "native"
    if native:
        lib = native_library(args, out)
        mapping = {}
        adapter = args.goivy / "smt/quantifier.go"
        code = replace_once(adapter.read_text(), "${SRCDIR}/../z3vendor/native_lib/libz3.a", str(lib))
        push = "func (s *Z3Solver) Push() {"
        write_overlay(mapping, adapter, out, replace_once(code, push, push + '\n defer qiWatch("push")()\n'))
    else:
        mapping = instrument_go(args, out)
    adapter = args.goivy / "smt" / ("native_verification_condition.go" if native else "g3_solver.go")
    code = adapter.read_text()
    if native:
        code = replace_once(code, NATIVE_METHOD, NATIVE_METHOD.replace("return s.Check()", 'defer qiWatch("check")(); return s.Check()'))
        code = replace_once(code, "return s.Check, func() {}", "return s.CheckVerificationCondition, func() {}")
    else:
        code = replace_once(code, G3_METHOD, G3_METHOD + '\n defer qiWatch("check")()')
        push = "func (s *Z3Solver) Push() {"
        code = replace_once(code, push, push + '\n defer qiWatch("push")()\n')
    deps = ["runtime", "os", "strconv"] + ([] if native else ["fmt"])
    imports = [f'"{p}"' for p in deps if f'"{p}"' not in code]
    if not native:
        imports.append(f'ops "{GO_COUNTER}"')
    code = replace_once(code, "package smt\n", "package smt\nimport (\n" + "\n".join(imports) + "\n)\n")
    report = """ops.IvyPhase=0
        for p:=1;p<=8;p++ {fmt.Fprint(os.Stderr,"IVY-OPS ",p);for _,n:=range ops.IvyOps[p]{fmt.Fprint(os.Stderr," ",n)};fmt.Fprintln(os.Stderr)}"""
    write_overlay(mapping, adapter, out, code + WATCH.replace("START", "" if native else "ops.IvyPhase=8").replace("END", "" if native else report))
    overlay = out / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    checker = out / "checker"
    tags = "xtrace_off,classic_z3" if native else "xtrace_off"
    subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-tags=" + tags, "-overlay=" + str(overlay),
                    "-o", checker, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    return checker


def measure(args, engine, checker, case):
    iso, window, ordinal, cpu, ref = case
    out = args.output / f"{engine}-{iso}-{window}-{ordinal}"
    out.mkdir()
    with (out / "stdout").open("wb") as so, (out / "stderr").open("wb") as se:
        subprocess.run(["taskset", "-c", cpu, checker, "isolate="+iso, args.spec], cwd=out, stdout=so, stderr=se,
                       env=dict(os.environ, GOMAXPROCS="1", IVY_QI_TARGET=window, IVY_QI_CHECK=ordinal), check=True)
    if (out / "stdout").read_bytes() != Path(ref).read_bytes() or not (out / "stdout").read_bytes().endswith(b"OK\n"):
        raise ValueError(f"proof transcript differs in {out}")
    lines = (out / "stderr").read_text().splitlines()
    counts = [s.split()[1:] for s in lines if s.startswith("IVY-OPS ")]
    passes = [s.split()[1:] for s in lines if s.startswith("IVY-PASS ")]
    if [r[0] for r in counts] != list(map(str, range(1, 9))) or any(len(r) != 12 for r in counts):
        raise ValueError(f"missing operation counts in {out}")
    prefix = [engine, iso, window, ordinal, cpu]
    keys = ["engine", "isolate", "window", "ordinal", "cpu"]
    for filename, columns, rows in (
            ("counts", ["phase", *EVENTS], [[PHASES[int(r[0])-1], *r[1:]] for r in counts]),
            ("passes", ["pass_index", "pass", "visits", "distinct"], passes)):
        with (out / (filename + ".csv")).open("w") as f:
            writer = csv.writer(f, lineterminator="\n")
            writer.writerow(keys + columns)
            writer.writerows(prefix + r for r in rows)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for arg in ("g3", "goivy", "build", "output", "spec"):
        p.add_argument("--"+arg, type=Path, required=True)
    p.add_argument("--jobs", type=int, default=4)
    p.add_argument("--case", nargs=5, action="append", required=True, metavar=("ISOLATE","WINDOW","ORDINAL","CPU","REFERENCE"))
    args = p.parse_args()
    if args.jobs < 1 or any(w not in ("push", "check") or int(n) < 1 for _, w, n, _, _ in args.case):
        p.error("use a positive job count, push or check, and a positive ordinal")
    args.g3, args.goivy, args.build = (a.resolve(strict=True) for a in (args.g3, args.goivy, args.build))
    args.output = args.output.resolve()
    args.output.mkdir(parents=True)
    args.spec = args.spec.absolute()

    def run(engine):
        checker = compile_checker(args, engine)
        with ThreadPoolExecutor(len(args.case)) as pool:
            list(pool.map(lambda case: measure(args, engine, checker, case), args.case))
    with ThreadPoolExecutor(2) as pool:
        list(pool.map(run, ["g3", "native"]))


if __name__ == "__main__":
    main()
