"""Count requests for existing short AST applications within each GoIvy solver check."""

import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
PKG = HERE.parents[2]
sys.path.insert(0, str(HERE.parent))
from phase_counters import build, replace_once

OBSERVER = '''
var appCheck int
func beginMeasuredCheck() func() {
    appCheck++
    n := appCheck
    ast.StudyAppBegin()
    return func() {
        for arity, counters := range ast.StudyAppEnd() {
            fmt.Fprintln(os.Stderr, "APP-COUNTERS", n, arity, counters[0], counters[1], counters[2], counters[3], counters[4])
        }
    }
}
'''
ATTEMPT = '''
    if studyAppActive {
        arity := min(args.count, 4)
        studyAppValues[arity][0]++
        if !ASTSourceDebug && !astValidateHandles && frame == nil && !m.BudgetField.InjectionArmed() &&
            args.count <= astInlineArgs && args.appOwner == nil && !args.raw &&
            uint64(args.argumentOffset) + uint64(args.count) <= uint64(len(args.slice)) {
            studyAppValues[arity][1]++
            old, first := m.countLookupApp(decl, args.slice[args.argumentOffset:args.argumentOffset+args.count])
            if old.slot != 0 {
                studyAppValues[arity][2]++
                if first { studyAppValues[arity][3]++ }
            }
        }
    }
'''
DUPLICATE = '''
    if studyAppActive && e == nil && r != original {
        studyAppValues[min(args.count,4)][4]++
    }
'''


def make_overlay(args, directory):
    name = "ast/ast_core_factories.go"
    source = (PKG / name).read_text()
    # Observe every request, including a canonical hit that avoids placement.
    anchor = "// known is supplied for exact-arity calls"
    source = replace_once(source, anchor, ATTEMPT + anchor)
    anchor = "if existing := m.hintedApp(decl, input, hint); existing.slot != 0 {"
    source = replace_once(source, anchor, anchor + """
            if studyAppActive { studyAppValues[min(args.count, 4)][4]++ }""")
    anchor = "if result, handled, err := m.registerSliceApp(r, decl, d, associative, input); handled {"
    source = replace_once(source, anchor, anchor + """
            if studyAppActive && err == nil && result != r {
                studyAppValues[min(args.count, 4)][4]++
            }""")
    anchor = "if e == nil && r == original {\n\t\tif frame != nil"
    source = replace_once(source, anchor, DUPLICATE + anchor)
    changed = directory / "factories.go"
    changed.write_text(source)
    cmd = ["go", "list", "-mod=readonly"]
    if args.modfile:
        cmd.append("-modfile=" + str(args.modfile.resolve(strict=True)))
    cmd += ["-m", "-json", "github.com/glycerine/g3/pkg"]
    module = Path(json.loads(subprocess.check_output(cmd, cwd=args.goivy))["Dir"])
    if module.resolve() != PKG:
        raise ValueError("the GoIvy module must use this g3 checkout")
    paths = {}
    for root in {module, PKG}:
        paths[str(root / name)] = str(changed)
        paths[str(root / "ast/study_app_counts.go")] = str(HERE / "app_counts.go.txt")
    overlay = directory / "ast.json"
    overlay.write_text(json.dumps({"Replace": paths}))
    return overlay


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", type=Path, required=True)
    p.add_argument("--modfile", type=Path)
    p.add_argument("--spec", type=Path, required=True)
    p.add_argument("--isolate", nargs=2, action="append", required=True, metavar=("NAME", "REFERENCE_STDOUT"))
    p.add_argument("--cpu", type=int, required=True)
    args = p.parse_args()
    args.goivy = args.goivy.resolve(strict=True)
    spec = args.spec.absolute()
    with tempfile.TemporaryDirectory() as scratch:
        directory = Path(scratch)
        binary = build("g3", args, directory, OBSERVER, ("fmt", "os", "github.com/glycerine/g3/pkg/ast"),
                       go_overlay=make_overlay(args, directory))
        writer = csv.writer(sys.stdout, lineterminator="\n")
        writer.writerow(["isolate", "check", "arity", "attempts", "eligible", "matched",
                         "first_probe_matches", "registered_duplicates"])
        for name, path in args.isolate:
            reference = Path(path).read_bytes()
            checks = reference.splitlines().count(b"PASS")
            if not re.fullmatch(r"[A-Za-z0-9_-]+", name) or not checks or reference.splitlines()[-1:] != [b"OK"]:
                p.error("supply an isolate name and its complete successful reference transcript")
            out = subprocess.run(["taskset", "-c", str(args.cpu), binary, "isolate=" + name, spec],
                                 cwd=directory, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"),
                                 capture_output=True, check=True)
            if out.stdout != reference:
                raise ValueError(f"{name}: complete output differs from reference")
            rows = [list(map(int, line.split()[1:])) for line in out.stderr.splitlines()
                    if line.startswith(b"APP-COUNTERS ")]
            if [r[:2] for r in rows] != [[i, a] for i in range(1, checks+1) for a in range(5)]:
                raise ValueError(f"{name}: incomplete check or arity coverage")
            writer.writerows([name, *row] for row in rows)


if __name__ == "__main__":
    main()
