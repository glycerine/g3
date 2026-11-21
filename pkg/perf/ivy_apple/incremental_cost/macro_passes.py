"""Count repeated unchanged macro traversals with the original rewrite running on every pass."""
import argparse
from collections import Counter
import csv
import json
import os
from pathlib import Path
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from phase_counters import replace_once
from thread_checks import PACKAGES


OBSERVE = """
        held, observeErr := m.manager.RefGet(oldTerm)
        if observeErr != nil { return resultDependency, observeErr }
        wasSeen := m.observedStable[held]
        index := held.Slot() % uint32(len(m.observedSlots))
        inSlot := m.observedSlots[index] == held
"""
RECORD = """
        if e == nil {
            stable := done && cfg.observedHits == 0
            fmt.Fprintf(os.Stderr, "IVY-MACRO %t %t %t %d %d\\n",
                stable, wasSeen, inSlot, cfg.observedLookups, cfg.observedHits)
            if stable {
                if m.observedStable == nil { m.observedStable = make(map[ast.ASTHandle]bool) }
                m.observedStable[held] = true
                m.observedSlots[index] = held
            }
        }
"""


def build(args, out):
    flags = ["-modfile=" + str(args.modfile.resolve(strict=True))] if args.modfile else []
    module_path = "github.com/glycerine/g3/pkg/ast/macros"
    location = subprocess.check_output(["go", "list", "-mod=readonly", *flags, "-f", "{{.Dir}}", module_path],
                                       cwd=args.goivy, text=True).strip()
    src = Path(location)
    if src.resolve() != args.g3.resolve() / "ast/macros":
        raise ValueError(f"GoIvy resolves {module_path} to a different checkout: {src}")
    manager = src / "macro_manager.go"
    expander = src / "macro_expander.go"
    code = replace_once(manager.read_text(), "type Manager struct {", """type Manager struct {
    observedStable map[ast.ASTHandle]bool
    observedSlots [128]ast.ASTHandle""")
    for point in ("tail := m.entries[limit:]", "m.byDeclaration[declaration] = len(m.entries)", "m.paramsContext, m.globals ="):
        code = replace_once(code, point, "clear(m.observedStable); clear(m.observedSlots[:])\n" + point)
    imports = PACKAGES if args.instructions else ("os", "fmt")
    source = replace_once(expander.read_text(), "import (", "import (\n" + "\n".join(f'"{name}"' for name in imports))
    source = replace_once(source, "type macroExpanderConfig struct {", "type macroExpanderConfig struct {\n observedLookups, observedHits int")
    point = "kind, err := m.GetKind(term)"
    source = replace_once(source, point, "c.observedLookups++\n" + point)
    point = "// get_subst's var_subst"
    source = replace_once(source, point, "c.observedHits++\n" + point)
    point = "done, e := m.expandMacrosStep("
    begin = "\n stopMacroCounter := beginMeasuredCheck()\n" if args.instructions else ""
    source = replace_once(source, point, OBSERVE + begin + point)
    point = "if e = errors.Join(e, cfg.close(), traversal.Close());"
    end = "\n stopMacroCounter()\n" if args.instructions else ""
    source = replace_once(source, point, end + RECORD + point)
    if args.instructions:
        source += Path(__file__).with_name("thread_counter.go.txt").read_text()
    if args.screen:
        code = replace_once(code, "observedStable map", "observedNegative *[1024]ast.ASTHandle\n observedStable map")
        code = code.replace("clear(m.observedStable)", "if m.observedNegative != nil { clear(m.observedNegative[:]) }; clear(m.observedStable)")
        source = replace_once(source, "stopMacroCounter := beginMeasuredCheck()", """
            stopScreen := beginMeasuredCheck()
            hasHead, err := m.observedContainsMacro(held)
            stopScreen()
            if err != nil {return resultDependency, err}
            fmt.Fprintln(os.Stderr, "IVY-SCREEN", !hasHead)
            stopMacroCounter := beginMeasuredCheck()""")
        source += Path(__file__).with_name("macro_screen.go.txt").read_text()
    mapping = {}
    for f, text in [(manager, code), (expander, source)]:
        target = out / f.name
        target.write_text(text)
        for name in (f, f.resolve()):
            mapping[str(name)] = str(target)
    overlay = out / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    binary = out / "checker"
    subprocess.run(["go", "build", "-pgo=off", "-buildvcs=false", "-mod=readonly", "-tags=xtrace_off",
                    *flags, "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    return binary


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--g3", type=Path, required=True, help="g3 pkg module directory")
    for key in ("goivy", "spec", "reference", "output"):
        p.add_argument("--" + key, type=Path, required=True)
    p.add_argument("--modfile", type=Path)
    p.add_argument("--screen", action="store_true", help="also measure a solver-local negative subtree lookup before each real rewrite")
    p.add_argument("--instructions", action="store_true", help="count Linux user-mode retired instructions per expansion pass")
    p.add_argument("--isolate", required=True)
    p.add_argument("--cpu", type=int, required=True)
    args = p.parse_args()
    args.instructions = args.instructions or args.screen
    out = args.output.resolve()
    out.mkdir(parents=True)
    binary = build(args, out)
    measure(args, out, binary)


def measure(args, out, binary):
    errfile = out / "stderr"
    with errfile.open("wb") as err:
        result = subprocess.check_output(["taskset", "-c", str(args.cpu), binary, "isolate=" + args.isolate, args.spec.absolute()],
                                         cwd=out, stderr=err, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1", IVY_THREAD_EVENT="1"))
    if result != args.reference.read_bytes() or not result.endswith(b"OK\n"):
        raise ValueError("incomplete or changed proof transcript")
    groups = Counter()
    instructions = None
    screen = None
    with errfile.open() as source:
        for line in source:
            if line.startswith("IVY-THREAD-CHECK "):
                _, _, count, empty, enabled, running, _ = line.split()
                if enabled != running or int(running) <= 0 or instructions is not None:
                    raise ValueError("overlapping or multiplexed instruction counter")
                instructions = int(count) - int(empty)
            if line.startswith("IVY-SCREEN "):
                if instructions is None or screen is not None:
                    raise ValueError("missing or overlapping screen count")
                screen = (line.split()[1], instructions)
                instructions = None
            if not line.startswith("IVY-MACRO "):
                continue
            _, stable, seen, slot, lookups, hits = line.split()
            key = (stable, seen, slot)
            if args.screen:
                if screen is None:
                    raise ValueError("missing screen measurement")
                key += (screen[0],)
                groups[key + ("screen_instructions",)] += screen[1]
                screen = None
            if args.instructions:
                if instructions is None:
                    raise ValueError("missing instruction count")
                groups[key + ("instructions",)] += instructions
                instructions = None
            for stat, value in (("passes", 1), ("lookups", int(lookups)), ("hits", int(hits))):
                groups[key + (stat,)] += value
    if not groups or instructions is not None or screen is not None:
        raise ValueError("no observed macro passes")
    out = csv.writer(sys.stdout, lineterminator="\n")
    stats = ["passes", "lookups", "hits"]
    if args.instructions:
        stats.append("instructions")
    fields = ["unchanged_without_macro", "previously_stable", "in_128_slots"]
    if args.screen:
        fields.append("screen_negative")
        stats.append("screen_instructions")
    out.writerow(["isolate", *fields, *stats])
    for key in sorted({k[:-1] for k in groups}):
        out.writerow([args.isolate, *key, *(groups[key + (stat,)] for stat in stats)])


if __name__ == "__main__":
    main()
