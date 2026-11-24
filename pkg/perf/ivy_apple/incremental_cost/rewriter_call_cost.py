"""Measure sampled instructions per rewriter operation on matched first pushes (Linux)."""

import argparse
from bisect import bisect_left
import csv
import gzip
import os
from pathlib import Path
import re
import subprocess

from first_push_rewrite_rows import G3, HERE, build_checker, make_value, native_library, stage_patch

OPS = {
    "visit": ("ast/rewriter/rewriter_engine.go", "func (r *rewriterEngine) visit(t "),
    "app": ("ast/rewriter/rewriter_engine.go", "func (r *rewriterEngine) processApp()"),
    "constant": ("ast/rewriter/rewriter_engine.go", "func (r *rewriterEngine) processConst(t "),
    "variable": ("ast/rewriter/rewriter_engine.go", "func (r *rewriterEngine) processVar(t "),
    "fold": ("ast/rewriter/rewriter_engine.go", "func (r *rewriterEngine) constantFold(frameIndex "),
    "substitution": ("ast/rewriter/th_rewriter.go", "func (c *theoryRewriterConfiguration) GetSubst("),
    "reduce": ("ast/rewriter/th_rewriter.go", "func (c *theoryRewriterConfiguration) ReduceApp("),
    "reduce-core": ("ast/rewriter/th_rewriter.go", "func (c *theoryRewriterConfiguration) reduceAppCore("),
    "find": ("ast/ast_activity_cache.go", "func (c *ActivityCache) Find("),
    "insert": ("ast/ast_activity_cache.go", "func (c *ActivityCache) Insert("),
}
HEADER = '#include "test/perf/ivy_rewriter_call_cost.h"\n'
OFFSETS = [0, 17, 34, 51, 62]

TEMPLATE_METHODS = {
    "visit": "bool rewriter_tpl<Config>::visit(expr * t, unsigned max_depth) {",
    "app": "void rewriter_tpl<Config>::process_app(app * t, frame & fr) {",
    "constant": "void rewriter_tpl<Config>::process_const(app * t) {",
    "variable": "void rewriter_tpl<Config>::process_var(var * v) {",
    "fold": "bool rewriter_tpl<Config>::constant_fold(app * t, frame & fr) {",
}

THEORY_METHODS = {
    "substitution": "bool get_subst(expr * s, expr * & t, proof * & pr) {",
    "reduce": "br_status reduce_app(func_decl * f, unsigned num, expr * const * args, expr_ref & result, proof_ref & result_pr) {",
    "reduce-core": "br_status reduce_app_core(func_decl * f, unsigned num, expr * const * args, expr_ref & result) {",
}


def replace_one(text, old, new):
    if text.count(old) != 1:
        raise ValueError(f"expected one source anchor {old!r}")
    return text.replace(old, new)


def edit(files, stage, path, source):
    target = stage / path.name
    target.write_text(source)
    files[path] = target
    return target


def prepare_go(args, stage, files):
    pkg = G3 / "pkg"
    meter = pkg / "util/visitmeter/meter.go"
    edit(files, stage, meter, (HERE / "rewriter_call_meter.go.txt").read_text())
    imp = '\nimport "github.com/glycerine/g3/pkg/util/visitmeter"\n'
    driver = pkg / "smtcore/smt_asserted_driver.go"
    s = replace_one(driver.read_text(), "package smtcore", "package smtcore" + imp)
    mark = "if err := assertedVerbose(10,"
    if mark not in s:
        raise ValueError("missing asserted pass marker")
    edit(files, stage, driver, s.replace(mark, "defer visitmeter.Begin(pass.sourceName())()\n" + mark, 1))
    name, method = OPS[args.operation]
    path = pkg / name
    s = path.read_text()
    start = s.index("{", s.index(method)) + 1
    s = s[:start] + "\n defer visitmeter.Leave(visitmeter.Enter())\n" + s[start:]
    edit(files, stage, path, replace_one(s, "import (", imp + "import ("))
    return build_checker(args.goivy, stage, files, args.abc)


