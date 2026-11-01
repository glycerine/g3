package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"math/big"
	"testing"
)

func TestDualityImplicantSolverNativeScopesAndModelCube(t *testing.T) {
	for _, extensional := range []bool{false, true} {
		name := "named"
		if extensional {
			name = "extensional"
		}
		t.Run(name, func(t *testing.T) {
			ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			g, err := NewRPFP(ctx)
			must(err)
			defer func() { must(g.Close()) }()
			must(g.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
			aux, err := NewImplicantSolver(g, native, extensional)
			must(err)
			defer func() { must(aux.Close()) }()
			p, err := ctx.BoolConst("implicant-solver-p")
			must(err)
			closeExpr(t, p)
			q, err := ctx.BoolConst("implicant-solver-q")
			must(err)
			closeExpr(t, q)
			nq, err := q.Not()
			must(err)
			closeExpr(t, nq)
			must(aux.Push())
			must(aux.Add(p))
			must(aux.Add(nq))
			status, err := aux.Check()
			must(err)
			if status != lbool.True {
				t.Fatal(status)
			}
			cube, err := aux.GetImplicant()
			must(err)
			defer cube.Close()
			h, err := cube.Raw()
			must(err)
			count, err := m.GetNumArgs(h)
			must(err)
			if count != 2 || g.dualModel == nil {
				t.Fatal("missing full model cube/owner model")
			}
			for i, want := range []*Expr{p, nq} {
				got, err := m.GetArg(h, uint32(i))
				must(err)
				expected, err := want.Raw()
				must(err)
				if got != expected {
					t.Fatal("cube atom order changed")
				}
			}
			must(aux.Push())
			must(aux.Add(q))
			status, err = aux.Check()
			must(err)
			if status != lbool.False {
				t.Fatal("nested contradiction not asserted", status)
			}
			must(aux.Pop())
			status, err = aux.Check()
			must(err)
			if status != lbool.True {
				t.Fatal("nested pop not restored", status)
			}
			must(aux.Push())
			must(aux.Add(p))
			status, err = aux.Check()
			must(err)
			if status != lbool.True {
				t.Fatal(status)
			}
			must(aux.Pop())
			afterPop, err := aux.GetImplicant()
			must(err)
			afterRaw, err := afterPop.Raw()
			must(err)
			if afterRaw != h || g.dualModel == nil {
				t.Fatal("last checked model did not survive pop")
			}
			must(afterPop.Close())
			must(aux.Pop())
			if len(aux.assumptions) != 0 || len(aux.limits) != 0 {
				t.Fatal("base assertion bookkeeping not restored")
			}
			if err := aux.Pop(); err == nil {
				t.Fatal("base scope popped")
			}
			must(aux.Close())
			status, err = native.CheckSat(nil)
			must(err)
			if status != lbool.True {
				t.Fatal("closing wrapper closed/changed borrowed solver", status)
			}
		})
	}
}

func TestDualityImplicantSolverReassertsStoreNamesAfterPop(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	aux, err := NewImplicantSolver(g, native, false)
	must(err)
	defer func() { must(aux.Close()) }()
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	zero := wrap(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
	one := wrap(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	integer, err := m.GetSort(raw(zero))
	must(err)
	arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), integer)
	must(err)
	ref, err := m.NewRef(arraySort)
	must(err)
	defer func() { must(m.DelRef(&ref)) }()
	base := wrap(m.MkConstArray(arraySort, raw(zero)))
	family, err := m.ArrayFamilyID()
	must(err)
	store := wrap(m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{raw(base), raw(one), raw(one)}))
	selectValue := wrap(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{raw(store), raw(one)}))
	formula := wrap(m.MkEq(raw(selectValue), raw(one)))
	must(aux.Push())
	must(aux.Push())
	must(aux.Add(formula))
	if len(aux.namings) != 1 || len(aux.assumptions) != 1 {
		t.Fatal("store naming/assertion bookkeeping")
	}
	transformed, err := aux.assumptions[0].Clone()
	must(err)
	defer transformed.Close()
	status, err := aux.Check()
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	cube, err := aux.GetImplicant()
	must(err)
	defer cube.Close()
	if raw(cube) != raw(formula) {
		t.Fatal("full cube did not undo store name")
	}
	must(aux.Pop())
	assertions, err := native.NumAssertions()
	must(err)
	if assertions != 1 || len(aux.assumptions) != 0 || len(aux.namings) != 0 {
		t.Fatal("lost definition not reasserted in parent", assertions)
	}
	opposite := wrap(m.MkNot(raw(transformed)))
	must(native.Push())
	must(native.Assert(raw(opposite)))
	status, err = native.CheckSat(nil)
	must(err)
	must(native.Pop(1))
	if status != lbool.False {
		t.Fatal("popped name lost its array definition", status)
	}
	must(aux.Add(formula))
	if raw(aux.assumptions[0]) != raw(transformed) || len(aux.namings) != 0 {
		t.Fatal("persistent memo failed to reuse parent definition")
	}
	must(aux.Pop())
	assertions, err = native.NumAssertions()
	must(err)
	if assertions != 0 {
		t.Fatal("enclosing native scope not restored", assertions)
	}
}

func TestDualityImplicantSolverCopiesGraphEdgeConstraints(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	factory, err := smtcore.NewSMTSolverFactory(smtcore.SMTSolverConfig{Params: ctx.config.params, Globals: globals})
	must(err)
	defer factory.Close()
	separate, err := factory.New(m, nil, false, true, false, "")
	must(err)
	defer separate.Close()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(native))
	p, err := ctx.BoolConst("implicant-edge-p")
	must(err)
	closeExpr(t, p)
	relation, err := ctx.BoolConst("implicant-edge-R")
	must(err)
	closeExpr(t, relation)
	node, err := g.CreateNode(relation)
	must(err)
	edge, err := g.CreateLowerBoundEdge(node)
	must(err)
	must(edge.F.SetEmpty())
	must(g.Push())
	must(g.AssertEdge(edge, 0, false, false))
	must(g.ConstrainEdgeLocalized(edge, p))
	aux, err := NewImplicantSolver(g, separate, true)
	must(err)
	defer func() { must(aux.Close()) }()
	must(aux.Push())
	must(aux.AddEdge(edge))
	if len(aux.assumptions) != 2 {
		t.Fatal("edge formula or localized constraint missing")
	}
	for i, ref := range []ast.ASTRef{edge.dual, edge.constraints[0]} {
		want, err := m.RefGet(ref)
		must(err)
		got, err := aux.assumptions[i].Raw()
		must(err)
		if got != want {
			t.Fatal("edge constraints copied out of source order")
		}
	}
	status, err := aux.Check()
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	opposite, err := p.Not()
	must(err)
	closeExpr(t, opposite)
	must(aux.Add(opposite))
	status, err = aux.Check()
	must(err)
	if status != lbool.False {
		t.Fatal("localized constraint missing from auxiliary solver", status)
	}
	must(aux.Pop())
	must(g.Pop(1))
	status, err = aux.Check()
	must(err)
	if status != lbool.True {
		t.Fatal("auxiliary scope survived pop", status)
	}
}
