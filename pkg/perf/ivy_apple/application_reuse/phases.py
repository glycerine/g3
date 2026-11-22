"""Count canonical table stages for a duplicate application, bypassing Go's optional hint."""

import argparse
from concurrent.futures import ThreadPoolExecutor
import csv
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from instructions import FIXTURE, NATIVE, PKG, THREAD
from native_benchmark import ROOT, make_variable
from phase_counters import replace_once

PHASES = ("empty", "dispatch", "place", "prepare", "lookup", "discard")
ARITIES = (1, 2, 3, 4)
GO_BEGIN = "scopePhaseEvent(1, true)\n"
GO_END = "scopePhaseEvent(1, false)\n"
CPP_BEGIN = "app_phase(true);\n"
CPP_END = "app_phase(false);\n"
FACTORIES = PKG / "ast/ast_core_factories.go"
SLICE = PKG / "ast/ast_app_slice.go"
REGISTER = PKG / "ast/ast_core_register.go"


def instrument_go(phase, files):
    factories = files[FACTORIES]
    hinted = "if known != nil && m.table.hintAttempts != 0"
    if hinted in factories:
        factories = replace_once(factories, hinted, "if false && known != nil && m.table.hintAttempts != 0")
        files[FACTORIES] = factories
    allocate = "r, e := m.allocateNode(ASTApp)"
    fast = files[SLICE] is not None
    if fast:
        table, lookup = SLICE, "existing, found, index, err := m.tableLookup(r, mixed)"
    else:
        table, lookup = REGISTER, "result, unique, e := m.tableInsert(r)"

    if phase == "empty":
        files[FACTORIES] = replace_once(factories, allocate, GO_BEGIN + GO_END + allocate)
    elif phase == "dispatch":
        public = "func (m *ASTManager) MkApp(decl ASTHandle, args []ASTHandle) (ASTHandle, error) {"
        factories = replace_once(factories, public, public + "\n" + GO_BEGIN)
        files[FACTORIES] = replace_once(factories, allocate, GO_END + allocate)
    elif phase == "place":
        files[FACTORIES] = replace_once(factories, allocate, GO_BEGIN + allocate + "\n" + GO_END)
    elif phase == "prepare":
        files[FACTORIES] = replace_once(factories, allocate, allocate + "\n" + GO_BEGIN)
        files[table] = replace_once(files[table], lookup, GO_END + lookup)
    elif phase == "lookup":
        files[table] = replace_once(files[table], lookup, GO_BEGIN + lookup + "\n" + GO_END)
    elif phase == "discard" and fast:
        first = "if found {"
        last = "return existing, true, nil"
        body = replace_once(files[SLICE], first, first + "\n" + GO_BEGIN)
        files[SLICE] = replace_once(body, last, GO_END + last)
    elif phase == "discard":
        first = "if !unique {\n\t\tm.freeNodeStorage(r)\n"
        files[REGISTER] = replace_once(files[REGISTER], first, "if !unique {\n" + GO_BEGIN +
                                       "m.freeNodeStorage(r)\n" + GO_END)


def go_binary(directory, phase, overlay):
    directory.mkdir()
    paths = json.loads(overlay.read_text())["Replace"] if overlay else {}

    def read(path):
        original = paths.get(str(path), str(path))
        if original and Path(original).is_file():
            return Path(original).read_text()
        return None

    def write(path, content):
        file = directory / path.name
        file.write_text(content)
        paths[str(path)] = str(file)

    sources = {path: read(path) for path in (FACTORIES, SLICE, REGISTER)}
    originals = sources.copy()
    instrument_go(phase, sources)
    for path, source in sources.items():
        if source != originals[path]:
            write(path, source)
    fixture = read(FIXTURE)
    marker = "b.ResetTimer()"
    begin = fixture.index(marker, fixture.index("func BenchmarkASTCanonicalApplication(")) + len(marker)
    end = fixture.index("\n\t\t})", begin)
    setup = "\nstartScopeCounter()\nscopePhaseVisits = 0\nscopePhaseActive = true\n"
    report = '\nscopePhaseActive = false\nb.ReportMetric(float64(scopePhaseVisits), "intervals")\nfinishScopeCounter()\n'
    write(FIXTURE, fixture[:begin] + setup + fixture[begin:end] + report + fixture[end:])
    write(FIXTURE.with_name("app_phase_counter.go"), THREAD.read_text().replace("package smtcore", "package ast"))
    final = directory / "overlay.json"
    final.write_text(json.dumps({"Replace": paths}))
    subprocess.run(["go", "test", "-p", "3", "-pgo=off", "-c", "-overlay=" + str(final),
                    "-o", directory / "bench", "./ast"], cwd=PKG, check=True)


