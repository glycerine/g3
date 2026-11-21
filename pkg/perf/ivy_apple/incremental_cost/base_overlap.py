"""Count shared expressions at the first external Push of each Ivy solver or reset."""
import argparse
import csv
import json
import os
from pathlib import Path
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from phase_counters import G3_METHOD, replace_once


# The diagnostic runs a single checker pinned to one thread. The global
# ordinal aligns with the independent hardware counter's chronological trace.
# Store handle values only. Taking arena references would change their lifetime.
SOLVER_FIELDS = """
    observedRoots []ast.ASTHandle
    observedNodes map[ast.ASTHandle]bool
    observedDepth, observedChecks, observedResets, observedID int
    sampledBase bool
"""
CONTEXT_FIELDS = """
    observedSolvers int
    observedRoots map[ast.ASTHandle]int
    observedNodes map[ast.ASTHandle]bool
"""
SAMPLE = """
var observedPushOrdinal int

func (s *Z3Solver) observeBase() {
    if s.observedChecks != 0 { return }
    observedPushOrdinal++
    if s.observedDepth != 0 || s.sampledBase { return }
    s.sampledBase = true
    ctx := s.ctx
    if ctx.observedNodes == nil {
        ctx.observedNodes = make(map[ast.ASTHandle]bool)
        ctx.observedRoots = make(map[ast.ASTHandle]int)
    }
    if s.observedNodes == nil {
        ctx.observedSolvers++
        s.observedID = ctx.observedSolvers
        s.observedNodes = make(map[ast.ASTHandle]bool)
    }
    m, err := ctx.c.Manager()
    fail(err)
    roots := make([]string, len(s.observedRoots))
    origin := uint64(0)
    for i, root := range s.observedRoots {
        origin = root.Origin()
        if ctx.observedRoots[root] == 0 {
            ctx.observedRoots[root] = len(ctx.observedRoots) + 1
        }
        roots[i] = strconv.Itoa(ctx.observedRoots[root])
    }
    pending := append([]ast.ASTHandle(nil), s.observedRoots...)
    visited := make(map[ast.ASTHandle]bool)
    solverSeen, contextSeen := 0, 0
    for len(pending) > 0 {
        h := pending[len(pending)-1]
        pending = pending[:len(pending)-1]
        if visited[h] { continue }
        visited[h] = true
        if s.observedNodes[h] { solverSeen++ }
        if ctx.observedNodes[h] { contextSeen++ }
        info, err := m.NodeHeader(h)
        fail(err)
        switch info.Kind {
        case ast.ASTQuantifier:
            body, err := m.GetQuantifierBody(h)
            fail(err)
            pending = append(pending, body)
        case ast.ASTApp:
            for j := uint32(0); j < info.NumArgs; j++ {
                child, err := m.GetArg(h, j)
                fail(err)
                pending = append(pending, child)
            }
        }
    }
    for h := range visited {
        s.observedNodes[h] = true
        ctx.observedNodes[h] = true
    }
    fmt.Fprintf(os.Stderr, "IVY-BASE %d %d %d %d %d %d %d %d %q\\n",
        observedPushOrdinal, origin, s.observedID, s.observedResets, len(roots),
        len(visited), solverSeen, contextSeen, strings.Join(roots, "."))
}
"""


def build(args, out):
    smt = args.goivy.resolve() / "smt"
    adapter = smt / "g3_solver.go"
    context = smt / "g3.go"
    code = adapter.read_text()
    missing = [f'"{m}"' for m in ("fmt", "os", "strings", "strconv") if f'"{m}"' not in code]
    if missing:
        code = replace_once(code, "import (", "import (\n" + "\n".join(missing))
    code = replace_once(code, "type Z3Solver struct {", "type Z3Solver struct {\n" + SOLVER_FIELDS)
    code = replace_once(code, G3_METHOD, G3_METHOD + "\n s.observedChecks++; defer func(){s.observedChecks--}()")
    code = replace_once(code, "fail(s.c.Assert(e.c))",
                        "fail(s.c.Assert(e.c)); if s.observedChecks==0 && s.observedDepth==0 {s.observedRoots=append(s.observedRoots,e.c)}")
    code = replace_once(code, "fail(s.c.Push())",
                        "s.observeBase(); fail(s.c.Push()); if s.observedChecks==0 {s.observedDepth++}")
    code = replace_once(code, "fail(s.c.Pop(1))", "fail(s.c.Pop(1)); if s.observedChecks==0 {s.observedDepth--}")
    reset = "func (s *Z3Solver) Reset() {"
    code = replace_once(code, reset, reset + "\n s.observedRoots=nil; s.sampledBase=false; s.observedDepth=0; s.observedResets++")
    ctx = replace_once(context.read_text(), "type Z3Context struct {", "type Z3Context struct {\n" + CONTEXT_FIELDS)
    mapping = {}
    for source, text in [(adapter, code + SAMPLE), (context, ctx)]:
        path = out / source.name
        path.write_text(text)
        mapping[str(source)] = str(path)
    overlay = out / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}))
    build = ["go", "build", "-mod=readonly", "-pgo=off", "-buildvcs=false", "-tags=xtrace_off", "-overlay=" + str(overlay)]
    if args.modfile:
        build.append("-modfile=" + str(args.modfile.resolve(strict=True)))
    binary = out / "checker"
    subprocess.run([*build, "-o", binary, "./cmd/goivy_check"], cwd=args.goivy, check=True)
    return binary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for arg in ("goivy", "spec", "reference", "output"):
        parser.add_argument("--" + arg, type=Path, required=True)
    parser.add_argument("--modfile", type=Path)
    parser.add_argument("--cpu", type=int, required=True)
    parser.add_argument("--isolate", required=True)
    args = parser.parse_args()
    out = args.output.resolve()
    out.mkdir(parents=True)
    binary = build(args, out)
    with (out / "stderr").open("wb") as err:
        result = subprocess.check_output(["taskset", "-c", str(args.cpu), binary, "isolate=" + args.isolate, args.spec.absolute()],
                                         cwd=out, stderr=err, env=dict(os.environ, GOMAXPROCS="1", XTRACE_OFF="1"))
    if result != args.reference.read_bytes() or not result.endswith(b"OK\n"):
        raise ValueError("incomplete or changed proof transcript")
    rows = [line.split() for line in (out / "stderr").read_text().splitlines() if line.startswith("IVY-BASE ")]
    if not rows:
        raise ValueError("no first pushes")
    writer = csv.writer(sys.stdout, lineterminator="\n")
    writer.writerow(["isolate", "push", "origin", "solver", "resets", "roots", "distinct_nodes", "seen_in_solver",
                     "seen_in_context", "ordered_root_ids"])
    for _, *values, ids in rows:
        writer.writerow([args.isolate, *values, json.loads(ids)])


if __name__ == "__main__":
    main()
