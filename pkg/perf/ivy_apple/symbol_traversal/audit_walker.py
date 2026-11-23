"""Benchmark both GoIvy symbol walkers on the same checkout with Go file overlays."""
import argparse
import csv
import json
import os
from pathlib import Path
import subprocess


def old_file(module, filename):
    return subprocess.check_output(["git", "show", "3ef4b9020:goivy/" + filename],
                                   cwd=module, text=True)


def arguments_function(source):
    start = source.index("func NodeArgs(n Expr)")
    switch = source.index("switch t := n.(type)", start)
    return source[start:source.index("\n}", switch) + 2]


def build(module, dest, variant, env):
    walker, util = "ivylogic_symbols.go", "ivylogic_util.go"
    overlays = {str(module / "audit_symbol_fallback_test.go"):
                str(Path(__file__).with_name("iff_ite_benchmark_test.go.txt").resolve())}
    if variant != "current":
        current = (module / util).read_text()
        historical = old_file(module, util)
        (dest / util).write_text(current.replace(arguments_function(current), arguments_function(historical)))
        source = old_file(module, walker)
        if variant == "combined":
            source = (module / walker).read_text()
            anchor = "return symbolsIluAstArgs(NodeArgs(node), yield)"
            if source.count(anchor) != 1:
                raise ValueError("current symbol walker has changed")
            source = source.replace(anchor, "var scratch [3]Expr\n\treturn symbolsIluAstArgs(nodeArgsInto(node, scratch[:0]), yield)")
        (dest / walker).write_text(source)
        for filename in (walker, util):
            overlays[str(module / filename)] = str(dest / filename)
    overlay = dest / "overlay.json"
    overlay.write_text(json.dumps({"Replace": overlays}))
    exe = dest / "bench.test"
    subprocess.run(["go", "test", "-mod=readonly", "-tags=xtrace_off", "-pgo=off", "-overlay=" + str(overlay),
                    "-c", "-o", exe, "."], cwd=module, env=env, check=True)
    return exe


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--rounds", type=int, default=5)
    args = parser.parse_args()
    module, output = args.goivy.resolve(), args.output.resolve()
    output.mkdir(parents=True)
    bins = {}
    for variant in ("current", "orphan", "combined"):
        dest = output / variant
        dest.mkdir()
        bins[variant] = build(module, dest, variant, os.environ)
    with (output / "micro.csv").open("w") as file:
        writer = csv.writer(file, lineterminator="\n")
        writer.writerow(["round", "cpu", "variant", "fixture", "iterations", "ns_per_op", "bytes_per_op", "allocs_per_op"])
        for i in range(args.rounds):
            for variant in (list(bins) if i % 2 == 0 else reversed(bins)):
                cmd = ["taskset", "-c", str(args.cpu), bins[variant], "-test.run=^Test(SymbolOccurrence|SymbolsIluAst|SymbolsAsts)",
                       "-test.bench=^BenchmarkSymbols(IluAstTraversal|IffIteAudit)$", "-test.benchtime=15000x", "-test.benchmem"]
                log = subprocess.check_output(cmd, cwd=module, env=dict(os.environ, GOMAXPROCS="1"), text=True)
                (output / variant / f"round-{i + 1}.log").write_text(log)
                for line in log.splitlines():
                    if line.startswith("Benchmark"):
                        fixture, iterations, ns, _, size, _, allocs, _ = line.split()
                        writer.writerow([i + 1, args.cpu, variant, fixture, iterations, ns, size, allocs])
                file.flush()


if __name__ == "__main__":
    main()
