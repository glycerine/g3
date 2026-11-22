"""Count calls on the root of each default strategic tactic in complete GoIvy runs."""
import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess

METHODS = ("Apply", "UpdateParams", "CollectParamDescrs", "CollectStatistics", "ResetStatistics", "Cleanup",
           "Reset", "SetLogic", "SetProgressCallback", "Translate")
PREFIX = "IVY-STRATEGY-USE "


def instrument(goivy, root):
    pkg = Path(subprocess.check_output(["go", "list", "-mod=readonly", "-m", "-f", "{{.Dir}}",
                                        "github.com/glycerine/g3/pkg"], cwd=goivy, text=True).strip())
    sources = {}
    owner = pkg / "tactic/tactic_and_then.go"
    source = owner.read_text().replace("import (", 'import (\n"fmt"\n"os"', 1)
    source = source.replace("type andThenTactic struct {", "type andThenTactic struct {\n studyOrigin uint64", 1)
    for method in METHODS:
        source, count = re.subn(r"(func \(t \*andThenTactic\) " + method + r"\([^\n]*?\{)",
                                r'\1 t.noteUsage("' + method + r'");', source)
        if count != 1:
            raise ValueError(f"unexpected root tactic signature for {method}")
    source += """
func RecordDefaultStrategy(t Tactic, origin uint64) {
    root := t.(*andThenTactic)
    root.studyOrigin = origin
    root.noteUsage("new")
}
func (t *andThenTactic) noteUsage(method string) {
    if t != nil && t.studyOrigin != 0 {
        fmt.Fprintf(os.Stderr, "IVY-STRATEGY-USE %p %d %s\\n", t, t.studyOrigin, method)
    }
}
"""
    sources[owner] = source
    manager = pkg / "ast/ast_core_storage.go"
    sources[manager] = manager.read_text() + "\nfunc (m *ASTManager) StudyStrategyOrigin() uint64 { return m.origin }\n"
    factory = pkg / "tactic/portfolio/smt_strategic_solver.go"
    source = factory.read_text()
    call = "result, err = newStrategicSolver(m, strategy, logic, p, proofs, models, cores, config)"
    if source.count(call) != 1:
        raise ValueError("default solver factory changed")
    sources[factory] = source.replace(call, "tactic.RecordDefaultStrategy(strategy, m.StudyStrategyOrigin())\n " + call)
    mapping = {}
    for original, content in sources.items():
        patched = root / original.name
        patched.write_text(content)
        mapping.update({str(p): str(patched) for p in (original, original.resolve())})
    overlay = root / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    checker = root / "checker"
    subprocess.run(["go", "build", "-mod=readonly", "-buildvcs=false", "-pgo=off", "-tags=xtrace_off",
                    "-overlay=" + str(overlay), "-o", checker, "./cmd/goivy_check"], cwd=goivy, check=True)
    return checker


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", required=True, type=Path)
    p.add_argument("--output", required=True, type=Path)
    p.add_argument("--cpu", required=True, type=int)
    p.add_argument("--workload", nargs=4, action="append", required=True, metavar=("NAME", "SPEC", "ISOLATE_OR_DASH", "REFERENCE"))
    a = p.parse_args()
    names = [entry[0] for entry in a.workload]
    if len(set(names)) != len(names) or any(not re.fullmatch(r"[A-Za-z0-9_-]+", n) for n in names):
        p.error("choose distinct workload names containing letters, digits, underscores or hyphens")
    root = a.output.resolve()
    root.mkdir(parents=True)
    checker = instrument(a.goivy.resolve(strict=True), root)
    with (root / "strategies.csv").open("w") as file:
        writer = csv.writer(file, lineterminator="\n")
        writer.writerow(["workload", "strategy", "manager", *METHODS])
        for name, spec, isolate, ref in a.workload:
            directory = root / name
            directory.mkdir()
            cmd = ["taskset", "-c", str(a.cpu), checker]
            if isolate != "-":
                cmd.append("isolate=" + isolate)
            reference = Path(ref).read_bytes()
            expected = int(any(line.strip() == b"FAIL" for line in reference.splitlines()))
            with (directory / "stderr").open("w") as err:
                result = subprocess.run([*cmd, Path(spec).absolute()], cwd=directory, stdout=subprocess.PIPE,
                                        stderr=err, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"))
            (directory / "stdout").write_bytes(result.stdout)
            if result.returncode != expected or result.stdout != reference:
                raise ValueError(f"{name} changed the reference transcript or exit status")
            counts_by_owner = []
            current = {}
            managers = {}
            for line in (directory / "stderr").read_text().splitlines():
                if line.startswith(PREFIX):
                    address, origin, method = line[len(PREFIX):].split()
                    if method == "new":
                        counts = dict.fromkeys(METHODS, 0)
                        manager = managers.setdefault(origin, len(managers) + 1)
                        counts_by_owner.append((manager, counts))
                        current[address] = counts
                    else:
                        current[address][method] += 1
            for number, (manager, counts) in enumerate(counts_by_owner, 1):
                writer.writerow([name, number, manager, *(counts[method] for method in METHODS)])
            if not counts_by_owner:
                writer.writerow([name, 0, 0, *([0] * len(METHODS))])
            file.flush()


if __name__ == "__main__":
    main()
