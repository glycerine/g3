"""Measure matching Go and C++ methods on saved incremental SMT queries (Linux)."""
import argparse
import csv
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys

from query_replay import CASES, HERE, query_inputs

PKG = HERE.parents[2]
ROOT = PKG.parent
OPS = {
    "check-sat": ("cmd_context/context.go", ["(*Context).CheckSatDisplay"],
                  "cmd_context/cmd_context.cpp", ["cmd_context::check_sat"]),
    "make-numeral": ("ast/ast_arith_numeral.go", ["(*ASTManager).mkArithmeticNumeralCaptured"],
                     "ast/arith_decl_plugin.cpp", ["arith_decl_plugin::mk_numeral(rational const & val, bool is_int)"]),
    "read-numeral": ("ast/ast_arith_numeral_text.go", ["(*ASTManager).ArithmeticNumeralRational"],
                     "ast/arith_decl_plugin.cpp", ["arith_recognizers::is_numeral(expr const * n, rational & val, bool & is_int) const"]),
    "boolean-equality": ("ast/rewriter/bool_rewriter_eq.go", ["(*boolRewriter).mkEqCore"],
                         "ast/rewriter/bool_rewriter.cpp", ["bool_rewriter::mk_eq_core"]),
    "arithmetic-comparison": ("ast/rewriter/arith_rewriter.go", ["(*ArithRewriter).rewriteComparison"],
                              "ast/rewriter/arith_rewriter.cpp", ["arith_rewriter::mk_le_core", "arith_rewriter::mk_ge_core",
                                                                "arith_rewriter::mk_lt_core", "arith_rewriter::mk_gt_core"]),
    "boolean-and-or": ("ast/rewriter/bool_rewriter.go", ["(*boolRewriter).mkNFlatCore"],
                       "ast/rewriter/bool_rewriter.cpp", ["bool_rewriter::mk_nflat_and_core", "bool_rewriter::mk_nflat_or_core"]),
    "theory-dispatch": ("ast/rewriter/th_rewriter.go", ["(*theoryRewriterConfiguration).reduceAppCore"],
                        "ast/rewriter/th_rewriter.cpp", ["reduce_app_core"]),
    "instantiate-body": ("smtcore/cached_var_subst.go", ["(*SMTCachedVarSubstitution).Instantiate"],
                         "smt/cached_var_subst.cpp", ["cached_var_subst::operator()"]),
}
STATS = ("decisions", "quant-instantiations", "conflicts", "rlimit-count")
IMPORT = 'import "github.com/glycerine/g3/pkg/util/replaycounter"'


def inject(code, pattern, hook):
    patched, count = re.subn(pattern, lambda m: m.group() + "\n" + hook, code, flags=re.M)
    if count != 1:
        raise ValueError(f"expected a single match for {pattern}, found {count}")
    return patched


def build_go(directory, path, methods, extra):
    mapping = dict(json.loads(extra.read_text())["Replace"]) if extra else {}
    source = PKG / path
    code = Path(mapping.get(str(source), source)).read_text()
    code = inject(code, r"^package \w+$", IMPORT)
    for method in methods:
        receiver, name = method.removeprefix("(*").split(").")
        pattern = r"^func \(\w+ \*" + receiver + r"\) " + name + r"\([^\n]+ \{$"
        code = inject(code, pattern, "defer replaycounter.Leave(replaycounter.Enter())")
    patched = directory / source.name
    patched.write_text(code)
    mapping[str(source)] = str(patched)
    main = PKG / "cmd/g3/main.go"
    code = inject(main.read_text(), r"^package main$", IMPORT)
    patched_main = directory / "main.go"
    patched_main.write_text(inject(code, r"^func main\(\) \{$", "defer replaycounter.Begin()()"))
    mapping[str(main)] = str(patched_main)
    mapping[str(PKG / "util/replaycounter/count.go")] = str(HERE / "replay_counter.go.txt")
    overlay = directory / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    shell = directory / "g3"
    subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-overlay=" + str(overlay),
                    "-o", shell, "./cmd/g3"], cwd=PKG, check=True)
    return shell


