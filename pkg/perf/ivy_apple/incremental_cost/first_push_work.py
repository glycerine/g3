"""Count work done by each rewriter during a fresh solver's first Push in a single-worker GoIvy run."""
import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
from phase_counters import replace_once

COLUMNS = """first_push pass config calls visits distinct_within_root_scope_depth distinct_terms
beta_ground beta_variables bound_replacements beta_probes beta_hits beta_rebuilds beta_unchanged
reduce_app nonfailed_reduce_app get_subst_found beta_keys_fitting duplicate_beta_calls duplicate_beta_visits""".split()


def build(ivy, directory):
    pkg = HERE.parents[2]
    module = json.loads(subprocess.check_output(
        ["go", "list", "-mod=readonly", "-m", "-json", "github.com/glycerine/g3/pkg"], cwd=ivy))
    local = Path(module["Dir"])
    if not local.samefile(pkg):
        raise ValueError(f"GoIvy uses {local}, expected {pkg}")
    replacements = {}

    def patch(path, code):
        target = directory / path.name
        target.write_text(code)
        roots = (path, local / path.relative_to(pkg)) if path.is_relative_to(pkg) else (path, path.resolve())
        replacements.update({str(source): str(target) for source in roots})

    rewriter = pkg / "ast/rewriter"
    patch(rewriter / "first_push_counts.go", (HERE / "first_push_counts.go.txt").read_text())
    engine = rewriter / "rewriter_engine.go"
    code = engine.read_text()
    hooks = [
        ("func (r *rewriterEngine) rewrite(t ast.ASTHandle, out, proof ast.ASTRef) error {",
         "defer censusRewrite(r,t)()"),
        ("func (r *rewriterEngine) visit(t ast.ASTHandle, depth uint32) (bool, error) {", "censusNode(r,t,depth)"),
        ("if found {", "if censusCurrent!=nil { censusCurrent.counts.substitutions++ }"),
        ("h := r.bindings[i]\n\t\tif !h.IsNull() {", "if censusCurrent!=nil { censusCurrent.counts.replacements++ }"),
        ("st, e := r.cfg.ReduceApp(d, args, r.r, r.pr2)",
         "if censusCurrent!=nil { censusCurrent.counts.reductions++; if st!=brFailed && e==nil { censusCurrent.counts.successful++ } }"),
        ("// flattening. Rebuild exactly when a child changed.\n\t\t\th := t",
         "if censusCurrent!=nil { if fr.newChild() { censusCurrent.counts.rebuilds++ } else { censusCurrent.counts.unchanged++ } }"),
    ]
    for marker, hook in hooks:
        code = replace_once(code, marker, marker + "\n" + hook)
    patch(engine, code)
    subst = rewriter / "var_substitution.go"
    code = subst.read_text()
    for marker, field in [("if ground {", "ground"), ("if kind == ast.ASTVar {", "variables"),
                          ("if cache {", "probes"), ("if !h.IsNull() {", "hits")]:
        code = replace_once(code, marker, marker + f"\nif censusCurrent!=nil {{ censusCurrent.counts.{field}++ }}")
    patch(subst, code)

    driver = pkg / "smtcore/smt_asserted_driver.go"
    code = driver.read_text()
    imported = '"github.com/glycerine/g3/pkg/ast/rewriter"'
    if imported not in code:
        code = replace_once(code, "package smtcore", "package smtcore\nimport " + imported)
    marker = 'if err := assertedVerbose(10, "(smt."+pass.sourceName()'
    patch(driver, replace_once(code, marker, "defer rewriter.CensusPass(pass.sourceName())()\n" + marker))

    adapter = ivy / "smt/g3_solver.go"
    code = replace_once(adapter.read_text(), "package smt", "package smt\nimport " + imported)
    code = replace_once(code, "type Z3Solver struct {", "type Z3Solver struct {\n censusPushed bool")
    code = replace_once(code, "Push() { fail(s.c.Push()) }", """Push() {
        if !s.censusPushed { s.censusPushed = true; defer rewriter.CensusPush()() }
        fail(s.c.Push())
    }""")
    reset = "func (s *Z3Solver) Reset() {"
    patch(adapter, replace_once(code, reset, reset + "\n s.censusPushed = false"))
    overlay = directory / "overlay.json"
    overlay.write_text(json.dumps({"Replace": replacements}))
    binary = directory / "checker"
    subprocess.run(["go", "build", "-mod=readonly", "-buildvcs=false", "-pgo=off", "-tags=xtrace_off",
                    "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"], cwd=ivy, check=True)
    return binary


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("goivy", "spec", "reference", "output"):
        p.add_argument("--" + name, required=True, type=Path)
    p.add_argument("--isolate", required=True)
    p.add_argument("--cpu", required=True, type=int)
    a = p.parse_args()
    directory = a.output.resolve()
    directory.mkdir(parents=True)
    binary = build(a.goivy.resolve(), directory)
    expected = a.reference.read_bytes()
    if not expected.endswith(b"OK\n"):
        raise ValueError("require a complete success reference")
    with (directory / "stdout").open("wb") as out, (directory / "stderr").open("wb") as err:
        subprocess.run(["taskset", "-c", str(a.cpu), binary, "isolate=" + a.isolate, a.spec.absolute()],
                       cwd=directory, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"),
                       stdout=out, stderr=err, check=True)
    if (directory / "stdout").read_bytes() != expected:
        raise ValueError("complete transcript differed from reference")
    rows = [line.split()[1:] for line in (directory / "stderr").read_text().splitlines()
            if line.startswith("PUSH-CENSUS ")]
    if not rows or any(len(row) != len(COLUMNS) for row in rows):
        raise ValueError("missing or incomplete first-push counters")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(COLUMNS)
    writer.writerows(sorted(rows, key=lambda row: (int(row[0]), row[1], row[2])))


if __name__ == "__main__":
    main()
