"""Count instructions in Push, Pop, Assert and Reset outside Ivy verification calls."""
import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from thread_checks import HERE, PACKAGES
from phase_counters import G3_METHOD, NATIVE_METHOD, replace_once


def build(args, engine):
    directory = args.output / engine
    directory.mkdir()
    smt = args.goivy / "smt"
    mapping = {}
    if args.go_overlay and engine == "g3":
        mapping.update(json.loads(args.go_overlay.read_text())["Replace"])

    def patch(name, transform):
        path = smt / name
        source = Path(mapping.get(str(path), path)).read_text()
        target = directory / name
        target.write_text(transform(source))
        mapping.update({str(p): str(target) for p in (path, path.resolve())})

    def mutations(source):
        for name in ["Push", "Pop", "Assert", "Reset"]:
            pattern = r"(func \(s \*Z3Solver\) " + name + r"\([^)]*\)\s*\{)"
            source, count = re.subn(pattern, r'\1\n defer ivyMeasureMutation("' + name + r'")()\n', source)
            if count != 1:
                raise ValueError(f"missing {name} in {engine}")
        return source

    def exclude_check(source):
        if engine == "g3":
            return replace_once(source, G3_METHOD, G3_METHOD + "\n defer ivyInsideCheck()()")
        source = replace_once(source, NATIVE_METHOD, NATIVE_METHOD.replace("return s.Check()",
                              "defer ivyInsideCheck()(); return s.Check()"))
        return replace_once(source, "return s.Check, func() {}", "return s.CheckVerificationCondition, func() {}")

    if engine == "g3":
        patch("g3_solver.go", lambda s: mutations(exclude_check(s)))
    else:
        patch("quantifier.go", mutations)
        patch("native_verification_condition.go", exclude_check)
    helper = (HERE / "thread_counter.go.txt").read_text()
    helper = replace_once(helper, "func beginMeasuredCheck() func() {", """
var ivyCheckDepth int
func ivyInsideCheck() func() {
    ivyCheckDepth++
    return func() { ivyCheckDepth-- }
}
func ivyMeasureMutation(operation string) func() {
    if ivyCheckDepth != 0 { return func() {} }
""")
    helper = replace_once(helper, '"IVY-THREAD-CHECK", check', '"IVY-MUTATION", operation, check')
    header = "package smt\nimport (\n" + "\n".join(f'"{name}"' for name in PACKAGES) + "\n)\n"
    target = directory / "mutation_counter.go"
    target.write_text(header + helper)
    mapping[str(smt / target.name)] = str(target)
    overlay = directory / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    tags = "xtrace_off,classic_z3" if engine == "native" else "xtrace_off"
    binary = directory / "checker"
    flags = [f"-modfile={args.modfile.resolve(strict=True)}"] if args.modfile else []
    subprocess.run(["go", "build", "-buildvcs=false", "-pgo=off", "-mod=readonly", "-tags=" + tags,
                    *flags, "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"],
                   cwd=args.goivy, check=True)
    return binary


def measure(args, engine, binary, name, reference, pair):
    directory = args.output / f"{name}-{pair}-{engine}"
    directory.mkdir()
    with (directory / "stderr").open("wb") as err:
        out = subprocess.check_output(["taskset", "-c", str(args.cpu), binary, "isolate=" + name, args.spec],
                                     cwd=directory, stderr=err,
                                     env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1", IVY_THREAD_EVENT="1"))
    if out != reference:
        raise ValueError(f"{directory} changed the proof transcript")
    rows = []
    for line in (directory / "stderr").read_text().splitlines():
        if not line.startswith("IVY-MUTATION "):
            continue
        _, operation, *fields = line.split()
        seq, count, empty, enabled, running, wall = map(int, fields)
        if enabled != running or running <= 0 or count <= 0:
            raise ValueError(f"incomplete instruction count in {directory}")
        rows.append((seq, operation, count, empty, enabled, running, wall))
    rows.sort()
    if not rows or [r[0] for r in rows] != list(range(1, len(rows) + 1)):
        raise ValueError(f"incomplete sequence in {directory}")
    return rows


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("goivy", "spec", "output"):
        p.add_argument("--" + name, type=Path, required=True)
    p.add_argument("--modfile", type=Path)
    p.add_argument("--go-overlay", type=Path)
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--pairs", type=int, default=3)
    p.add_argument("--isolate", action="append", nargs=2, required=True, metavar=("NAME", "REFERENCE_STDOUT"))
    a = p.parse_args()
    a.goivy, a.spec, a.output = a.goivy.resolve(strict=True), a.spec.absolute(), a.output.resolve()
    references = {n: Path(ref).read_bytes() for n, ref in a.isolate}
    if len(references) != len(a.isolate) or not a.spec.is_file() or a.pairs < 1 or any(
            not re.fullmatch(r"[a-zA-Z0-9_-]+", n) or not r.rstrip().endswith(b"\nOK")
            for n, r in references.items()):
        p.error("supply distinct isolate names, successful references and a positive pair count")
    a.output.mkdir(parents=True)
    binaries = {engine: build(a, engine) for engine in ("g3", "native")}
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "cpu", "pair", "engine", "sequence", "operation", "count", "empty",
                     "enabled_ns", "running_ns", "wall_ns"])
    expected = {}
    for pair in range(1, a.pairs + 1):
        for name, reference in references.items():
            for engine in list(binaries)[::1 if pair % 2 else -1]:
                rows = measure(a, engine, binaries[engine], name, reference, pair)
                operations = tuple(row[1] for row in rows)
                if operations != expected.setdefault(name, operations):
                    raise ValueError(f"{name} operation sequence differs from the reference run")
                writer.writerows((name, a.cpu, pair, engine, *row) for row in rows)
                sys.stdout.flush()


if __name__ == "__main__":
    main()
