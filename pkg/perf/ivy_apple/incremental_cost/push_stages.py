"""Measure corresponding preprocessing stages inside external GoIvy Push calls (Linux)."""
import argparse
import csv
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

HERE = Path(__file__).resolve().parent
PKG = HERE.parents[2]
ROOT = PKG.parent
sys.path.insert(0, str(HERE.parent))
from phase_counters import replace_once
from relevance_counts import make_value

STAGES = {
    "reduce-assertions": ("smtcore/smt_asserted_driver.go", "reduceAssertedPending"),
    "expand": ("ast/macros/macro_expander.go", "ExpandMacros"),
    "instantiate-body": ("smtcore/cached_var_subst.go", "Instantiate"),
    "passes": ("smtcore/smt_asserted_driver.go", "invokeAssertedPass"),
}
IMPORT = 'import "github.com/glycerine/g3/pkg/util/pushcounter"'


def build_go(args, directory):
    module = json.loads(subprocess.check_output(
        ["go", "list", "-mod=readonly", "-m", "-json", "github.com/glycerine/g3/pkg"], cwd=args.goivy))
    local = Path(module["Dir"])
    if not local.samefile(PKG):
        raise ValueError(f"GoIvy uses {local}, expected {PKG}")
    mapping = dict(json.loads(args.go_overlay.read_text())["Replace"]) if args.go_overlay else {}
    name, method = STAGES[args.stage]
    src = PKG / name
    code = Path(mapping.get(str(src), src)).read_text()
    code = re.sub(r"^package \w+$", lambda m: m.group()+"\n"+IMPORT, code, count=1, flags=re.M)
    if args.stage == "passes":
        marker = 'if err := assertedVerbose(10, "(smt."+pass.sourceName()'
        code = replace_once(code, marker, "defer pushcounter.Pass(pass.sourceName(),len(k.assertions)-k.assertedHead)()\n"+marker)
    else:
        lines = [ln for ln in code.splitlines() if ln.startswith("func ") and ") "+method+"(" in ln]
        if len(lines) != 1:
            raise ValueError(f"unrecognized {method}")
        code = replace_once(code, lines[0], lines[0]+"\n defer pushcounter.Leave(pushcounter.Enter())")
    patched = directory / src.name
    patched.write_text(code)
    for pkg in (local, PKG):
        mapping[str(pkg / name)] = str(patched)
        mapping[str(pkg / "util/pushcounter/counter.go")] = str(HERE / "push_stage_counter.go.txt")
    adapter = args.goivy / "smt/g3_solver.go"
    code = Path(mapping.get(str(adapter), adapter)).read_text()
    code = replace_once(code, "package smt", "package smt\n"+IMPORT)
    code = replace_once(code, "Push() { fail(s.c.Push()) }", "Push() {defer pushcounter.Scope()(); fail(s.c.Push())}")
    patch_adapter = directory / adapter.name
    patch_adapter.write_text(code)
    mapping[str(adapter)] = str(patch_adapter)
    overlay = directory / "g3.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    shell = directory / "g3"
    subprocess.run(["go", "build", "-mod=readonly", "-buildvcs=false", "-pgo=off", "-tags=xtrace_off",
                    "-overlay="+str(overlay), "-o", shell, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    return shell


def asserted_archive(args, directory, library):
    member = "asserted_formulas.o"
    if subprocess.check_output(["ar", "p", library, member]) != (args.build / "smt" / member).read_bytes():
        raise ValueError("the fork's asserted_formulas.o differs from the GoIvy archive")
    source = ROOT / "src/smt/asserted_formulas.cpp"
    code = 'extern "C" unsigned long long ivy_asserted_pass_start();\n'
    code += 'extern "C" void ivy_asserted_pass_end(unsigned long long, char const*, unsigned);\n'
    code += replace_once(source.read_text(), "    s();\n    IF_VERBOSE(10000,", """
    unsigned inputs = m_formulas.size() - m_qhead;
    auto start = ivy_asserted_pass_start();
    s();
    ivy_asserted_pass_end(start, s.id(), inputs);
    IF_VERBOSE(10000,""")
    patched = directory / source.name
    patched.write_text(code)
    makefile, config = [(args.build / name).read_text() for name in ("Makefile", "config.mk")]
    rule = re.search(r"\$\((includes_\d+)\).*\.\./src/smt/asserted_formulas\.cpp$", makefile, re.M)
    obj = directory / member
    subprocess.run([*make_value(config, "CXX"), *make_value(config, "CXXFLAGS"), *make_value(makefile, rule[1]),
                    "-o", obj, patched], cwd=args.build, check=True)
    archive = directory / "libz3.a"
    shutil.copyfile(library, archive)
    subprocess.run(["ar", "rcs", archive, obj], check=True)
    adapter = args.goivy / "smt/quantifier.go"
    patched = directory / adapter.name
    patched.write_text(replace_once(adapter.read_text(), "${SRCDIR}/../z3vendor/native_lib/libz3.a", str(archive)))
    overlay = directory / "native.json"
    overlay.write_text(json.dumps({"Replace": {str(adapter): str(patched)}}))
    return archive, ["-overlay="+str(overlay)]


def build_native(args, directory):
    source = ROOT / "src/test/perf/ivy_push_stages.cpp"
    obj = directory / "stages.o"
    subprocess.run(["c++", "-std=c++17", "-O2", "-fPIC", "-D_MP_INTERNAL", "-I", ROOT / "src",
                    "-I", ROOT / "src/api", "-c", source, "-o", obj], check=True)
    library = args.goivy / "z3vendor/native_lib/libz3.a"
    flags = []
    if args.stage == "passes":
        library, flags = asserted_archive(args, directory, library)
    methods = re.findall(r"^#define \w+ (_ZN\S+)$", source.read_text(), re.M)
    if len(methods) != 4:
        raise ValueError("unrecognized native wrapper symbols")
    link = " ".join(["-Wl,--wrap="+k for k in ["Z3_solver_push", *methods]] +
                    [str(obj), str(library), "-lstdc++", "-lm", "-lgomp"])
    shell = directory / "native"
    subprocess.run(["go", "build", "-mod=readonly", "-buildvcs=false", "-pgo=off", "-tags=xtrace_off,classic_z3",
                    *flags, "-ldflags", f"-linkmode=external '-extldflags={link}'", "-o", shell, "./cmd/goivy_check"],
                   cwd=args.goivy, check=True)
    return shell


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("goivy", "spec", "output", "reference"):
        p.add_argument("--"+name, type=Path, required=True)
    p.add_argument("--stage", choices=STAGES, required=True)
    p.add_argument("--isolate", required=True)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--pairs", type=int, default=3)
    p.add_argument("--build", type=Path, default=ROOT / "build")
    p.add_argument("--go-overlay", type=Path)
    args = p.parse_args()
    if args.pairs < 1:
        p.error("pairs must be positive")
    args.goivy, args.build = args.goivy.resolve(strict=True), args.build.resolve(strict=True)
    reference = args.reference.read_bytes()
    if not reference.endswith(b"OK\n"):
        raise ValueError("supply a successful reference transcript")
    dest = args.output.resolve()
    dest.mkdir(parents=True)
    shells = {"g3": build_go(args, dest), "native": build_native(args, dest)}
    wr = csv.writer(sys.stdout, lineterminator="\n")
    wr.writerow(["round", "isolate", "engine", "push", "stage", "calls", "inputs", "instructions"])
    for i in range(args.pairs):
        observed = None
        for engine in list(shells)[::1 if i%2==0 else -1]:
            proc = subprocess.run(["taskset", "-c", str(args.cpu), shells[engine],
                                   "isolate="+args.isolate, args.spec.absolute()], cwd=dest, capture_output=True,
                                  env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1", IVY_PUSH_STAGE=args.stage,
                                           SCOPE_THREAD_EVENT="1"), check=True)
            if proc.stdout != reference:
                (dest / f"{engine}-mismatch.stdout").write_bytes(proc.stdout)
                raise ValueError(f"{engine} changed the complete proof transcript")
            lines = [ln.split() for ln in proc.stderr.decode().splitlines() if ln.startswith("IVY-PUSH-")]
            pushes = [int(x[1]) for x in lines if x[0]=="IVY-PUSH-STAGE"]
            if not pushes or pushes != list(range(1,len(pushes)+1)) or observed is not None and pushes!=observed:
                raise ValueError("incomplete push sequence")
            observed = pushes
            for kind, push, stage, n, instructions in lines:
                if args.stage == "passes" and kind == "IVY-PUSH-PASS":
                    wr.writerow([i+1,args.isolate,engine,push,stage,1,n,instructions])
                elif args.stage != "passes" and kind == "IVY-PUSH-STAGE":
                    wr.writerow([i+1,args.isolate,engine,push,stage,n,"",instructions])
            sys.stdout.flush()


if __name__ == "__main__":
    main()
