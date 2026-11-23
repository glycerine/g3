"""Observe asserted-formula rewrites during the first physical push of each GoIvy batch."""
import argparse
import gzip
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

HERE = Path(__file__).resolve().parent
G3 = HERE.parents[3]
sys.path.insert(0, str(HERE.parent))
from relevance_counts import make_value


def patched_files(args, stage):
    study = "native" if args.engine == "native" else args.mode
    diff = gzip.decompress((HERE / f"nov23-row-{study}.patch.gz").read_bytes()).decode()
    return stage_patch(diff, args.goivy, stage)


def stage_patch(diff, goivy, stage):
    sources = {}
    for before, after in re.findall(r"^--- (\S+)\n\+\+\+ b/(\S+)$", diff, re.M):
        project, relative = after.split("/", 1)
        root = {"g3": G3, "ivy": goivy.parent}[project]
        src = root / relative
        target = stage / after
        target.parent.mkdir(parents=True, exist_ok=True)
        if before != "/dev/null":
            shutil.copyfile(src, target)
        sources[src] = target
    subprocess.run(["patch", "--batch", "-p1", "-d", stage], input=diff, text=True, check=True)
    return sources


def native_library(args, stage, sources):
    build = args.cpp_build.resolve()
    member = "asserted_formulas.o"
    lib = args.goivy / "z3vendor/native_lib/libz3.a"
    if subprocess.check_output(["ar", "p", lib, member]) != (build / "smt" / member).read_bytes():
        raise ValueError("GoIvy and the in-tree C++ build have different asserted_formulas objects")
    src = G3 / "src"
    makefile = (build / "Makefile").read_text()
    config = (build / "config.mk").read_text()
    rule = re.search(r"\$\((includes_\d+)\).*\.\./src/smt/asserted_formulas\.cpp$", makefile, re.M)
    subprocess.run([*make_value(config, "CXX"), *make_value(config, "CXXFLAGS"),
                    *make_value(makefile, rule[1]), "-o", stage / member,
                    sources[src / "smt/asserted_formulas.cpp"]], cwd=build, check=True)
    archive = stage / "libz3.a"
    shutil.copyfile(lib, archive)
    subprocess.run(["ar", "rcs", archive, stage / member], check=True)
    obj = stage / "row_counter.o"
    subprocess.run(["c++", "-std=c++17", "-O2", "-fPIC", "-I", src, "-I", src / "api", "-c",
                    src / "test/perf/ivy_asserted_rows.cpp", "-o", obj], check=True)
    adapter = args.goivy / "smt/quantifier.go"
    text = adapter.read_text()
    name = "${SRCDIR}/../z3vendor/native_lib/libz3.a"
    if text.count(name) != 1:
        raise ValueError("unrecognized GoIvy native archive setting")
    sources[adapter] = stage / "quantifier.go"
    sources[adapter].write_text(text.replace(name, str(archive)))
    return ["-ldflags", f"-linkmode=external '-extldflags=-Wl,--wrap=Z3_solver_push {obj} {archive} -lstdc++ -lm -lgomp'"]


def checker(args, stage):
    sources = patched_files(args, stage)
    native = args.engine == "native"
    flags = native_library(args, stage, sources) if native else []
    return build_checker(args.goivy, stage, sources, args.abc, native, flags)


def build_checker(goivy, stage, sources, abc=None, native=False, flags=()):
    local = Path(json.loads(subprocess.check_output(
        ["go", "list", "-mod=readonly", "-m", "-json", "github.com/glycerine/g3/pkg"], cwd=goivy))["Dir"])
    if not local.samefile(G3 / "pkg"):
        raise ValueError(f"GoIvy uses {local}, expected a local replacement to {G3 / 'pkg'}")
    mapping = {}
    for src, patched in sources.items():
        if src.suffix == ".go":
            mapping[str(src)] = str(patched)
            if src.is_relative_to(G3 / "pkg"):
                mapping[str(local / src.relative_to(G3 / "pkg"))] = str(patched)
    overlay = stage / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    binary = stage / "goivy_check"
    tags = "xtrace_off,classic_z3" if native else "xtrace_off"
    subprocess.run(["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-tags=" + tags,
                    "-overlay=" + str(overlay), *flags, "-o", binary, "./cmd/goivy_check"],
                   cwd=goivy, check=True)
    if abc:
        (stage / "abc").symlink_to(abc.resolve(strict=True))
    return binary


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("goivy", "spec", "stdout", "stderr", "output"):
        p.add_argument("--" + name, type=Path, required=True)
    p.add_argument("--cpp-build", type=Path, default=G3 / "build")
    p.add_argument("--abc", type=Path)
    p.add_argument("--engine", choices=("g3", "native"), required=True)
    p.add_argument("--mode", choices=("rows", "instructions"), required=True)
    p.add_argument("--isolate")
    p.add_argument("--cpu", type=int, required=True)
    p.add_argument("--pairs", type=int, default=2)
    p.add_argument("--expected-exit", type=int, required=True)
    args = p.parse_args()
    if args.pairs <= 0:
        p.error("use a positive number of runs")
    args.goivy = args.goivy.resolve(strict=True)
    output = args.output.resolve()
    output.mkdir(parents=True)
    binary = checker(args, output)
    stdout, stderr = args.stdout.read_bytes(), args.stderr.read_bytes()
    env = dict(os.environ, XTRACE_OFF="1", GOMAXPROCS="1", GOGC="100")
    if args.mode == "instructions":
        env["SCOPE_THREAD_EVENT"] = "1"
    else:
        env.pop("SCOPE_THREAD_EVENT", None)
    command = ["taskset", "-c", str(args.cpu), binary,
               *(["isolate=" + args.isolate] if args.isolate else []), args.spec.absolute()]
    for run in range(1, args.pairs + 1):
        path = output / str(run)
        path.mkdir()
        with (path / "stdout").open("wb") as out, (path / "stderr").open("wb") as err:
            process = subprocess.run(command, cwd=path, stdout=out, stderr=err, env=env)
        captured = (path / "stderr").read_bytes()
        other = b"".join(x for x in captured.splitlines(keepends=True)
                         if not x.startswith((b"CENSUS-", b"IVY-FIRST-", b"ROW-COST ")))
        if process.returncode != args.expected_exit or (path / "stdout").read_bytes() != stdout or other != stderr:
            raise ValueError(f"verification changed in {path}")
        with gzip.open(path / "observations.gz", "wb") as compressed:
            compressed.write(captured)


if __name__ == "__main__":
    main()
