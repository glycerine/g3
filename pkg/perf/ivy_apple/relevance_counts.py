#!/usr/bin/env python3
"""Count relevance operations in complete, incremental GoIvy verification runs."""

import argparse
import csv
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import sys
import tempfile

from search_counters import GO_METHOD, ROOT


DATA = Path(__file__).resolve().parent / "incremental_cost"
NATIVE = ROOT / "src/test/perf"
COUNTERS = ("mark_calls set_marks propagate_calls propagated_nodes assignment_events "
            "handler_visits positive_visits negative_visits or_calls and_calls all_calls "
            "all_children or_test and_test or_witness and_witness or_select and_select "
            "or_handlers and_handlers").split()
PATHS = ["no variable", "no gate", "no literals", "bound gate"]
PATH_COUNTERS = ["calls", "child_tests", "witness", "selection", "mark_all"]


def patched(source, patch, dest):
    subprocess.run(["patch", "--batch", "--fuzz=0", "--quiet", "-o", dest, source, patch],
                   stdout=sys.stderr, check=True)
    return dest


def go_build(args, flags, dest):
    cmd = ["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false"]
    if args.modfile:
        cmd.append(f"-modfile={args.modfile}")
    subprocess.run([*cmd, *flags, "-o", dest, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    return dest


def build_go(args, tmp):
    cmd = ["go", "list", "-mod=readonly"]
    if args.modfile:
        cmd.append(f"-modfile={args.modfile}")
    module = Path(subprocess.check_output(
        [*cmd, "-m", "-f", "{{.Dir}}", "github.com/glycerine/g3/pkg"], cwd=args.goivy, text=True).strip())
    if not module.samefile(ROOT / "pkg"):
        raise ValueError(f"GoIvy uses {module}. Select this checkout in its module replacement.")
    source = module / "smtcore/smt_relevancy.go"
    patch = DATA / ("root-gates.patch" if args.root_gates else "go-relevance.patch")
    s = patched(source, patch, tmp / source.name)
    adapter = args.goivy / "smt/g3_solver.go"
    code = adapter.read_text()
    if code.count(GO_METHOD) != 1:
        raise ValueError(f"unrecognized GoIvy adapter {adapter}")
    if '"github.com/glycerine/g3/pkg/smtcore"' not in code:
        code = code.replace("import (", 'import (\n"github.com/glycerine/g3/pkg/smtcore"', 1)
    g = tmp / adapter.name
    g.write_text(code.replace(GO_METHOD, GO_METHOD + "\n defer smtcore.ReportRelevanceCounts()"))
    replacements = {str(k): str(v) for f, v in [(source, s), (adapter, g)] for k in [f, f.resolve()]}
    overlay = tmp / "overlay.json"
    overlay.write_text(json.dumps({"Replace": replacements}))
    return go_build(args, ["-tags=xtrace_off", f"-overlay={overlay}"], tmp / "g3")


def make_value(text, name):
    return shlex.split(re.search(r"^" + name + r"\s*=\s*(.*)$", text, re.MULTILINE)[1])


def build_native(args, tmp):
    # Verify the observed translation unit is the one in the vendored library.
    library = args.goivy / "z3vendor/native_lib/libz3.a"
    obj = "smt_relevancy.o"
    if subprocess.check_output(["ar", "p", library, obj]) != (args.native_build / "smt" / obj).read_bytes():
        raise ValueError("the vendored relevance object differs from the reference build")
    source = ROOT / "src/smt/smt_relevancy.cpp"
    cpp = patched(source, NATIVE / "ivy_relevance_counts.patch", tmp / source.name)
    config = (args.native_build / "config.mk").read_text()
    makefile = (args.native_build / "Makefile").read_text()
    includes = re.search(r"\$\((includes_\d+)\).*\.\./src/smt/smt_relevancy\.cpp", makefile)[1]
    compiler = make_value(config, "CXX")
    flags = make_value(config, "CXXFLAGS")
    subprocess.run([*compiler, *flags, *make_value(makefile, includes), "-o", tmp / obj, cpp],
                   cwd=args.native_build, check=True)
    archive = tmp / "libz3.a"
    shutil.copyfile(library, archive)
    subprocess.run(["ar", "rcs", archive, tmp / obj], check=True)
    wrapper = tmp / "check.o"
    subprocess.run([*compiler, *flags, "-I", ROOT / "src/api", "-o", wrapper,
                    NATIVE / "ivy_relevance_counts.cpp"], cwd=args.native_build, check=True)

    # Replace the archive at the position where GoIvy already links it.
    external = shlex.split(subprocess.check_output(["go", "env", "CC"], text=True).strip())
    linker = tmp / "linker"
    used = tmp / "linked"
    linker.write_text(f"""#!/usr/bin/env python3
import os, sys
from pathlib import Path
args = {external!r}
for arg in sys.argv[1:]:
    if arg.endswith("/z3vendor/native_lib/libz3.a"):
        args.extend({["-Wl,--wrap=Z3_solver_check", str(wrapper), str(archive)]!r})
        Path({str(used)!r}).touch()
    else:
        args.append(arg)
os.execvp(args[0], args)
""")
    linker.chmod(0o700)
    binary = go_build(args, ["-tags=xtrace_off,classic_z3", f"-ldflags=-extld={linker}"], tmp / "native")
    if not used.is_file():
        raise ValueError("GoIvy did not link the observed library")
    return binary


def run(args, binary, reference, writer, cwd):
    got = subprocess.run(["taskset", "-c", str(args.cpu), binary, f"isolate={args.isolate}", args.spec],
                         cwd=cwd, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"),
                         capture_output=True, check=True)
    if got.stdout != reference:
        raise ValueError(f"{binary.name} transcript differs from the reference")
    marker = b"ROOT-CENSUS " if args.root_gates else b"REL-OPS "
    rows = [list(map(int, line.split()[1:])) for line in got.stderr.splitlines() if line.startswith(marker)]
    width = len(PATHS) * len(PATH_COUNTERS) if args.root_gates else len(COUNTERS)
    if len(rows) != reference.splitlines().count(b"PASS") or any(len(r) != width for r in rows):
        raise ValueError("incomplete relevance counters")
    previous = [0] * width
    for seq, row in enumerate(rows, 1):
        delta = [a - b for a, b in zip(row, previous)]
        if args.root_gates:
            for i, path in enumerate(PATHS):
                start = i * len(PATH_COUNTERS)
                writer.writerow([args.isolate, seq, path, *delta[start:start + len(PATH_COUNTERS)]])
        else:
            writer.writerow([args.isolate, binary.name, seq, *delta])
        previous = row


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", type=Path, required=True)
    p.add_argument("--spec", type=Path, required=True)
    p.add_argument("--reference", type=Path, required=True, help="known-good full stdout for this isolate")
    p.add_argument("--isolate", default="rfn2")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--modfile", type=Path)
    p.add_argument("--native-build", type=Path, default=ROOT / "build", help="configured Makefile build")
    p.add_argument("--root-gates", action="store_true", help="measure the Go disjunction paths")
    args = p.parse_args()
    args.goivy = args.goivy.absolute()
    args.native_build = args.native_build.resolve()
    args.spec = args.spec.absolute()
    if args.modfile:
        args.modfile = args.modfile.resolve(strict=True)
    reference = args.reference.read_bytes()
    if reference.splitlines()[-1:] != [b"OK"] or b"PASS" not in reference.splitlines():
        p.error("the reference must contain successful obligations and finish with OK")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "check", "path", *PATH_COUNTERS] if args.root_gates else
                    ["isolate", "engine", "check", *COUNTERS])
    with tempfile.TemporaryDirectory() as scratch:
        tmp = Path(scratch)
        run(args, build_go(args, tmp), reference, writer, tmp)
        if not args.root_gates:
            run(args, build_native(args, tmp), reference, writer, tmp)


if __name__ == "__main__":
    main()
