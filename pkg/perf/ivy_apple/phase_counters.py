#!/usr/bin/env python3
"""Count user-mode instructions inside selected incremental Apple solver calls."""

import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile


EVENTS = ("instructions:u", "cycles:u", "branches:u", "branch-misses:u")
G3_METHOD = "func (s *Z3Solver) checkVerificationCondition(prove func() (bool, error)) Z3CheckResult {"
NATIVE_METHOD = "func (s *Z3Solver) CheckVerificationCondition() Z3CheckResult { return s.Check() }"
OBSERVER = r'''
var phaseCommand, phaseReply *os.File
var phaseSequence int

func phaseSignal(command string) {
    if phaseCommand == nil {
        var err error
        phaseCommand, err = os.OpenFile(os.Getenv("IVY_PERF_CONTROL"), os.O_WRONLY, 0)
        if err != nil { panic(err) }
        phaseReply, err = os.Open(os.Getenv("IVY_PERF_ACK"))
        if err != nil { panic(err) }
    }
    if _, err := phaseCommand.WriteString(command + "\n"); err != nil { panic(err) }
    var reply [4]byte
    // Some perf versions append a NUL to each acknowledgment.
    for reply[0] == 0 {
        if _, err := io.ReadFull(phaseReply, reply[:1]); err != nil { panic(err) }
    }
    if _, err := io.ReadFull(phaseReply, reply[1:]); err != nil { panic(err) }
    if string(reply[:]) != "ack\n" { panic(fmt.Sprintf("unexpected perf acknowledgment %q", reply)) }
}

func beginMeasuredCheck() func() {
    phaseSequence++
    n := phaseSequence
    selection := os.Getenv("IVY_PERF_SELECTION")
    outside := selection == "outside"
    selected := outside || selection == "inside" || selection == strconv.Itoa(n)
    startCommand, endCommand := "enable", "disable"
    if outside { startCommand, endCommand = endCommand, startCommand }
    if selected { phaseSignal(startCommand) }
    start := time.Now()
    return func() {
        elapsed := time.Since(start)
        if selected { phaseSignal(endCommand) }
        fmt.Fprintln(os.Stderr, "IVY-PERF-CHECK", n, elapsed.Nanoseconds())
    }
}
'''


def replace_once(source, old, new):
    if source.count(old) != 1:
        raise ValueError(f"expected one GoIvy adapter fragment {old!r}")
    return source.replace(old, new)


