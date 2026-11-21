"""Count retired instructions on the benchmark thread for the Go and native AST application loops."""

import argparse
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from compare import ITERATIONS, ROOT, build_native

PKG = ROOT / "pkg"
THREAD = PKG / "perf/scope_restoration/pop_phases/thread.go.txt"
FIXTURE = PKG / "ast/ast_app_benchmark_test.go"
NATIVE = ROOT / "src/test/perf"
EXPECTED = {("single", arity, 1) for arity in range(1, 5)} | {
    ("rotating", arity, resident) for arity in range(1, 5) for resident in (0, 192, 256)}
COUNTERS = re.compile(r"APP-COUNTER (single|rotating) (\d+) (\d+) (\d+)\s+SCOPE-THREAD (\d+) (\d+) (\d+)")


def go_fixture(source):
    cases = (("BenchmarkASTCanonicalApplication", "single", "1"),
             ("BenchmarkASTApplicationReuse", "rotating", "resident"))
    for function, workload, resident in cases:
        begin = source.index("func " + function)
        start = source.index("b.ResetTimer()", begin) + len("b.ResetTimer()")
        end = source.index("\n" + ("\t\t" if workload == "single" else "\t\t\t") + "})", start)
        finish = f'\nendAppInterval("{workload}", arity, {resident}, b.N)'
        source = source[:start] + "\nstartAppInterval()" + source[start:end] + finish + source[end:]
    return source


def go_binary(directory, overlay):
    directory.mkdir()
    paths = json.loads(overlay.read_text())["Replace"] if overlay else {}
    original = Path(paths.get(str(FIXTURE), FIXTURE))
    fixture = directory / FIXTURE.name
    fixture.write_text(go_fixture(original.read_text()))
    counter = directory / "app_counter.go"
    counter.write_text(THREAD.read_text().replace("package smtcore", "package ast") + '''
var appEmptyInterval = os.Getenv("APP_EMPTY_INTERVAL") == "1"
func startAppInterval() {
    startScopeCounter()
    scopePhaseActive = true
    scopePhaseEvent(1, true)
    if appEmptyInterval { scopePhaseEvent(1, false) }
}
func endAppInterval(workload string, arity, resident, iterations int) {
    if !appEmptyInterval { scopePhaseEvent(1, false) }
    scopePhaseActive = false
    fmt.Printf("APP-COUNTER %s %d %d %d\\n", workload, arity, resident, iterations)
    finishScopeCounter()
}
''')
    paths.update({str(FIXTURE): str(fixture), str(FIXTURE.with_name("app_counter.go")): str(counter)})
    final = directory / "overlay.json"
    final.write_text(json.dumps({"Replace": paths}))
    binary = directory / "bench"
    subprocess.run(["go", "test", "-pgo=off", "-c", "-overlay=" + str(final), "-o", binary, "./ast"],
                   cwd=PKG, check=True)
    return binary


def native_binary(directory, build):
    directory.mkdir()
    source = (NATIVE / "ast_application_reuse.cpp").read_text()
    marker = "auto start = clock_type::now();"
    if source.count(marker) != 2:
        raise ValueError("native fixture loop boundaries changed")
    source = source.replace(marker, marker + "\n        begin_app_count();")
    for workload, resident in [("single", "1"), ("rotating", "retained")]:
        marker = f'print_result("{workload}", arity, {resident}, start);'
        if source.count(marker) != 1:
            raise ValueError("native fixture end changed")
        source = source.replace(marker, f'end_app_count("{workload}", arity, {resident}, iterations);\n    ' + marker)
    header = (NATIVE / "thread_counter.h").as_posix()
    prelude = f'#include "{header}"\n' + '''
static bool app_empty;
static void begin_app_count() {
    app_empty = std::getenv("APP_EMPTY_INTERVAL")[0] == '1';
    open_counter();
    signal_counter(true);
    if (app_empty) signal_counter(false);
}
static void end_app_count(char const* workload, unsigned arity, unsigned resident, unsigned iterations) {
    if (!app_empty) signal_counter(false);
    std::printf("APP-COUNTER %s %u %u %u\\n", workload, arity, resident, iterations);
    report_counter();
}
'''
    path = directory / "fixture.cpp"
    path.write_text(prelude + source)
    binary = directory / "bench"
    build_native(build, binary, path)
    return binary


def measure(binary, native, cpu, empty):
    cmd = ["taskset", "-c", str(cpu), binary]
    if not native:
        cmd += ["-test.run", "^$", "-test.bench", "^BenchmarkAST(CanonicalApplication|ApplicationReuse)$",
                f"-test.benchtime={ITERATIONS}x"]
    env = dict(os.environ, GOMAXPROCS="1", SCOPE_PHASE="1", SCOPE_THREAD_EVENT="1",
               APP_EMPTY_INTERVAL="1" if empty else "0")
    log = subprocess.check_output(cmd, text=True, env=env)
    rows = {}
    for workload, arity, resident, n, instructions, enabled, running in COUNTERS.findall(log):
        if int(n) != ITERATIONS:  # Go's initial one-iteration calibration
            continue
        key = workload, int(arity), int(resident)
        if int(enabled) <= 0 or enabled != running or key in rows:
            raise ValueError(f"invalid interval from {binary}: {key}, {enabled}, {running}")
        rows[key] = int(instructions), int(enabled), int(running)
    if rows.keys() != EXPECTED:
        raise ValueError(f"{binary} missing {EXPECTED - rows.keys()}")
    return rows


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", nargs=2, action="append", metavar=("NAME", "OVERLAY"),
                   help="use '-' for the current working tree")
    p.add_argument("--dir", type=Path, required=True, help="fresh directory for the instrumented builds")
    p.add_argument("--build", type=Path, default=ROOT / "build")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--rounds", type=int, default=9)
    args = p.parse_args()
    out = args.dir.resolve()
    variants = args.go or [("g3", "-")]
    names = [name for name, _ in variants] + ["native"]
    if args.rounds < 1 or len(names) != len(set(names)) or any(not re.fullmatch(r"[\w-]+", n) for n in names):
        p.error("supply positive rounds and unique build labels")
    out.mkdir(parents=True)
    binaries = {name: go_binary(out / name, None if path == "-" else Path(path).resolve(strict=True))
                for name, path in variants}
    binaries["native"] = native_binary(out / "native", args.build.resolve(strict=True))
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "cpu", "engine", "interval", "workload", "arity", "resident", "iterations",
                     "instructions", "enabled_ns", "running_ns"])
    for i in range(args.rounds):
        offset = i % len(names)
        order = names[offset:] + names[:offset]
        for name in order[::-1] if i % 2 else order:
            for empty in ([False, True] if i % 2 else [True, False]):
                for key, counts in measure(binaries[name], name == "native", args.cpu, empty).items():
                    writer.writerow([i+1, args.cpu, name, "empty" if empty else "loop", *key, ITERATIONS, *counts])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
