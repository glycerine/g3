#!/usr/bin/env python3
"""Compare GoIvy's first reported cycle with the historical implementation."""

import argparse
import json
import subprocess
import tempfile
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goivy", required=True, type=Path)
    parser.add_argument("--graphs", type=int, default=30000)
    parser.add_argument("--seed", type=int, default=7309)
    args = parser.parse_args()
    if args.graphs < 1:
        parser.error("--graphs must be positive")
    module = args.goivy.resolve()
    old = subprocess.check_output(
        ["git", "show", "01aa2c6fa:goivy/fragment.go"], cwd=module, text=True
    )
    start = old.index("type arcTupleKey = string")
    stop = old.index("// --- Module-level API ---", start)
    reference = old[start:stop].replace(
        "func (c *checker) findCycle()", "func (c *checker) legacyCycle()"
    )
    test = r'''
func TestArcWalkMatchesFirstCycle(t *testing.T) {
    r := rand.New(rand.NewSource(SEED))
    for iter := 0; iter < GRAPHS; iter++ {
        var c checker
        n := 1 + r.Intn(24)
        nodes := make([]*UFNode, n)
        for i := range nodes { nodes[i] = NewUFNode() }
        for i := 0; i < n/5; i++ {
            UFUnify(nodes[r.Intn(n)], nodes[r.Intn(n)])
        }
        for i, count := 0, r.Intn(100); i < count; i++ {
            a := arc{
                from: nodes[r.Intn(n)], to: nodes[r.Intn(n)], fmla: True,
                loc: Location{Filename: "test.ivy", Line: r.Intn(19)},
                argIdx: r.Intn(4), hasIdx: r.Intn(2) != 0,
            }
            if r.Intn(2) != 0 { a.fmla = False }
            c.arcs = append(c.arcs, a)
            if r.Intn(4) == 0 { c.arcs = append(c.arcs, a) }
        }
        got, want := c.findCycle(), c.legacyCycle()
        if !slices.Equal(got, want) { t.Fatalf("graph %d: %v vs %v", iter, got, want) }
    }
}
'''
    with tempfile.TemporaryDirectory() as tmp:
        directory = Path(tmp)
        source = directory / "cycle_random_check_test.go"
        source.write_text(
            'package goivy\nimport ("fmt"; "math/rand"; "slices"; "testing")\n'
            + reference
            + test.replace("SEED", str(args.seed)).replace("GRAPHS", str(args.graphs))
        )
        overlay = directory / "overlay.json"
        overlay.write_text(json.dumps({"Replace": {str(module / source.name): str(source)}}))
        subprocess.run(
            ["go", "test", "-tags=xtrace_off", "-count=1", "-overlay=" + str(overlay),
             "-run", "^TestArcWalkMatchesFirstCycle$", "."],
            cwd=module, check=True,
        )


if __name__ == "__main__":
    main()
