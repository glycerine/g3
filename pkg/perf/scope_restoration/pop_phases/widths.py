"""Compare queue-enabled g3 and in-tree Z3 scope pops at several atom counts."""

import argparse
import csv
import json
import os
from pathlib import Path
from types import SimpleNamespace

from measure import PHASES, PKG, ROOT, WARMUP, build_go, build_native, measure, replace_once


def go_width_fixture(path):
    source = replace_once(path.read_text(), '"fmt"', '"fmt"\n"os"\n"strconv"')
    start = source.index("func benchmarkSMTScopeRestoration(")
    setup, body = source[:start], source[start:]
    body = body.replace("smtHotPathWidth", "width")
    body = replace_once(body, "for _, scoped :=", """width, err := strconv.Atoi(os.Getenv("SCOPE_ATOMS"))
    if err != nil || width < 4 || width%2 != 0 { b.Fatal("invalid scope width") }
    for _, scoped :=""")
    body = replace_once(body, "const clauses = width / 2", "clauses := width / 2")
    body = replace_once(body, "var original [clauses]*SMTClause", "original := make([]*SMTClause, clauses)")
    for name in ("clauses", "width"):
        body = replace_once(body, f"b.ReportMetric({name},", f"b.ReportMetric(float64({name}),")
    return setup + body


def native_width_fixture():
    source = (ROOT / "src/test/smt_scope_restoration.cpp").read_text()
    source = replace_once(source, "#include <limits>", "#include <limits>\n#include <vector>")
    source = replace_once(source, "constexpr unsigned num_atoms = 128;\nconstexpr unsigned num_clauses = num_atoms / 2;", "")
    source = replace_once(source, "ast_manager m;", """unsigned const num_atoms = std::stoul(std::getenv("SCOPE_ATOMS"));
    VERIFY(num_atoms >= 4 && num_atoms % 2 == 0);
    unsigned const num_clauses = num_atoms / 2;
    ast_manager m;""")
    source = replace_once(source, "smt::literal literals[num_atoms];", "std::vector<smt::literal> literals(num_atoms);")
    return replace_once(source, "smt::clause* clauses[num_clauses];", "std::vector<smt::clause*> clauses(num_clauses);")


def build_width_fixtures(output, build):
    path = PKG / "smtcore/smt_hotpath_benchmark_test.go"
    fixture = output / path.name
    fixture.write_text(go_width_fixture(path))
    overlay = output / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {str(path): str(fixture)}}))
    return {"g3": build_go(output / "g3", overlay, "instructions:u"),
            "native": build_native(output / "native", build.resolve(), "instructions:u", native_width_fixture())}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True, help="new output directory")
    parser.add_argument("--build", type=Path, default=ROOT / "build", help="in-tree Z3 make build with libz3.a")
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--rounds", type=int, default=5)
    parser.add_argument("--widths", nargs="+", type=int, default=[4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096])
    args = parser.parse_args()
    if args.rounds < 1 or any(w < 4 or w % 2 for w in args.widths):
        parser.error("use positive rounds and even widths of at least four atoms")
    output = args.output.resolve()
    output.mkdir(parents=True)
    bins = build_width_fixtures(output, args.build)
    with (output / "runs.csv").open("w") as dest:
        writer = csv.writer(dest)
        writer.writerow("round width case phase engine intervals instructions enabled running".split())
        for rr in range(args.rounds):
            offset = 2 * rr % len(args.widths)
            for width in args.widths[offset:] + args.widths[:offset]:
                os.environ["SCOPE_ATOMS"] = str(width)
                iterations = WARMUP + max(32, min(1200, 98304 // width))
                settings = SimpleNamespace(cpu=args.cpu, iterations=iterations, thread_event="instructions:u", timeout=150)
                for case, phases in [("ScopedAtoms", PHASES), ("SurvivingAtoms", ("empty", "pop"))]:
                    for phase in phases:
                        for name in list(bins)[::1 if rr % 2 == 0 else -1]:
                            run = output / f"{rr+1}-{width}-{case}-{phase}-{name}"
                            counts = measure(run, bins[name], name, case, phase, settings)
                            fields = (run / "out").read_text().split()
                            if float(fields[fields.index("atoms/op") - 1]) != width:
                                raise ValueError(f"fixture used the wrong width in {run}")
                            writer.writerow([rr + 1, width, case, phase, name, *counts])
                            dest.flush()


if __name__ == "__main__":
    main()