def instrument_cache(stage, src, operation):
    # Keep the exact act_cache.o embedded in GoIvy. Wrap the linked operation
    # so a separately compiled cache implementation cannot skew this comparison.
    find = operation == "find"
    sym = "_ZN9act_cache4findEP4expr" if find else "_ZN9act_cache6insertEP4exprS1_"
    result = "void*" if find else "void"
    args = "void* cache, void* key" + ("" if find else ", void* value")
    call = "cache, key" + ("" if find else ", value")
    wrap = stage / "wrapper.cpp"
    wrap.write_text(HEADER + f'extern "C" {result} __real_{sym}({args});\n'
                    f'extern "C" {result} __wrap_{sym}({args}) {{\n'
                    f'    ivy_rewriter_cost::sample measured;\n    return __real_{sym}({call});\n}}\n')
    obj = stage / "wrapper.o"
    subprocess.run(["c++", "-std=c++17", "-O3", "-fPIC", "-I", src, "-c", wrap, "-o", obj], check=True)
    return ["-ldflags", f"-linkmode=external '-extldflags=-Wl,--wrap={sym} {obj} {stage}/libz3.a -lstdc++ -lm -lgomp'"]


def prepare_native(args, stage, files):
    src = G3 / "src"
    driver = src / "smt/asserted_formulas.cpp"
    hook = "if (!s.should_apply()) return true;"
    s = HEADER + "thread_local ivy_rewriter_cost::counts* ivy_rewriter_cost::active = nullptr;\n"
    s += replace_one(driver.read_text(), hook, hook + "\nivy_rewriter_cost::pass measured(s.id());")
    theory = src / "ast/rewriter/th_rewriter.cpp"
    patched = None
    sample = "\nivy_rewriter_cost::sample measured;"
    if args.operation in TEMPLATE_METHODS:
        inc = 'ast/rewriter/rewriter_def.h'
        h = src / inc
        method = TEMPLATE_METHODS[args.operation]
        h = edit(files, stage, h, HEADER + replace_one(h.read_text(), method, method + sample))
        include = f'#include "{inc}"'
        private = f'#include "{h}"'
        s = replace_one(s, include, private)
        patched = edit(files, stage, theory, replace_one(theory.read_text(), include, private))
    elif args.operation in THEORY_METHODS:
        method = THEORY_METHODS[args.operation]
        patched = edit(files, stage, theory, HEADER + replace_one(theory.read_text(), method, method + sample))
    edit(files, stage, driver, s)
    native_library(args, stage, files, counter=None)
    if patched is not None:
        lib = args.goivy / "z3vendor/native_lib/libz3.a"
        build = args.cpp_build
        member = "th_rewriter.o"
        if subprocess.check_output(["ar", "p", lib, member]) != (build / "ast/rewriter" / member).read_bytes():
            raise ValueError(f"GoIvy and the in-tree C++ build use different {member} objects")
        makefile = (build / "Makefile").read_text()
        config = (build / "config.mk").read_text()
        rule = re.search(r"\$\((includes_\d+)\).*\.\./src/ast/rewriter/th_rewriter\.cpp$", makefile, re.M)
        if rule is None:
            raise ValueError("missing th_rewriter.cpp build rule")
        obj = stage / member
        subprocess.run([*make_value(config, "CXX"), *make_value(config, "CXXFLAGS"), *make_value(makefile, rule[1]),
                        "-o", obj, patched], cwd=build, check=True)
        subprocess.run(["ar", "rcs", stage / "libz3.a", obj], check=True)
        flags = []
    else:
        flags = instrument_cache(stage, src, args.operation)
    return build_checker(args.goivy, stage, files, args.abc, True, flags)


