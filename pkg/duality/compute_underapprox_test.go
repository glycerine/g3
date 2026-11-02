package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestComputeUnderapproxRecursiveModelAndAllocationFailures(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	must(graph.BindSolver(native))
	must(graph.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	p, err := ctx.BoolConst("lower-parent")
	must(err)
	defer p.Close()
	q, err := ctx.BoolConst("lower-child")
	must(err)
	defer q.Close()
	qd, err := q.Decl()
	must(err)
	defer qd.Close()
	parent, err := graph.CreateNode(p)
	must(err)
	child, err := graph.CreateNode(q)
	must(err)
	must(parent.Bound.SetEmpty())
	transformer, err := NewTransformer(ctx, []*FuncDecl{qd}, nil, q)
	must(err)
	defer transformer.Close()
	edge, err := graph.CreateEdge(parent, transformer, []*Node{child})
	must(err)
	leaf, err := graph.CreateLowerBoundEdge(child)
	must(err)
	must(graph.Push())
	defer graph.Pop(1)
	must(graph.AssertNode(parent))
	must(graph.AssertEdge(edge, 0, false, false))
	must(graph.AssertEdge(leaf, 0, false, false))
	status, err := graph.Check(parent, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	value, err := graph.ComputeUnderapprox(parent, simplifier)
	must(err)
	must(value.Close())
	full, err := child.Underapprox.IsFull()
	must(err)
	if !full {
		t.Fatal("active child lower fact")
	}
	full, err = parent.Underapprox.IsFull()
	must(err)
	if !full {
		t.Fatal("parent lower fact")
	}
	baseline := m.Stats().LiveReferences
	complete := false
	for n := int64(0); n < 1024; n++ {
		m.Budget().FailAfter(n)
		value, failure := graph.ComputeUnderapprox(parent, simplifier)
		m.Budget().FailAfter(-1)
		if failure == nil {
			must(value.Close())
			complete = true
		} else {
			if value != nil {
				t.Fatal("partial result", n, failure)
			}
			retry, err := graph.ComputeUnderapprox(parent, simplifier)
			must(err)
			must(retry.Close())
		}
		if got := m.Stats().LiveReferences; got != baseline {
			t.Fatal("lower fact references", n, got, baseline)
		}
		if complete {
			break
		}
	}
	if !complete {
		t.Fatal("allocation sweep did not finish")
	}
}
