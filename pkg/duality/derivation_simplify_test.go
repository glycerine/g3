package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"strconv"
	"testing"
)

func TestDerivationAnnotationComplexitySourceThresholds(t *testing.T) {
	ctx, m, _, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer manager.Close()
	original, err := NewRPFP(ctx)
	must(err)
	defer original.Close()
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer working.Close()
	base, err := NewDerivationTree(original, working)
	must(err)
	search, err := NewBacktrackingDerivation(base)
	must(err)
	relation, err := ctx.BoolConst("complexity-R")
	must(err)
	closeExpr(t, relation)
	node, err := working.CreateNode(relation)
	must(err)
	raw := func(v *Expr) ast.ASTHandle { t.Helper(); h, err := v.Raw(); must(err); return h }
	wrap := func(h ast.ASTHandle, err error) *Expr {
		t.Helper()
		must(err)
		v, e := NewExpr(ctx, h)
		must(e)
		closeExpr(t, v)
		return v
	}
	chain := func(operators int, repeated bool) *Expr {
		t.Helper()
		var terms []*Expr
		for i := 0; i <= operators; i++ {
			name := "complexity-atom-" + strconv.Itoa(i)
			if repeated {
				name = "complexity-repeated"
			}
			term, err := ctx.BoolConst(name)
			must(err)
			closeExpr(t, term)
			terms = append(terms, term)
		}
		out := terms[operators]
		for i := operators - 1; i >= 0; i-- {
			if i%2 == 0 {
				out = wrap(m.MkAnd(raw(terms[i]), raw(out)))
			} else {
				out = wrap(m.MkOr(raw(terms[i]), raw(out)))
			}
		}
		return out
	}
	set := func(formula *Expr) { t.Helper(); must(m.SetRef(node.Annotation.formula, raw(formula))) }
	for _, count := range []int{3, 4, 10} {
		set(chain(count, false))
		complicated, err := search.NodeTooComplicated(node, manager)
		must(err)
		if complicated != (count > 3) {
			t.Fatal("post-simplification threshold mismatch", count, complicated)
		}
		after, err := node.Annotation.Formula()
		must(err)
		remaining, err := CountOperators(after)
		must(err)
		must(after.Close())
		if remaining != count {
			t.Fatal("independent alternating skeleton changed", remaining, count)
		}
	}
	// This formula could collapse by redundancy removal, but the source >10
	// fast path returns immediately, without requiring the simplifier service.
	large := chain(11, true)
	set(large)
	complicated, err := search.NodeTooComplicated(node, nil)
	must(err)
	after, err := node.Annotation.Formula()
	must(err)
	defer after.Close()
	if !complicated || raw(after) != raw(large) {
		t.Fatal("large-annotation shortcut mutated the fact")
	}
	p, err := ctx.BoolConst("complexity-contradiction")
	must(err)
	closeExpr(t, p)
	negative := wrap(m.MkNot(raw(p)))
	contradiction := wrap(m.MkAnd(raw(p), raw(negative)))
	set(contradiction)
	complicated, err = search.NodeTooComplicated(node, manager)
	must(err)
	final, err := node.Annotation.Formula()
	must(err)
	defer final.Close()
	if complicated || !m.IsFalse(raw(final)) {
		t.Fatal("small annotation not simplified before classification")
	}
}

func TestDerivationSimplificationRejectsForeignManagerBeforeRewrite(t *testing.T) {
	ctx, _, native, _ := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	other, otherManager, _, otherGlobals := dualityNativeSolverServicesFixture(t)
	foreign, err := interp.NewManagerWithParams(otherManager, other.config.params, otherGlobals)
	must(err)
	defer foreign.Close()
	original, err := NewRPFP(ctx)
	must(err)
	defer original.Close()
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer working.Close()
	must(working.BindSolver(native))
	base, err := NewDerivationTree(original, working)
	must(err)
	search, err := NewBacktrackingDerivation(base)
	must(err)
	term, err := ctx.BoolConst("foreign-simplifier-R")
	must(err)
	closeExpr(t, term)
	node, err := working.CreateNode(term)
	must(err)
	base.Top = node
	_, err = working.CreateLowerBoundEdge(node)
	must(err)
	before := otherManager.Stats()
	if _, err := search.NodeTooComplicated(node, foreign); !errors.Is(err, ErrContextMismatch) {
		t.Fatal("foreign complexity simplifier accepted", err)
	}
	formula, err := node.Annotation.Formula()
	must(err)
	defer formula.Close()
	if out, cases, err := working.StrengthenFormulaByCaseSplitting(formula, foreign, nil); !errors.Is(err, ErrContextMismatch) || out != nil || cases != nil {
		t.Fatal("foreign case simplifier accepted", err)
	}
	runner := &CaseInterpolator{Graph: working, Auxiliary: native, Simplifier: foreign}
	if err := runner.Interpolate(node, node, nil); !errors.Is(err, ErrContextMismatch) {
		t.Fatal("foreign interpolation simplifier accepted", err)
	}
	after := otherManager.Stats()
	if before.LiveReferences != after.LiveReferences {
		t.Fatal("foreign-owner rejection allocated leaked references")
	}
	depth, err := native.ScopeLevel()
	must(err)
	if depth != 0 {
		t.Fatal("foreign-owner rejection pushed solver scope")
	}
}
