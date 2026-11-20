"""Count AST manager accesses on the matched Raft kernel using a Go test overlay."""

import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

from replay_order_kernel import verify

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
METHODS = {
    "ast_core_storage.go": ["nodeAt", "appAt", "declAt", "varAt", "quantifierAt", "sortAt",
                            "validNode", "external"],
    "ast_read_access.go": ["fastHandle"],
    "ast_core_accessors.go": ["NodeHeader", "NodeShape", "AppIdentity", "GetSort", "GetArg", "lockFree"],
    "ast_core_payload.go": ["appArg"],
    "ast_core_factories.go": ["requireKind"],
    "ast_rewriter_access.go": ["SubstitutionShape"],
    "ast_ref.go": ["RefGetHeld"],
}


def sources(work):
    names = []
    replacements = {}
    count = sum(len(methods) for methods in METHODS.values())
    for name, methods in METHODS.items():
        source = ROOT / "ast" / name
        text = source.read_text()
        if name == "ast_core_storage.go":
            end = text.index("\n}", text.index("type ASTManager struct {"))
            text = text[:end] + f"\n studyCalls [{count}]uint64" + text[end:]
            text += f"\nfunc (m *ASTManager) StudyCalls() [{count}]uint64 {{ return m.studyCalls }}\n"
        for method in methods:
            header = rf"(func \(m \*ASTManager\) {method}\([^\n]*\) [^\n]*?\{{)"
            text, matches = re.subn(header, rf"\1\n if m != nil {{m.studyCalls[{len(names)}]++}}", text)
            if matches != 1:
                raise ValueError(f"{name}: found {matches} definitions of {method}")
            names.append(method)
        target = work / name
        target.write_text(text)
        replacements[str(source)] = str(target)
    replacements[str(ROOT / "api/raft_study_calls_test.go")] = str(HERE / "ast_calls_test.go.txt")
    overlay = work / "overlay.json"
    overlay.write_text(json.dumps({"Replace": replacements}))
    return overlay, names


SITE_METHODS = {
    "nodeAt": ("slot uint32", "*astNodeRecord", "slot"),
    "appAt": ("slot uint32", "*astAppRecord", "slot"),
    "declAt": ("slot uint32", "*astFuncDeclRecord", "slot"),
    "validNode": ("node astNodeRef", "error", "node"),
}


def site_sources(work, method):
    tool = work / "sites.go"
    tool.write_bytes((HERE / "ast_call_sites.go.txt").read_bytes())
    sites = json.loads(subprocess.check_output(["go", "run", tool, ROOT / "ast", method], cwd=ROOT))
    if not sites:
        raise ValueError(f"no {method} calls found")
    changes = {}
    count = len(sites)
    # Descending byte offsets keep earlier positions valid after each edit.
    for index in reversed(range(count)):
        site = sites[index]
        name = site["File"]
        if name not in changes:
            changes[name] = (ROOT / "ast" / name).read_bytes()
        data = changes[name]
        offset, lparen = site["Offset"], site["Lparen"]
        if data[offset:offset+len(method)] != method.encode() or data[lparen:lparen+1] != b"(":
            raise ValueError(f"unexpected call at {name}:{site['Line']}")
        changes[name] = (data[:offset] + b"studyCountedCall" + data[offset+len(method):lparen+1]
                         + f"{index}, ".encode() + data[lparen+1:])
    name = "ast_core_storage.go"
    storage = (changes[name].decode() if name in changes else (ROOT / "ast" / name).read_text())
    end = storage.index("\n}", storage.index("type ASTManager struct {"))
    storage = storage[:end] + f"\n studyCalls [{count}]uint64" + storage[end:]
    storage += f"\nfunc(m *ASTManager) StudyCalls() [{count}]uint64 {{return m.studyCalls}}\n"
    parameter, result, argument = SITE_METHODS[method]
    storage += (f"func(m *ASTManager) studyCountedCall(site int, {parameter}) {result} {{\n"
                f" m.studyCalls[site]++; return m.{method}({argument})\n}}\n")
    changes[name] = storage.encode()
    replacements = {}
    for name, data in changes.items():
        target = work / name
        target.write_bytes(data)
        replacements[str(ROOT / "ast" / name)] = str(target)
    replacements[str(ROOT / "api/raft_study_calls_test.go")] = str(HERE / "ast_calls_test.go.txt")
    overlay = work / "overlay.json"
    overlay.write_text(json.dumps({"Replace": replacements}))
    return overlay, sites


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sites", choices=SITE_METHODS, help="sum calls to this method by source location")
    args = parser.parse_args()
    with tempfile.TemporaryDirectory() as tmp:
        work = Path(tmp)
        overlay, names = site_sources(work, args.sites) if args.sites else sources(work)
        output = work / "calls.jsonl"
        subprocess.run(["go", "test", f"-overlay={overlay}", "./api", "-count=1",
                        "-run", "^TestStudyRaftASTCalls$"], cwd=ROOT,
                       env=dict(os.environ, RAFT_CALL_OUTPUT=str(output)), stdout=sys.stderr, check=True)
        rows = [json.loads(line) for line in output.read_text().splitlines()]
    with (HERE / "order-matched-search.csv").open() as source:
        expected = list(csv.DictReader(source))
    if len(rows) != 419 or len(expected) != len(rows):
        raise ValueError("incomplete Raft replay")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    if not args.sites:
        writer.writerow(["obligation", "phase", *names])
    for row, want in zip(rows, expected):
        if row["Obligation"] != int(want["obligation"]):
            raise ValueError("obligations out of order")
        verify("instrumented g3", want, row["Statistics"])
        if not args.sites:
            for phase in ["Load", "Check"]:
                writer.writerow([row["Obligation"], phase.lower(), *row[phase]])
    if args.sites:
        writer.writerow(["file", "line", "caller", "loading_calls", "checking_calls"])
        for i, name in enumerate(names):
            writer.writerow([name["File"], name["Line"], name["Caller"],
                             sum(r["Load"][i] for r in rows), sum(r["Check"][i] for r in rows)])


if __name__ == "__main__":
    main()