def make_words(build, var, text):
    match = re.search(r"^" + re.escape(var) + r"\s*=\s*(.*?)\s*$", text, re.M)
    if not match:
        raise ValueError(f"missing {var} in {build}")
    return shlex.split(match[1])


def build_native(directory, path, methods, build):
    source = ROOT / "src" / path
    code = source.read_text()
    for method in methods:
        signature = re.escape(method)
        if "(" not in method or method.endswith("operator()"):
            signature += r"\([^\n]+\)"
        pat = r"^\s*(?:br_status |void |bool |app \*\s*)" + signature + r" \{$"
        code = inject(code, pat, "replay_measurement::interval measure;")
    patched = directory / source.name
    patched.write_text('#include "test/perf/replay_interval.h"\n' + code)
    config = (build / "config.mk").read_text()
    makefile = (build / "Makefile").read_text()
    includes = re.search(r"\$\((includes_\d+)\).*\.\./src/" + re.escape(path) + r"$", makefile, re.M)
    if includes is None:
        raise ValueError("missing C++ compile rule for " + path)
    obj = directory / (source.stem + ".o")
    subprocess.run([*make_words(build, "CXX", config), *make_words(build, "CXXFLAGS", config),
                    *make_words(build, includes[1], makefile), "-o", obj, patched], cwd=build, check=True)
    # Use the same objects, archive ordering and flags as the ordinary shell.
    # This explicit object satisfies the references normally served by its archive member.
    link = subprocess.check_output(["make", "-n", "-W", "shell/main.o", "z3"], cwd=build, text=True)
    cmds = [shlex.split(ln) for ln in link.splitlines() if "shell/main.o" in ln and " -o z3 " in ln]
    if len(cmds) != 1:
        raise ValueError("unrecognized native shell link command")
    args = cmds[0]
    shell = directory / "z3"
    args[args.index("-o")+1] = str(shell)
    args.insert(args.index("shell/main.o"), str(obj))
    subprocess.run(args, cwd=build, check=True)
    return shell


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--output", required=True, type=Path)
    p.add_argument("--build", type=Path, default=ROOT / "build")
    p.add_argument("--operation", required=True, choices=OPS)
    p.add_argument("--go-overlay", type=Path)
    p.add_argument("--pairs", type=int, default=5)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--queries", nargs="+", choices=CASES, default=CASES)
    args = p.parse_args()
    if args.pairs < 1:
        p.error("pairs must be positive")
    directory = args.output.resolve()
    directory.mkdir(parents=True)
    path, methods, cpp, native_methods = OPS[args.operation]
    shells = {"g3": build_go(directory, path, methods, args.go_overlay),
              "native": build_native(directory, cpp, native_methods, args.build.resolve())}
    wr = csv.writer(sys.stdout, lineterminator="\n")
    wr.writerow(["query", "round", "engine", "operation", "calls", "instructions", *STATS])
    for case in args.queries:
        text = query_inputs(HERE.parent / "test_vectors" / (case+".smt2.gz"))["incremental"]
        text += "\n(get-info :all-statistics)\n"
        expected = None
        for i in range(args.pairs):
            for engine in list(shells)[::1 if i%2 == 0 else -1]:
                process = subprocess.run(["taskset", "-c", str(args.cpu), shells[engine], "-in", "-smt2"],
                                         input=text, text=True, capture_output=True, timeout=120, check=True,
                                         env=dict(os.environ, GOMAXPROCS="1", SCOPE_THREAD_EVENT="1"))
                stats = dict(re.findall(r":([\w-]+)\s+(\d+)\b", process.stdout))
                sample = re.findall(r"^REPLAY-OP (\d+) (\d+)$", process.stderr, re.M)
                counters = tuple(int(stats[k]) for k in STATS)
                if (process.stdout.splitlines()[0] != "unsat" or "(error" in process.stdout or len(sample) != 1
                        or args.operation == "check-sat" and int(sample[0][0]) != 1
                        or expected is not None and counters != expected):
                    raise ValueError(f"{engine} {case} changed verdict, counters, or interval count")
                expected = counters
                wr.writerow([case, i+1, engine, args.operation, *sample[0], *counters])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