def instrument_native(phase, source):
    allocate = "void * mem = allocate_node(sz);"
    start = source.index("app * ast_manager::mk_app_core(func_decl * decl, unsigned num_args")
    left = source.index(allocate, start)
    right = left + len(allocate)
    lookup = "ast * r = m_ast_table.insert_if_not_there(n);"
    if phase == "empty":
        return source[:left] + CPP_BEGIN + CPP_END + source[left:]
    if phase == "dispatch":
        public = "app * ast_manager::mk_app(func_decl * decl, unsigned num_args, expr * const * args) {"
        return replace_once(source[:left] + CPP_END + source[left:], public, public + "\n" + CPP_BEGIN)
    if phase == "place":
        return source[:left] + CPP_BEGIN + allocate + "\n" + CPP_END + source[right:]
    if phase == "prepare":
        return replace_once(source[:right] + "\n" + CPP_BEGIN + source[right:], lookup, CPP_END + lookup)
    if phase == "lookup":
        return replace_once(source, lookup, CPP_BEGIN + lookup + "\n" + CPP_END)
    discard = "deallocate_node(n, ::get_node_size(n));"
    return replace_once(source, discard + "\n        return r;", CPP_BEGIN + discard + "\n" + CPP_END + "return r;")


def native_binary(directory, phase, build):
    directory.mkdir()
    path = directory / "ast.cpp"
    code = instrument_native(phase, (ROOT / "src/ast/ast.cpp").read_text())
    path.write_text("extern void app_phase(bool);\n" + code)
    text = (NATIVE / "ast_application_reuse.cpp").read_text()
    text = replace_once(text, "constexpr unsigned iterations = 200000;", "unsigned iterations;")
    begin = text.index("auto start = clock_type::now();")
    text = text[:begin] + "app_visits=0; open_counter(); app_active=true;\n" + text[begin:]
    end = 'print_result("single", arity, 1, start);'
    report = 'app_active=false; std::printf("APP-PHASE %u %u\\n", arity, app_visits); report_counter();\n'
    text = replace_once(text, end, report + end)
    initialize = "memory::initialize(0);"
    text = replace_once(text, initialize, initialize + ' iterations=std::stoul(std::getenv("APP_ITERATIONS"));')
    text = replace_once(text, "for (unsigned n : {0u, 192u, 256u})\n            run(arity, n, false);", "")
    fixture = directory / "fixture.cpp"
    fixture.write_text(f'#include "{NATIVE / "thread_counter.h"}"\n' + """
static bool app_active;
static unsigned app_visits;
void app_phase(bool begin) {
    if (!app_active) return;
    if (begin) ++app_visits;
    signal_counter(begin);
}
""" + text)
    config = (build / "config.mk").read_text()
    makefile = (build / "Makefile").read_text()
    cxx = make_variable(config, "CXX")
    for source, unit in [(path, "ast/ast"), (fixture, "test/smt_scope_restoration")]:
        include = re.search(r"\$\((includes_\d+)\).*\.\./src/" + unit + r"\.cpp", makefile)[1]
        subprocess.run([*cxx, *make_variable(config, "CXXFLAGS"), *make_variable(makefile, include),
                        "-o", source.with_suffix(".o"), source], cwd=build, check=True)
    subprocess.run([*cxx, "-o", directory / "bench", fixture.with_suffix(".o"), path.with_suffix(".o"),
                    build / "libz3.a", *make_variable(config, "LINK_EXTRA_FLAGS")], cwd=build, check=True)


