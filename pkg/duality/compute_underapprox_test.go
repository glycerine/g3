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
	profile := NewProfiler(nil)
	ctx.activeProfiler = profile
	value, err := graph.ComputeUnderapprox(parent, simplifier)
	must(err)
	phase := profile.top.children["UnderapproxFormula"]
	if phase == nil || profile.current != profile.top || phase.children["RedVars"] != nil {
		t.Fatal("recursive underapproximation timer scopes")
	}
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
		if profile.current != profile.top {
			t.Fatal("underapproximation timer left active after allocation failure", n)
		}
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

func TestComputeUnderapproxParameterizedBooleanFact(t *testing.T) {
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
	x, err := ctx.BoolConst("lower-param-x")
	must(err)
	defer x.Close()
	sort, err := x.Sort()
	must(err)
	defer sort.Close()
	name, err := ctx.StringSymbol("lower-param-P")
	must(err)
	decl, err := ctx.Function(name, []*Sort{sort}, sort)
	must(err)
	defer decl.Close()
	application, err := decl.Call(x)
	must(err)
	defer application.Close()
	node, err := graph.CreateNode(application)
	must(err)
	must(node.Bound.SetEmpty())
	transformer, err := NewTransformer(ctx, nil, []*Expr{x}, x)
	must(err)
	defer transformer.Close()
	edge, err := graph.CreateEdge(node, transformer, nil)
	must(err)
	must(graph.Push())
	defer graph.Pop(1)
	must(graph.AssertNode(node))
	must(graph.AssertEdge(edge, 0, false, false))
	status, err := graph.Check(node, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	fact, err := graph.ComputeUnderapprox(node, simplifier)
	must(err)
	must(fact.Close())
	actual, err := node.Underapprox.Formula()
	must(err)
	defer actual.Close()
	ah, err := actual.Raw()
	must(err)
	xh, err := x.Raw()
	must(err)
	iff, err := m.MkIff(ah, xh)
	must(err)
	ref, err := m.NewRef(iff)
	must(err)
	defer m.DelRef(&ref)
	neg, err := m.MkNot(iff)
	must(err)
	must(native.Push())
	must(native.Assert(neg))
	status, err = native.CheckSat(nil)
	must(err)
	must(native.Pop(1))
	if status != lbool.False {
		t.Fatal("lower fact failed to preserve parameter", status)
	}
}

func TestComputeUnderapproxParameterizedIntegerFact(t *testing.T) {
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
	three, err := m.ArithmeticIntegerNumeralText("3")
	must(err)
	threeRef, err := m.NewRef(three)
	must(err)
	defer m.DelRef(&threeRef)
	integer, err := m.GetSort(three)
	must(err)
	symbol, err := m.Symbols.Intern("lower-int-x")
	must(err)
	xh, err := m.MkConst(symbol, integer)
	must(err)
	x, err := NewExpr(ctx, xh)
	must(err)
	defer x.Close()
	integerSort, err := x.Sort()
	must(err)
	defer integerSort.Close()
	truth, err := ctx.BoolVal(true)
	must(err)
	defer truth.Close()
	booleanSort, err := truth.Sort()
	must(err)
	defer booleanSort.Close()
	name, err := ctx.StringSymbol("lower-int-P")
	must(err)
	decl, err := ctx.Function(name, []*Sort{integerSort}, booleanSort)
	must(err)
	defer decl.Close()
	app, err := decl.Call(x)
	must(err)
	defer app.Close()
	node, err := graph.CreateNode(app)
	must(err)
	must(node.Bound.SetEmpty())
	eq, err := m.MkEq(xh, three)
	must(err)
	condition, err := NewExpr(ctx, eq)
	must(err)
	defer condition.Close()
	transformer, err := NewTransformer(ctx, nil, []*Expr{x}, condition)
	must(err)
	defer transformer.Close()
	edge, err := graph.CreateEdge(node, transformer, nil)
	must(err)
	must(graph.Push())
	defer graph.Pop(1)
	must(graph.AssertNode(node))
	must(graph.AssertEdge(edge, 0, false, false))
	status, err := graph.Check(node, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	fact, err := graph.ComputeUnderapprox(node, simplifier)
	must(err)
	must(fact.Close())
	actual, err := node.Underapprox.Formula()
	must(err)
	defer actual.Close()
	ah, err := actual.Raw()
	must(err)
	iff, err := m.MkIff(ah, eq)
	must(err)
	ref, err := m.NewRef(iff)
	must(err)
	defer m.DelRef(&ref)
	neg, err := m.MkNot(iff)
	must(err)
	must(native.Push())
	must(native.Assert(neg))
	status, err = native.CheckSat(nil)
	must(err)
	must(native.Pop(1))
	if status != lbool.False {
		t.Fatal("integer lower fact differs from x = 3", status)
	}
}
