package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"testing"
)

func TestRPFPModelEvaluationCompletesIndexedParameters(t *testing.T) {
	ctx, m, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	must(err)
	defer catalog.Close()
	globals, err := uparams.NewGlobalParamsService(m.Symbols, ctx.config.params, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	must(err)
	defer globals.Close()
	must(globals.InitializeSource())
	memory, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: m.Budget()})
	must(err)
	defer memory.Close()
	cooperation, err := cooperate.NewCooperation(m.Budget())
	must(err)
	defer cooperation.Close()
	x, err := ctx.BoolConst("eval-x")
	must(err)
	closeExpr(t, x)
	sort, err := x.Sort()
	must(err)
	defer sort.Close()
	name, err := ctx.StringSymbol("eval-R")
	must(err)
	decl, err := ctx.Function(name, []*Sort{sort}, sort)
	must(err)
	closeDecl(t, decl)
	term, err := decl.Call(x)
	must(err)
	closeExpr(t, term)
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	must(g.BindModelEvaluator(model.ModelEvaluatorConfig{Context: ctx.config.params, Globals: globals, Memory: memory, Cooperation: cooperation}))
	node, err := g.CreateNode(term)
	must(err)
	must(node.Bound.SetEmpty())
	edge, err := g.CreateLowerBoundEdge(node)
	must(err)
	if _, err = g.EvalNode(node); err == nil {
		t.Fatal("evaluated absent model")
	}
	must(g.Push())
	must(g.AssertNode(node))
	status, err := g.Check(node, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal("expected SAT")
	}
	empty, err := g.Empty(node)
	must(err)
	if empty {
		t.Fatal("active node reported empty")
	}
	// No assertion mentions the indexed parameter: source model completion
	// supplies a value and EvalNode applies the original relation declaration.
	evaluated, err := g.EvalNode(node)
	must(err)
	h, err := evaluated.Raw()
	must(err)
	d, err := m.GetDecl(h)
	must(err)
	wantDecl, err := decl.Raw()
	must(err)
	if d != wantDecl {
		t.Fatal("changed node relation")
	}
	arg, err := m.GetArg(h, 0)
	must(err)
	if !m.IsTrue(arg) && !m.IsFalse(arg) {
		t.Fatal("parameter was not completed")
	}
	value, err := g.Eval(edge, x)
	must(err)
	vh, err := value.Raw()
	must(err)
	if vh != arg {
		t.Fatal("edge evaluation did not use indexed node parameter")
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	lower, err := g.ComputeUnderapprox(node, simplifier)
	must(err)
	lowerHandle, err := lower.Raw()
	must(err)
	if !m.IsTrue(lowerHandle) {
		t.Fatal("unconstrained lower fact should be true")
	}
	must(lower.Close())
	full, err := node.Underapprox.IsFull()
	must(err)
	if !full {
		t.Fatal("active node did not acquire its lower fact")
	}
	leaf, err := g.CloneNode(node)
	must(err)
	lower, err = g.ComputeUnderapprox(leaf, simplifier)
	must(err)
	must(lower.Close())
	emptyLower, err := leaf.Underapprox.IsEmpty()
	must(err)
	if !emptyLower {
		t.Fatal("unexpanded leaf must have empty lower fact")
	}
	must(value.Close())
	must(evaluated.Close())
	must(g.Pop(1))
	b, vars, err := g.RedVars(node)
	must(err)
	negative, err := b.Not()
	must(err)
	must(g.Push())
	must(g.ConstrainEdgeLocalized(edge, negative))
	status, err = g.Check(node, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal("inactive node graph was not SAT")
	}
	empty, err = g.Empty(node)
	must(err)
	if !empty {
		t.Fatal("inactive node reported active")
	}
	must(negative.Close())
	must(b.Close())
	for _, v := range vars {
		must(v.Close())
	}
	must(g.Pop(1))
	must(g.Close())
}
