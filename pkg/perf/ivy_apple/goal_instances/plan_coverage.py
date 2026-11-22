"""Attribute private ground, expanded-witness and short-premise attempts in a live GoIvy checker."""

import argparse
import csv
import json
import os
from pathlib import Path
import subprocess


def replace_once(source, needle, replacement):
    if source.count(needle) != 1:
        raise ValueError(f"expected one occurrence of {needle!r}")
    return source.replace(needle, replacement)


def build(pkg, ivy, directory):
    proof = pkg / "api/api_goal_instances_check.go"
    code = replace_once(proof.read_text(), '"errors"', '"errors"\n"fmt"\n"os"')
    hook = """
    defer func() {
        fmt.Fprintf(os.Stderr, "G3-PLAN %T %t %t\\n", plan, proved, err != nil)
    }()
"""
    code = replace_once(code, "aux := &orderProofState{source: p.source}", hook + "aux := &orderProofState{source: p.source}")
    target = directory / proof.name
    target.write_text(code)
    replacements = {str(proof): str(target)}
    # go build uses the path spelling in GoIvy's local module replacement.
    module = json.loads(subprocess.check_output(["go", "list", "-mod=readonly", "-m", "-json",
                        "github.com/glycerine/g3/pkg"], cwd=ivy))
    module_pkg = Path(module["Dir"])
    if not module_pkg.samefile(pkg):
        raise ValueError(f"GoIvy uses {module_pkg}, expected {pkg}")
    replacements[str(module_pkg / proof.relative_to(pkg))] = str(target)

    adapter = ivy / "smt/g3_solver.go"
    code = adapter.read_text()
    marker = "func (s *Z3Solver) checkVerificationCondition(prove func() (bool, error)) Z3CheckResult {"
    code = replace_once(code, marker, marker + '\nprintln("G3-CHECK")')
    target = directory / adapter.name
    target.write_text(code)
    replacements[str(adapter)] = str(target)
    replacements[str(adapter.resolve())] = str(target)
    overlay = directory / "overlay.json"
    overlay.write_text(json.dumps({"Replace": replacements}))
    binary = directory / "checker"
    subprocess.run(["go", "build", "-mod=readonly", "-buildvcs=false", "-pgo=off",
                    "-tags=xtrace_off", "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"],
                   cwd=ivy, check=True)
    return binary


def measure(binary, directory, cpu, workload, writer):
    name, spec, isolate, expected_exit, reference = workload
    run = directory / name
    run.mkdir()
    args = [binary, *([] if isolate == "-" else ["isolate=" + isolate]), Path(spec).absolute()]
    with (run / "stdout").open("wb") as out, (run / "stderr").open("wb") as err:
        result = subprocess.run(["taskset", "-c", str(cpu), *args], cwd=run, stdout=out, stderr=err,
                                env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"))
    if result.returncode != int(expected_exit) or (run / "stdout").read_bytes() != Path(reference).read_bytes():
        raise ValueError(f"{name} differed from the reference")
    check = 0
    attempts = []
    for line in (run / "stderr").read_text().splitlines() + ["G3-CHECK"]:
        if line == "G3-CHECK":
            if check:
                for plan, proved, error in attempts or [("", "", "")]:
                    writer.writerow([name, check, plan, proved, error])
            check += 1
            attempts.clear()
        elif line.startswith("G3-PLAN "):
            if not check:
                raise ValueError(f"unexpected attempt before first check in {name}")
            _, plan, proved, error = line.split()
            attempts.append((plan.removeprefix("*api."), proved, error))
    if check == 1:
        raise ValueError(f"no verification calls in {name}")


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", required=True, type=Path)
    p.add_argument("--output", required=True, type=Path)
    p.add_argument("--cpu", required=True, type=int)
    p.add_argument("--workload", action="append", nargs=5, required=True,
                   metavar=("NAME", "SPEC", "ISOLATE_OR_DASH", "EXIT", "REFERENCE_STDOUT"))
    args = p.parse_args()
    directory = args.output.resolve()
    directory.mkdir(parents=True)
    binary = build(Path(__file__).resolve().parents[3], args.goivy.absolute(), directory)
    with (directory / "coverage.csv").open("w") as out:
        writer = csv.writer(out, lineterminator="\n")
        writer.writerow(["workload", "check", "plan", "proved", "error"])
        for workload in args.workload:
            measure(binary, directory, args.cpu, workload, writer)
            out.flush()


if __name__ == "__main__":
    main()
