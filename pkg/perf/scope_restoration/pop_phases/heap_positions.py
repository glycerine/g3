"""Count the heap positions removed by the existing scoped-pop benchmark."""

import argparse
import json
import os
from pathlib import Path
import subprocess

from measure import PKG, WARMUP, replace_once


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dir", type=Path, required=True, help="new directory for the benchmark overlay")
    parser.add_argument("--iterations", type=int, default=1008)
    args = parser.parse_args()
    if args.iterations <= WARMUP:
        parser.error("supply more than eight iterations")
    output = args.dir.resolve()
    output.mkdir(parents=True)
    heap = PKG / "util/heap/heap.go"
    fixture = PKG / "smtcore/smt_hotpath_benchmark_test.go"
    anchor = "last := h.values[len(h.values)-1]\n\th.indices[v] = 0"
    h = replace_once(heap.read_text(), anchor, """if MeasureErasePositions {
        switch {
        case int(i) == len(h.values)-1: ErasePositions[0]++
        case int64(i)*2 >= int64(len(h.values)-1): ErasePositions[1]++
        default: ErasePositions[2]++
        }
    }
    """ + anchor)
    h += "\nvar MeasureErasePositions bool\nvar ErasePositions [3]int\n"
    f = replace_once(fixture.read_text(), "import (", 'import (\n measureheap "github.com/glycerine/g3/pkg/util/heap"')
    f = replace_once(f, "start := time.Now()", f"measureheap.MeasureErasePositions = i >= {WARMUP}\nstart := time.Now()")
    f = replace_once(f, "elapsed += time.Since(start)", "elapsed += time.Since(start)\nmeasureheap.MeasureErasePositions = false")
    f = replace_once(f, "b.ReportMetric(float64(elapsed.Nanoseconds())", f"""
    if b.N > {WARMUP} {{
        counts := measureheap.ErasePositions
        if counts[0]+counts[1]+counts[2] != 128*(b.N-{WARMUP}) {{
            b.Fatalf("unexpected removals per scoped pop: %v", counts)
        }}
        for i, n := range counts {{
            b.ReportMetric(float64(n)/float64(b.N-{WARMUP}), []string{{"last/pop","other-leaf/pop","interior/pop"}}[i])
        }}
    }}
    measureheap.ErasePositions = [3]int{{}}
    b.ReportMetric(float64(elapsed.Nanoseconds())""")
    paths = {}
    for path, code in [(heap, h), (fixture, f)]:
        changed = output / path.name
        changed.write_text(code)
        paths[str(path)] = str(changed)
    overlay = output / "overlay.json"
    overlay.write_text(json.dumps({"Replace": paths}))
    subprocess.run(["go", "test", "-overlay=" + str(overlay), "./smtcore", "-run", "^$",
                    "-bench", "^BenchmarkSMTScopeRestorationWithQueue$/^ScopedAtoms$",
                    f"-benchtime={args.iterations}x", "-count=1"], cwd=PKG,
                   env=dict(os.environ, GOMAXPROCS="1"), check=True)


if __name__ == "__main__":
    main()
