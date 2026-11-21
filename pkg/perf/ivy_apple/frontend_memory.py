"""Measure Go runtime allocation and GC work over complete matched Ivy proofs."""
import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from compare import EVENTS, measure

METRICS = (
    "/cpu/classes/gc/mark/assist:cpu-seconds", "/cpu/classes/gc/mark/dedicated:cpu-seconds",
    "/cpu/classes/gc/mark/idle:cpu-seconds", "/cpu/classes/gc/total:cpu-seconds",
    "/gc/scan/heap:bytes", "/gc/heap/allocs:bytes", "/gc/heap/allocs:objects",
    "/gc/cycles/total:gc-cycles", "/gc/heap/goal:bytes", "/gc/heap/live:bytes",
)
CALLS = {
    "symbolsIluAstRec": "ivylogic_symbols.go",
    "renameASTRec": "module_astutil.go",
    "UnusedNameWithBase": "ivyutils_renamer.go",
    "sliceSexp": "actions_action_expr.go",
    "UsedSymbolNamesClauses": "module_ops.go",
    "ComposeUpdates": "actions_transrel.go",
}


def compile_checker(args, engine):
    target = args.output / engine
    target.mkdir()
    paths = json.loads(args.frontend_overlay.read_text())["Replace"] if args.frontend_overlay else {}
    if engine == "g3" and args.go_overlay:
        paths.update(json.loads(args.go_overlay.read_text())["Replace"])

    def original(name):
        path = args.goivy / name
        return Path(paths.get(str(path), path)).read_text()

    def put(name, code):
        source = target / name
        source.parent.mkdir(parents=True, exist_ok=True)
        source.write_text(code)
        paths[str(args.goivy / name)] = str(source)

    if args.count_calls:
        for index, (name, filename) in enumerate(CALLS.items()):
            code = original(filename)
            start = re.search(r"^func " + name + r"\(", code, re.M)
            if start is None:
                raise ValueError(f"missing function {name} in {filename}")
            body = code.index("{\n", start.end()) + 1
            put(filename, code[:body] + f"\nIvyFrontendCounters[{index}].Add(1)\n" + code[body:])
        put("observe_frontend.go", 'package goivy\nimport "sync/atomic"\n'
            + f"var IvyFrontendCounters [{len(CALLS)}]atomic.Uint64\n")

    name = "cmd/goivy_check/main.go"
    code = original(name)
    main_call = "os.Exit(goivy.MainWithConfig(args, cfg))"
    if code.count(main_call) != 1 or code.count("import (") != 1:
        raise ValueError("checker main function has changed")
    report = """status := goivy.MainWithConfig(args, cfg)
    samples := []metrics.Sample{""" + ",".join("{Name:" + json.dumps(k) + "}" for k in METRICS) + """}
    metrics.Read(samples)
    for _, s := range samples {
        switch s.Value.Kind() {
        case metrics.KindUint64: fmt.Fprintln(os.Stderr, "IVY-MEMORY", s.Name, s.Value.Uint64())
        case metrics.KindFloat64: fmt.Fprintln(os.Stderr, "IVY-MEMORY", s.Name, s.Value.Float64())
        default: panic("runtime metric unavailable")
        }
    }
    """
    if args.count_calls:
        report += """for i := range goivy.IvyFrontendCounters {
            fmt.Fprintln(os.Stderr, "IVY-FRONTEND-CALLS", i, goivy.IvyFrontendCounters[i].Load())
        }
        """
    imports = 'import (\n"runtime/metrics"'
    if args.ballast_mib:
        imports += '\n"runtime"\n"strconv"'
        report = """mib, err := strconv.Atoi(os.Getenv("IVY_BALLAST_MIB"))
        if err != nil || mib < 0 { panic("invalid heap ballast") }
        ballast := make([]byte, int64(mib)<<20)
        for i := 0; i < len(ballast); i += 4096 { ballast[i] = 1 }
        """ + report + "runtime.KeepAlive(ballast)\n"
    put(name, code.replace("import (", imports).replace(main_call, report + "os.Exit(status)"))
    overlay = target / "overlay.json"
    overlay.write_text(json.dumps({"Replace": paths}))
    binary = target / "checker"
    tags = "xtrace_off,classic_z3" if engine == "native" else "xtrace_off"
    flag = [f"-modfile={args.modfile.resolve(strict=True)}"] if args.modfile else []
    subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-tags=" + tags, *flag,
                    "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    return binary


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("goivy", "spec", "output", "reference"):
        p.add_argument("--" + name, type=Path, required=True)
    p.add_argument("--modfile", type=Path)
    p.add_argument("--go-overlay", type=Path)
    p.add_argument("--frontend-overlay", type=Path, help="Go source overlay applied to both backends")
    p.add_argument("--engine", choices=("g3", "native"), nargs="+", default=("g3", "native"))
    p.add_argument("--isolate", default="dramc_nb2")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--pairs", type=int, default=3)
    p.add_argument("--gogc", type=int, nargs="+", default=[100, 200, 400])
    p.add_argument("--ballast-mib", type=int, nargs="+",
                   help="retain this much additional pointer-free Go heap during the proof")
    p.add_argument("--count-calls", action="store_true", help="add counters to six shared frontend functions")
    p.add_argument("--perf", help="optional Linux perf executable")
    a = p.parse_args()
    a.goivy, a.spec, a.output = a.goivy.resolve(strict=True), a.spec.absolute(), a.output.resolve()
    reference = a.reference.read_bytes()
    if (not a.spec.is_file() or not reference.rstrip().endswith(b"\nOK") or a.pairs < 1
            or min(a.gogc) <= 0 or len(a.gogc) != len(set(a.gogc))):
        p.error("supply a successful reference, a positive pair count and distinct positive GOGC settings")
    if a.ballast_mib and (min(a.ballast_mib) < 0 or len(a.ballast_mib) != len(set(a.ballast_mib))):
        p.error("heap ballast values must be distinct nonnegative sizes in MiB")
    a.output.mkdir(parents=True)
    engines = {name: compile_checker(a, name) for name in dict.fromkeys(a.engine)}
    variants = [(gc, pad, name) for gc in a.gogc for pad in (a.ballast_mib or [0]) for name in engines]
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "cpu", "round", "gogc", *(["ballast_mib"] if a.ballast_mib else []), "engine", "wall_s", "user_s", "sys_s", "rss_kib",
                     *(EVENTS if a.perf else ()), *METRICS, *(CALLS if a.count_calls else ())])
    expected_calls = None
    for i in range(a.pairs):
        offset = i % len(variants)
        order = variants[offset:] + variants[:offset]
        for gc, pad, name in reversed(order) if i % 2 else order:
            os.environ["GOGC"] = str(gc)
            if a.ballast_mib:
                os.environ["IVY_BALLAST_MIB"] = str(pad)
            directory = a.output / f"{i+1}-{gc}-{pad}-{name}"
            status, usage, counters = measure(engines[name], a, directory)
            if status or (directory / "stdout").read_bytes() != reference:
                raise ValueError(f"{directory} failed to reproduce the reference transcript")
            metrics, calls = {}, {}
            for line in (directory / "stderr").read_text().splitlines():
                if line.startswith("IVY-MEMORY "):
                    _, key, value = line.split()
                    metrics[key] = value
                elif line.startswith("IVY-FRONTEND-CALLS "):
                    _, index, value = line.split()
                    calls[int(index)] = int(value)
            if expected_calls is None:
                expected_calls = calls
            if set(metrics) != set(METRICS) or calls != expected_calls or len(calls) != (len(CALLS) if a.count_calls else 0):
                raise ValueError(f"{directory} has missing metrics or different frontend call counts")
            writer.writerow([a.isolate, a.cpu, i+1, gc, *([pad] if a.ballast_mib else []), name, *usage, *counters,
                             *(metrics[k] for k in METRICS), *(calls[k] for k in range(len(calls)))])
            sys.stdout.flush()


if __name__ == "__main__":
    main()
