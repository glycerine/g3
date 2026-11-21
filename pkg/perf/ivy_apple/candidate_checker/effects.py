"""Compare ordered quantifier-candidate side effects within one complete GoIvy proof check."""

import argparse
from collections import Counter
import csv
import filecmp
import json
import os
import re
from pathlib import Path
import struct
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from search_counters import GO_METHOD

EVENTS = ["candidate_start", "candidate_result", "temporary_equality", "disequality_request", "congruence_bucket"]


def edit(code, anchor, replacement):
    if code.count(anchor) != 1:
        raise ValueError(f"expected one occurrence of {anchor!r}")
    return code.replace(anchor, replacement)


def build(args, directory, overlay):
    local = Path(subprocess.check_output(["go", "list", "-mod=readonly", *args.modflags, "-m", "-f", "{{.Dir}}",
                                          "github.com/glycerine/g3/pkg"], cwd=args.goivy, text=True).strip())
    roots = {local, local.resolve(strict=True)}
    mapping = json.loads(Path(overlay).read_text())["Replace"] if overlay != "-" else {}

    def patch(name, transform):
        originals = {root / name for root in roots}
        source = next((Path(mapping[str(p)]) for p in originals if str(p) in mapping), local / name)
        path = directory / Path(name).name
        path.write_text(transform(source.read_text()))
        mapping.update({str(p): str(path) for p in originals})

    line = "satisfied, err := b.checker.run(body, true, bindings, stat)"
    patch("smtcore/smt_qi_instance.go", lambda s: edit(s, line,
          "startObservedCandidate(b.manager, quantifier, len(bindings))\n" + line + "\nendObservedCandidate(satisfied)"))

    def checker(s):
        line = "c.diseqTmp, c.diseqTmpHeld = ref, true"
        s = edit(s, line, line + "\nobserveCandidateASTs(m,2,eq,first.owner,second.owner)")
        line = "if m == nil || first == nil || second == nil {\n\t\treturn\n\t}"
        return edit(s, line, line + "\nobserveCandidateASTs(m,3,ast.ASTHandle{},first.owner,second.owner)")

    patch("smtcore/smt_checker.go", checker)
    line = "t.buckets = append(t.buckets, bucket)"
    patch("smtcore/smt_cg_table.go", lambda s: edit(s, line,
          "observeCandidateASTs(t.manager,4,node.declaration,ast.ASTHandle{},ast.ASTHandle{})\n" + line))
    helper = Path(__file__).with_name("effects.go.in").resolve()
    mapping.update({str(root / "smtcore/observer_candidate_effects.go"): str(helper) for root in roots})
    adapter = args.goivy / "smt/g3_solver.go"
    src = Path(mapping.get(str(adapter), adapter)).read_text()
    src = edit(src, "import (", 'import (\n candidateObserver "github.com/glycerine/g3/pkg/smtcore"')
    src = edit(src, GO_METHOD, "var candidateObserverCheck int\n" + GO_METHOD + """
        candidateObserverCheck++
        if candidateObserverCheck == CHECK {
            candidateObserver.BeginCandidateObservation()
            defer candidateObserver.EndCandidateObservation()
        }
    """.replace("CHECK", str(args.check)))
    patched = directory / adapter.name
    patched.write_text(src)
    mapping.update({str(p): str(patched) for p in {adapter, adapter.resolve()}})
    ov = directory / "overlay.json"
    ov.write_text(json.dumps({"Replace": mapping}))
    binary = directory / "checker"
    subprocess.run(["go", "build", "-pgo=off", "-buildvcs=false", "-mod=readonly", "-tags=xtrace_off", *args.modflags,
                    f"-overlay={ov}", "-o", binary, "./cmd/goivy_check"], check=True, cwd=args.goivy)
    return binary


def run(args, label, overlay):
    directory = args.output / label
    directory.mkdir()
    binary = build(args, directory, overlay)
    with (directory / "stderr").open("wb") as errors:
        transcript = subprocess.check_output(["taskset", "-c", str(args.cpu), binary,
                                             f"isolate={args.isolate}", args.spec], cwd=directory, stderr=errors,
                                            env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"))
    (directory / "stdout").write_bytes(transcript)
    if transcript != args.reference.read_bytes():
        raise ValueError(f"{label} changed the proof transcript")
    return directory / "effects.bin"


def counts(path):
    totals = Counter()
    with path.open("rb") as source:
        while chunk := source.read(24 * 4096):
            for _, _, kind, _, _, _ in struct.iter_unpack("<IIIIII", chunk):
                totals[EVENTS[kind]] += 1
    if not totals["candidate_start"] or totals["candidate_start"] != totals["candidate_result"]:
        raise ValueError(f"incomplete candidate observations in {path}")
    return totals


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ["goivy", "spec", "reference", "output"]:
        p.add_argument("--" + name, required=True, type=Path)
    p.add_argument("--modfile", type=Path)
    p.add_argument("--variant", action="append", nargs=2, required=True, metavar=("LABEL", "OVERLAY_OR_DASH"))
    p.add_argument("--isolate", default="rfn")
    p.add_argument("--check", type=int, default=29)
    p.add_argument("--cpu", required=True, type=int)
    args = p.parse_args()
    args.goivy = args.goivy.resolve(strict=True)
    args.spec = args.spec.absolute()  # GoIvy prints this spelling in its transcript.
    args.reference = args.reference.resolve(strict=True)
    if not args.spec.is_file():
        p.error("missing specification")
    args.output = args.output.resolve()
    args.output.mkdir(parents=True, exist_ok=False)
    args.modflags = [f"-modfile={args.modfile.resolve(strict=True)}"] if args.modfile else []
    if (args.check < 1 or len(dict(args.variant)) != len(args.variant) or
            any(not re.fullmatch(r"[A-Za-z0-9_-]+", label) for label, _ in args.variant)):
        p.error("supply a positive check number and distinct alphanumeric variant labels")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["variant", "isolate", "check", "total", *EVENTS, "matches_first"])
    first = None
    for label, overlay in args.variant:
        path = run(args, label, overlay)
        if first is None:
            first = path
        result = counts(path)
        same = filecmp.cmp(path, first, shallow=False)
        writer.writerow([label, args.isolate, args.check, sum(result.values()), *(result[k] for k in EVENTS), int(same)])
        sys.stdout.flush()
        if not same:
            raise ValueError(f"{path} differs from {first}")


if __name__ == "__main__":
    main()