def build(engine, args, scratch):
    native = engine == "native"
    filename = "native_verification_condition.go" if native else "g3_solver.go"
    original = args.goivy / "smt" / filename
    code = original.read_text()
    imports = [f'"{name}"' for name in ("fmt", "io", "os", "strconv", "time")
               if f'"{name}"' not in code]
    code = replace_once(code, "package smt", "package smt\nimport (\n" + "\n".join(imports) + "\n)")
    if native:
        code = replace_once(code, NATIVE_METHOD, NATIVE_METHOD.replace("return s.Check()",
                            "defer beginMeasuredCheck()(); return s.Check()"))
        code = replace_once(code, "return s.Check, func() {}", "return s.CheckVerificationCondition, func() {}")
    else:
        code = replace_once(code, G3_METHOD, G3_METHOD + "\n defer beginMeasuredCheck()()")
    patched = scratch / filename
    patched.write_text(code + OBSERVER)
    overlay = scratch / (engine + ".json")
    overlay.write_text(json.dumps({"Replace": {str(original): str(patched)}}))
    tags = "xtrace_off,classic_z3" if native else "xtrace_off"
    binary = scratch / engine
    cmd = ["go", "build", "-pgo=off", "-mod=readonly", "-buildvcs=false", f"-tags={tags}",
           f"-overlay={overlay}", "-o", str(binary)]
    if args.modfile:
        cmd.append(f"-modfile={args.modfile.resolve(strict=True)}")
    subprocess.run([*cmd, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    return binary


def measure(binary, selection, directory, args, reference):
    directory.mkdir()
    control, ack = directory / "control", directory / "ack"
    os.mkfifo(control, 0o600)
    os.mkfifo(ack, 0o600)
    env = dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1", IVY_PERF_SELECTION=selection,
               IVY_PERF_CONTROL=str(control), IVY_PERF_ACK=str(ack))
    counts, resources = directory / "perf.csv", directory / "resources"
    command = ["taskset", "-c", str(args.cpu), args.perf, "stat", "--control", f"fifo:{control},{ack}",
               "-x", ",", "-e", ",".join(EVENTS), "-o", str(counts)]
    if selection != "outside":
        command.extend(["-D", "-1"])
    command.extend(["--", "/usr/bin/time", "-f", "%e %U %S %M", "-o", str(resources),
                    str(binary), "isolate=" + args.isolate, str(args.spec)])
    try:
        with (directory / "stdout").open("wb") as out, (directory / "stderr").open("wb") as err:
            subprocess.run(command, cwd=directory, env=env, stdout=out, stderr=err, check=True)
    finally:
        control.unlink()
        ack.unlink()
    if (directory / "stdout").read_bytes() != reference:
        raise ValueError(f"{directory}: full transcript differs from reference")
    times = [line.split()[1:] for line in (directory / "stderr").read_text().splitlines()
             if line.startswith("IVY-PERF-CHECK ")]
    if not times or [int(n) for n, _ in times] != list(range(1, len(times) + 1)):
        raise ValueError(f"{directory}: incomplete solver check sequence")
    if selection.isdecimal() and int(selection) > len(times):
        raise ValueError(f"{directory}: selected check was never reached")
    values = {row[2]: int(row[0]) for row in csv.reader(counts.read_text().splitlines())
              if len(row) > 2 and row[2] in EVENTS}
    return [*(values[key] for key in EVENTS), *resources.read_text().split(), len(times),
            round(sum(int(ns) for _, ns in times) / 1e9, 6)]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", type=Path, required=True)
    p.add_argument("--spec", type=Path, required=True)
    p.add_argument("--reference", type=Path, required=True, help="complete known-good stdout")
    p.add_argument("--isolate", default="rfn2")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--pairs", type=int, default=3)
    p.add_argument("--select", nargs="+", default=["inside", "outside"], metavar="PHASE",
                   help="inside, outside, or a one-based check number")
    p.add_argument("--perf", default="perf", help="Linux perf executable")
    p.add_argument("--modfile", type=Path, help="optional alternate GoIvy module file")
    p.add_argument("--output", type=Path, required=True, help="new directory for the raw measurements")
    args = p.parse_args()
    if args.pairs < 1 or len(set(args.select)) != len(args.select) or any(
            s not in ("inside", "outside") and (not s.isdecimal() or s != str(int(s)) or int(s) < 1)
            for s in args.select):
        p.error("use a positive pair count and distinct valid selections")
    reference = args.reference.read_bytes()
    if not reference.rstrip().endswith(b"\nOK"):
        p.error("the reference transcript must end in OK")
    args.goivy = args.goivy.resolve(strict=True)
    # Ivy quotes the input path verbatim in its transcript.
    args.spec = args.spec.absolute()
    if not args.spec.is_file():
        p.error("the input specification does not exist")
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["selection", "cpu", "round", "engine", *EVENTS, "wall_s", "user_s", "sys_s",
                     "rss_kib", "checks", "inside_wall_s"])
    with tempfile.TemporaryDirectory() as tmp:
        binaries = [build(engine, args, Path(tmp)) for engine in ("g3", "native")]
        for pair in range(1, args.pairs + 1):
            for selection in args.select:
                for binary in binaries if pair % 2 else reversed(binaries):
                    directory = output / f"{pair}-{selection}-{binary.name}"
                    row = measure(binary, selection, directory, args, reference)
                    writer.writerow([selection, args.cpu, pair, binary.name, *row])
                    sys.stdout.flush()


if __name__ == "__main__":
    main()
