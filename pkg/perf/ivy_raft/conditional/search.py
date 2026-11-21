"""Measure every private order-solver call in one complete GoIvy Raft run."""

import argparse
import csv
from contextlib import contextmanager
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
SOURCE = HERE.parents[2] / "api/api_order_abstraction.go"
METHOD = "func (p *orderProofState) prove(maxSteps uint32, reuse bool) (proved bool, err error) {"
KEYS = ["decisions", "quant-instantiations", "conflicts", "rlimit-count"]
OBSERVER = '''
func (p *orderProofState) studyStats() string {
    stats, err := p.child.Statistics()
    if err != nil { panic(err) }
    text, err := stats.String()
    if err != nil { panic(err) }
    if err := stats.Close(); err != nil { panic(err) }
    return text
}
'''


def replace_once(code, old, new):
    if code.count(old) != 1:
        raise ValueError(f"unexpected order solver source at {old!r}")
    return code.replace(old, new)



def observed_source():
    code = replace_once(SOURCE.read_text(), "import (", 'import (\n"encoding/json"\n"fmt"\n"os"')
    code = replace_once(code, METHOD, METHOD + '\nfmt.Fprintln(os.Stderr, "RAFT-PROOF")')
    check = "answer, err = p.child.Check()"
    code = replace_once(code, check, "before := p.studyStats()\n" + check + '''
    report, e := json.Marshal([2]string{before, p.studyStats()})
    if e != nil { panic(e) }
    fmt.Fprintln(os.Stderr, "RAFT-SEARCH", string(report))
''')
    return code + OBSERVER


@contextmanager
def observed_checker(ivy, code):
    with tempfile.TemporaryDirectory() as scratch:
        tmp = Path(scratch)
        patched, overlay, exe = tmp / "observed.go", tmp / "overlay.json", tmp / "goivy_check"
        patched.write_text(code)
        module = subprocess.check_output(
            ["go", "list", "-mod=readonly", "-m", "-f", "{{.Dir}}", "github.com/glycerine/g3/pkg"],
            cwd=ivy, text=True).strip()
        dependency = Path(module) / "api/api_order_abstraction.go"
        if not SOURCE.samefile(dependency):
            raise ValueError(f"GoIvy must use this g3 checkout, found {module}")
        # Build overlays use the spelling of the path in a local module replacement.
        overlay.write_text(json.dumps({"Replace": {str(f): str(patched) for f in (SOURCE, dependency)}}))
        subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-tags=xtrace_off",
                        f"-overlay={overlay}", "-o", exe, "./cmd/goivy_check"], cwd=ivy, check=True)
        yield exe


def measure(exe, spec, cpu, env=os.environ):
    run = subprocess.run(["taskset", "-c", str(cpu), exe, spec], cwd=exe.parent,
                         env=dict(env, GOMAXPROCS="1", XTRACE_OFF="1"),
                         capture_output=True, text=True, check=True)
    transcript = run.stdout.splitlines()
    if transcript.count("PASS") != 419 or transcript[-1:] != ["OK"]:
        raise ValueError("incomplete Raft checker transcript")
    obligation = 0
    rows = {}
    for line in run.stderr.splitlines():
        if line == "RAFT-PROOF":
            obligation += 1
        if line.startswith("RAFT-SEARCH "):
            pair = json.loads(line.split(" ", 1)[1])
            old, new = [{k: int(v) for k, v in re.findall(r":([\w-]+)\s+(\d+)(?=[)\s])", value)}
                        for value in pair]
            rows.setdefault(obligation, []).append([new.get(k, 0) - old.get(k, 0) for k in KEYS])
    if obligation != 419 or not rows:
        raise ValueError("missing private solver measurements, check the GoIvy local g3 replacement")
    return rows


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--goivy", type=Path, required=True)
    p.add_argument("--spec", type=Path, required=True)
    p.add_argument("--cpu", type=int, required=True)
    args = p.parse_args()
    ivy, spec = args.goivy.resolve(strict=True), args.spec.resolve(strict=True)
    with observed_checker(ivy, observed_source()) as exe:
        rows = measure(exe, spec, args.cpu)
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["obligation", "stage", *KEYS])
    for n, calls in rows.items():
        for i, counters in enumerate(calls):
            writer.writerow([n, "main" if i + 1 == len(calls) else "condition", *counters])


if __name__ == "__main__":
    main()