def measure(binary, native, cpu, iterations):
    command = ["taskset", "-c", str(cpu), binary]
    if not native:
        command += ["-test.run", "^$", "-test.bench", "^BenchmarkASTCanonicalApplication$",
                    f"-test.benchtime={iterations}x"]
    env = dict(os.environ, GOMAXPROCS="1", SCOPE_PHASE="1", SCOPE_THREAD_EVENT="1", APP_ITERATIONS=str(iterations))
    output = subprocess.check_output(command, text=True, env=env)
    if native:
        rows = re.findall(r"APP-PHASE (\d+) (\d+)\nSCOPE-THREAD (\d+) (\d+) (\d+)", output)
    else:
        # Only the measured iteration count is printed with the benchmark name.
        readings = re.findall(r"BenchmarkASTCanonicalApplication/(\d+)(?:-\d+)?\s+SCOPE-THREAD (\d+) (\d+) (\d+)"
                              r"\n\s+(\d+)\s+[^\n]*?\s+(\d+) intervals", output)
        rows = [(arity, visits, count, enabled, running) for arity, count, enabled, running, calls, visits in readings
                if int(calls) == iterations]
    rows = [tuple(map(int, row)) for row in rows]
    if [r[0] for r in rows] != list(ARITIES) or any(visits != iterations or enabled != running or not running
                                                       for _, visits, _, enabled, running in rows):
        raise ValueError(f"incomplete or multiplexed measurements from {binary}\n{output}")
    return rows


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--go", nargs=2, action="append", metavar=("NAME", "OVERLAY"), help="use '-' for this checkout")
    p.add_argument("--dir", required=True, type=Path)
    p.add_argument("--build", default=ROOT / "build", type=Path)
    p.add_argument("--rounds", default=8, type=int)
    p.add_argument("--iterations", default=50_000, type=int)
    p.add_argument("--cpu", required=True, type=int)
    p.add_argument("--reuse", action="store_true", help="use previously built binaries in --dir with the same labels")
    args = p.parse_args()
    variants = args.go or [("g3", "-")]
    names = [name for name, _ in variants] + ["native"]
    if (len(set(names)) != len(names) or args.iterations <= 1 or args.rounds < 1 or
            any(not re.fullmatch(r"[\w-]+", name) for name in names)):
        p.error("supply unique engine labels, iterations greater than one and positive rounds")
    directory = args.dir.resolve()
    if not args.reuse:
        directory.mkdir(parents=True)
        with ThreadPoolExecutor(3) as pool:
            jobs = []
            for phase in PHASES:
                for name, overlay in variants:
                    source = None if overlay == "-" else Path(overlay).resolve(strict=True)
                    jobs.append(pool.submit(go_binary, directory / f"{phase}-{name}", phase, source))
                jobs.append(pool.submit(native_binary, directory / f"{phase}-native", phase, args.build.resolve(strict=True)))
            for job in jobs:
                job.result()
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["round", "cpu", "engine", "phase", "arity", "intervals", "instructions", "enabled_ns", "running_ns"])
    for i in range(args.rounds):
        engines = names[i % len(names):] + names[:i % len(names)]
        phases = PHASES[i % len(PHASES):] + PHASES[:i % len(PHASES)]
        for engine in engines[::-1] if i % 2 else engines:
            for phase in phases:
                binary = directory / f"{phase}-{engine}" / "bench"
                for row in measure(binary, engine == "native", args.cpu, args.iterations):
                    writer.writerow([i + 1, args.cpu, engine, phase, *row])
                sys.stdout.flush()


if __name__ == "__main__":
    main()
