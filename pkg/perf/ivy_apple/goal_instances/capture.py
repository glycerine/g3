"""Export selected verification conditions from linked Z3 before it checks them."""
import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from tempfile import TemporaryDirectory

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from phase_counters import NATIVE_METHOD, replace_once


def standard_declarations(text):
    # The linked fork prints two older declaration forms. The expression bodies,
    # binder names and assertion order remain byte-for-byte as exported.
    text = re.sub(r"(?m)^(\s*\(declare-sort [^()\s]+)\)$", r"\1 0)", text)
    return re.sub(r"(?m)^(\s*\(declare-datatypes \(\(([^()\s]+) 0\)\) \(\()\2 ",
                  r"\1", text)


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", required=True, type=Path)
    p.add_argument("--spec", required=True, type=Path)
    p.add_argument("--reference", required=True, type=Path)
    p.add_argument("--output", required=True, type=Path)
    p.add_argument("--isolate", required=True)
    p.add_argument("--checks", nargs="+", required=True, type=int)
    args = p.parse_args()
    if any(n <= 0 for n in args.checks) or len(set(args.checks)) != len(args.checks):
        p.error("check numbers must be positive and distinct")
    args.output = args.output.resolve()
    args.output.mkdir(parents=True)
    ivy = args.goivy.resolve(strict=True)
    original = ivy / "smt/native_verification_condition.go"
    with TemporaryDirectory() as tmp:
        dest = Path(tmp)
        code = replace_once(original.read_text(), "package smt", 'package smt\nimport ("os"; "strconv")')
        code = replace_once(code, NATIVE_METHOD, NATIVE_METHOD.replace("return s.Check()", "captureVC(s); return s.Check()"))
        code = replace_once(code, "return s.Check, func() {}", "return s.CheckVerificationCondition, func() {}")
        code += """
var capturedCheck int
func captureVC(s *Z3Solver) {
    capturedCheck++
    switch capturedCheck {
    case CHECKS:
        name := "check-" + strconv.Itoa(capturedCheck) + ".smt2"
        if err := os.WriteFile(name, []byte(s.ToSMT2()), 0600); err != nil { panic(err) }
    }
}
""".replace("CHECKS", ",".join(map(str, args.checks)))
        patched = dest / original.name
        patched.write_text(code)
        overlay = dest / "overlay.json"
        overlay.write_text(json.dumps({"Replace": {str(original): str(patched), str(original.resolve()): str(patched)}}))
        checker = dest / "native"
        subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-tags=xtrace_off,classic_z3",
                        "-overlay=" + str(overlay), "-o", checker, "./cmd/goivy_check"], cwd=ivy, check=True)
        output = subprocess.check_output([checker, "isolate=" + args.isolate, args.spec.absolute()], cwd=args.output,
                                        env=dict(os.environ, GOMAXPROCS="1"))
    if output != args.reference.read_bytes() or not output.endswith(b"OK\n"):
        raise ValueError("the captured run changed the proof transcript")
    for n in args.checks:
        path = args.output / f"check-{n}.smt2"
        text = standard_declarations(path.read_text())
        path.write_text(text)
        if "(check-sat)" not in text:
            raise ValueError(f"incomplete benchmark for check {n}")


if __name__ == "__main__":
    main()
