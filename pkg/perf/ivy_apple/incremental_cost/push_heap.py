"""Measure allocations during each external Push and optionally capture two heap profiles around one call."""
import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from phase_counters import replace_once


METHOD = """
var heapPushOrdinal int

func heapSnapshot(name string) {
    // The runtime's allocation profile can be up to two collections old.
    runtime.GC()
    runtime.GC()
    file, err := os.Create(name)
    if err != nil { panic(err) }
    if err := pprof.WriteHeapProfile(file); err != nil { panic(err) }
    if err := file.Close(); err != nil { panic(err) }
}

func (s *Z3Solver) Push() {
    heapPushOrdinal++
    wanted, _ := strconv.Atoi(os.Getenv("IVY_PUSH_HEAP"))
    if wanted == heapPushOrdinal { heapSnapshot("before.pprof") }
    var before, after runtime.MemStats
    runtime.ReadMemStats(&before)
    fail(s.c.Push())
    runtime.ReadMemStats(&after)
    fmt.Fprintln(os.Stderr, "PUSH-MEM", heapPushOrdinal, after.TotalAlloc-before.TotalAlloc,
        after.Mallocs-before.Mallocs, after.NumGC-before.NumGC, after.HeapAlloc, before.HeapAlloc)
    if wanted == heapPushOrdinal { heapSnapshot("after.pprof") }
}
"""


def main():
    p = argparse.ArgumentParser(description=__doc__)
    for name in ("goivy", "spec", "reference", "output"):
        p.add_argument("--" + name, type=Path, required=True)
    p.add_argument("--isolate", required=True)
    p.add_argument("--cpu", required=True, type=int)
    p.add_argument("--push", type=int, help="one-based ordinal for differential allocation profiles")
    a = p.parse_args()
    if a.push is not None and a.push < 1:
        p.error("the external Push ordinal starts at 1")
    dest = a.output.resolve()
    dest.mkdir(parents=True)
    ivy = a.goivy.resolve()
    src = ivy / "smt/g3_solver.go"
    patched = dest / src.name
    code = src.read_text()
    imports = [name for name in ("runtime", "runtime/pprof", "os", "fmt", "strconv") if f'"{name}"' not in code]
    code = replace_once(code, "package smt", "package smt\nimport (\n" +
                        "\n".join(f'"{name}"' for name in imports) + "\n)")
    patched.write_text(replace_once(code, "func (s *Z3Solver) Push() { fail(s.c.Push()) }", METHOD))
    overlay = dest / "overlay.json"
    overlay.write_text(json.dumps({"Replace": {str(src): str(patched)}}))
    binary = dest / "checker"
    subprocess.run(["go", "build", "-mod=readonly", "-buildvcs=false", "-pgo=off", "-tags=xtrace_off",
                    "-overlay=" + str(overlay), "-o", binary, "./cmd/goivy_check"], cwd=ivy, check=True)
    environment = dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1", IVY_PUSH_HEAP=str(a.push or 0))
    if a.push:
        environment["GODEBUG"] = ",".join(filter(None, [environment.get("GODEBUG"), "memprofilerate=32768"]))
    with (dest / "stdout").open("wb") as out, (dest / "stderr").open("wb") as err:
        subprocess.run(["taskset", "-c", str(a.cpu), binary, "isolate=" + a.isolate, a.spec.absolute()],
                       cwd=dest, stdout=out, stderr=err, env=environment, check=True)
    expected = a.reference.read_bytes()
    if not expected.endswith(b"OK\n") or (dest / "stdout").read_bytes() != expected:
        raise ValueError("incomplete or differing verification transcript")
    rows = [line.split()[1:] for line in (dest / "stderr").read_text().splitlines()
            if line.startswith("PUSH-MEM ")]
    if not rows or [int(row[0]) for row in rows] != list(range(1, len(rows) + 1)):
        raise ValueError("missing Push measurements")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow("push allocated_bytes objects collections after_heap before_heap".split())
    writer.writerows(rows)
    if a.push:
        for metric in ("alloc_space", "alloc_objects"):
            with (dest / (metric + ".txt")).open("w") as out:
                subprocess.run(["go", "tool", "pprof", "-base", dest / "before.pprof", "-sample_index=" + metric,
                                "-top", "-nodecount=35", binary, dest / "after.pprof"], stdout=out, check=True)


if __name__ == "__main__":
    main()