def measure(args, binary, directory, offset):
    directory.mkdir()
    cmd = ["taskset", "-c", str(args.cpu), binary]
    if args.isolate:
        cmd.append("isolate=" + args.isolate)
    p = subprocess.run([*cmd, args.spec], cwd=directory, capture_output=True,
                       env=dict(os.environ, GOMAXPROCS="1", GOGC="100", XTRACE_OFF="1", VISIT_OFFSET=str(offset)))
    spans, events, rest = [], [], bytearray()
    for line in p.stderr.splitlines(keepends=True):
        if line.startswith(b"W-INTERVAL "):
            _, batch, kind, _, tid, start, end = line.split()
            if kind == b"first":
                spans.append(tuple(map(int, (batch, tid, start, end))))
        elif line.startswith(b"VISIT-COST "):
            _, name, *values = line.decode().split()
            events.append((name, *map(int, values)))
        else:
            rest.extend(line)
    if (p.returncode != args.expect_exit or p.stdout != (args.reference / "stdout").read_bytes() or
            rest != (args.reference / "stderr").read_bytes()):
        (directory / "stdout").write_bytes(p.stdout)
        (directory / "stderr").write_bytes(p.stderr)
        raise RuntimeError(f"verification output differed in {directory}")
    ends = [s[3] for s in spans]
    if any(s[2] > s[3] or i > 0 and s[2] < ends[i-1] for i, s in enumerate(spans)):
        raise ValueError("out-of-order or overlapping first-push intervals")
    samples = 0
    with (directory / "calls.csv").open("w", newline="") as f:
        out = csv.writer(f)
        out.writerow(["push", "pass", "calls", "samples", "raw", "empty"])
        for name, tid, start, end, calls, n, instructions, empty in events:
            i = bisect_left(ends, start)
            if i < len(spans) and tid == spans[i][1] and spans[i][2] <= start <= end <= ends[i]:
                out.writerow([spans[i][0], name, calls, n, instructions, empty])
                samples += n
    if not samples:
        raise RuntimeError(f"no {args.operation} calls sampled inside the first pushes")
    return len(spans), samples


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for key in ("goivy", "spec", "reference", "output"):
        p.add_argument("--" + key, type=Path, required=True)
    p.add_argument("--operation", choices=OPS, required=True)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=len(OFFSETS))
    p.add_argument("--cpp-build", type=Path, default=G3 / "build")
    p.add_argument("--abc", type=Path)
    p.add_argument("--expect-exit", type=int, default=0)
    p.add_argument("--isolate", default="")
    args = p.parse_args()
    if args.rounds < 1:
        p.error("--rounds must be positive")
    args.spec = args.spec.absolute()
    for key in ("goivy", "reference", "cpp_build", "output"):
        setattr(args, key, getattr(args, key).resolve())
    args.output.mkdir(parents=True)
    subprocess.run(["go", "work", "init", G3 / "pkg", args.goivy], cwd=args.output,
                   env=dict(os.environ, GOWORK="off"), check=True)
    os.environ["GOWORK"] = str(args.output / "go.work")
    # Use the same independently bracketed frontend Push windows as the pass census.
    patch = gzip.decompress((HERE / "nov24-first-pass-observers.patch.gz").read_bytes()).decode()
    frontend = "".join(h for h in re.split(r"(?=^--- )", patch, flags=re.M) if "\n+++ b/ivy/" in h)
    bins = {}
    for engine, build in [("g3", prepare_go), ("native", prepare_native)]:
        stage = args.output / ("build-" + engine)
        stage.mkdir()
        bins[engine] = build(args, stage, stage_patch(frontend, args.goivy, stage))
        (stage / "build-info.txt").write_bytes(subprocess.check_output(["go", "version", "-m", bins[engine]]))
    with (args.output / "runs.csv").open("w", newline="") as f:
        out = csv.writer(f)
        out.writerow(["round", "operation", "offset", "engine", "pushes", "samples"])
        for i in range(args.rounds):
            offset = OFFSETS[i % len(OFFSETS)]
            for engine in list(bins)[::1 if i % 2 == 0 else -1]:
                pushes, samples = measure(args, bins[engine], args.output / f"{i+1}-{engine}", offset)
                out.writerow([i+1, args.operation, offset, engine, pushes, samples])
                f.flush()


if __name__ == "__main__":
    main()
