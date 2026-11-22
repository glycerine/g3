"""Observe the arena and parameter catalog when the public API constructs its default strategy."""
import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess

PKG = Path(__file__).resolve().parents[3]
BRANCHES = ("qfFPLRA", "qfFP", "nra", "lira", "qfNIA", "qfNRA", "qfLRA", "qfAUFLIA", "qfLIA", "qfAUFBV", "qfBV")

ARENA_OBSERVER = """
func (m *ASTManager) StudyStrategyState(stage string) {
    fmt.Fprintln(os.Stderr, "STRATEGY stage", stage, m.nextNode, m.ExprIDs.Range(), m.declIDs.Range(), m.ExprIDs.FreeIDs(), m.declIDs.FreeIDs())
    for slot := uint32(1); slot < m.nextNode; slot++ {
        n := &m.nodeSlotAt(slot).node
        if n.live {
            fmt.Fprintln(os.Stderr, "STRATEGY node", slot, n.generation, n.kind, n.id, n.hash, n.rc)
        }
    }
}
"""

CATALOG_TEST = """package api

import (
    "encoding/csv"
    "os"
    "strconv"
    "testing"
)

func TestObserveStrategicCatalog(t *testing.T) {
    c, err := NewSMTContext()
    if err != nil { t.Fatal(err) }
    defer c.Close()
    solver, err := c.NewSolver()
    if err != nil { t.Fatal(err) }
    defer solver.Close()
    d, err := solver.ParameterDescriptions()
    if err != nil { t.Fatal(err) }
    defer d.Close()
    file, err := os.Create(os.Getenv("G3_STUDY_CATALOG"))
    if err != nil { t.Fatal(err) }
    defer file.Close()
    rows := csv.NewWriter(file)
    rows.Write([]string{"position", "key", "kind", "doc", "default", "has_default", "module", "has_module"})
    size, err := d.Size()
    if err != nil { t.Fatal(err) }
    for i := uint32(0); i < size; i++ {
        name, err := d.Name(i)
        if err != nil { t.Fatal(err) }
        kind, err := d.Kind(name)
        if err != nil { t.Fatal(err) }
        doc, err := d.Documentation(name)
        if err != nil { t.Fatal(err) }
        value, hasDefault, err := d.value.DefaultName(name)
        if err != nil { t.Fatal(err) }
        module, hasModule, err := d.value.ModuleName(name)
        if err != nil { t.Fatal(err) }
        rows.Write([]string{strconv.Itoa(int(i)), name, strconv.Itoa(int(kind)), doc, value,
            strconv.FormatBool(hasDefault), module, strconv.FormatBool(hasModule)})
    }
    rows.Flush()
    if err := rows.Error(); err != nil { t.Fatal(err) }
}
"""


def probe_factory(source, omit):
    anchor = "// mk_default_tactic is one nested call expression;"
    if source.count(anchor) != 1:
        raise ValueError("default portfolio construction changed")
    source = source.replace(anchor, 'm.StudyStrategyState("start")\n\t' + anchor)
    if omit:
        wrapper = "diagnosticSMT := func() (tactic.Tactic, error) { return smtcore.NewSMTTactic(m, p, config) }"
        source = source.replace(anchor, wrapper + "\n\t" + anchor)
    for name in omit:
        pattern = r"(\n\t" + name + r", err := )smtlogics\.\w+\([^\n]*\)"
        source, n = re.subn(pattern, r"\1diagnosticSMT()", source)
        if n != 1:
            raise ValueError(f"unexpected constructor for {name}")
    pattern = r"(\n\t(ordinary|" + "|".join(BRANCHES) + r"), err (?:=|:=) keepTactic\([^\n]+\))"
    source, n = re.subn(pattern, r'\1\n m.StudyStrategyState("\2")', source)
    if n != len(BRANCHES) + 1:
        raise ValueError("unexpected number of strategy branches")
    return source


def save_stages(log, output):
    stages = []
    for line in log.splitlines():
        fields = line.split()
        if fields[:2] == ["STRATEGY", "stage"]:
            stages.append((fields[2:], set()))
        elif fields[:2] == ["STRATEGY", "node"]:
            stages[-1][1].add(tuple(fields[2:]))
    if len(stages) != len(BRANCHES) + 2:
        raise ValueError("incomplete arena observations")
    with output.open("w") as stream:
        writer = csv.writer(stream, lineterminator="\n")
        writer.writerow(["stage", "live_slots", "new_or_changed_records", "replaced_or_retired_records", "next_slot",
                         "next_expression_id", "next_declaration_id", "free_ids"])
        previous = set()
        for fields, current in stages:
            writer.writerow([fields[0], len(current), len(current - previous), len(previous - current), *fields[1:4],
                             " ".join(fields[4:])])
            previous = current


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--output", required=True, type=Path, help="new directory for the overlay, raw observations and CSVs")
    p.add_argument("--omit", nargs="*", choices=BRANCHES, default=[],
                   help="diagnostic replacement of selected strategy branches by SMT")
    a = p.parse_args()
    root = a.output.resolve()
    root.mkdir(parents=True)
    factory = PKG / "tactic/portfolio/smt_strategic_solver.go"
    arena = PKG / "ast/ast_core_storage.go"
    sources = {factory: probe_factory(factory.read_text(), a.omit),
               arena: arena.read_text().replace("import (", 'import (\n"fmt"\n"os"', 1) + ARENA_OBSERVER,
               PKG / "api/observe_strategy_catalog_test.go": CATALOG_TEST}
    mapping = {}
    for original, source in sources.items():
        patch = root / original.name
        patch.write_text(subprocess.check_output(["gofmt"], input=source, text=True))
        mapping[str(original)] = str(patch)
    overlay = root / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    run = subprocess.run(["go", "test", "-count=1", "-pgo=off", "-overlay=" + str(overlay), "-run",
                          "^TestObserveStrategicCatalog$", "-v", "./api"], cwd=PKG,
                         env=dict(os.environ, G3_STUDY_CATALOG=str(root / "catalog.csv")),
                         stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    (root / "observations").write_text(run.stdout)
    run.check_returncode()
    save_stages(run.stdout, root / "stages.csv")


if __name__ == "__main__":
    main()
