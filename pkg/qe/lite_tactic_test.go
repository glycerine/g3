package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/statistics"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (f *fmFixture) liteTactic() *LiteTactic {
	t, err := NewLiteTactic(f.r.manager, nil, f.r.config)
	fmMust(f.t, err)
	f.t.Cleanup(func() { fmMust(f.t, t.Close()) })
	return t
}
func (f *fmFixture) quantifiedDefinition() ast.ASTHandle {
	x := f.variable(0, f.integer)
	m := f.r.manager
	body := f.own(m.MkAndArray([]ast.ASTHandle{f.own(m.MkEq(x, f.number("3"))), f.predicate("tactic-p", x)}))
	name, err := m.Symbols.Intern("tactic-x")
	fmMust(f.t, err)
	return f.own(m.MkQuantifier(ast.ASTQuantifierSpec{Sorts: []ast.ASTHandle{f.integer}, Names: []usymbol.Symbol{name}, Body: body}))
}
func (f *fmFixture) goal(proofs, cores bool) *tactic.Goal {
	g, err := tactic.NewGoal(f.r.manager, proofs, true, cores)
	fmMust(f.t, err)
	f.t.Cleanup(func() { fmMust(f.t, g.Close()) })
	return g
}
func (f *fmFixture) result() *tactic.TacticResult {
	r := new(tactic.TacticResult)
	f.t.Cleanup(func() { fmMust(f.t, r.Close()) })
	return r
}

func TestLiteTacticGoalDependenciesAppendDepthAndCancellation(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	tt := f.liteTactic()
	g := f.goal(false, true)
	result := f.result()
	q := f.quantifiedDefinition()
	label := f.constant("tactic-label", f.boolean)
	dep, err := m.DependencyLeaf(label)
	fmMust(t, err)
	fmMust(t, m.DependencyInc(dep))
	t.Cleanup(func() { fmMust(t, m.DependencyDec(dep)) })
	fmMust(t, g.AssertWithProof(q, ast.ASTHandle{}, dep))
	fmMust(t, result.BindManager(m))
	fmMust(t, result.AddGoal(g))
	fmMust(t, result.SetCore(dep))
	m.ResourceLimit().IncCancel()
	err = tt.Apply(g, result)
	reason := m.ResourceLimit().CancelMessage()
	m.ResourceLimit().DecCancel()
	var canceled tactic.TacticException
	if !errors.As(err, &canceled) || canceled.Message() != reason {
		t.Fatal("source cancellation", err)
	}
	n, err := g.Depth()
	fmMust(t, err)
	if n != 0 || len(result.Goals) != 1 {
		t.Fatal("canceled call changed goals/depth")
	}
	if !result.Core.IsNull() {
		t.Fatal("source converter/core reset was skipped")
	}
	fmMust(t, tt.Apply(g, result))
	if len(result.Goals) != 2 || result.Goals[0] != g || result.Goals[1] != g {
		t.Fatal("source appends borrowed input goal")
	}
	n, err = g.Depth()
	fmMust(t, err)
	if n != 1 {
		t.Fatal("depth increment", n)
	}
	h, err := g.Form(0)
	fmMust(t, err)
	if h != f.predicate("tactic-p", f.number("3")) {
		t.Fatal("quantified assertion not rewritten")
	}
	actual, err := g.Dependency(0)
	fmMust(t, err)
	if actual != dep {
		t.Fatal("assertion dependency changed")
	}
	if result.ModelConverter != nil || result.ProofConverter != nil || !result.Core.IsNull() {
		t.Fatal("unexpected converter/core")
	}
	copy := *tt
	if err := copy.Apply(g, result); err == nil {
		t.Fatal("copied tactic accepted")
	}
	fmMust(t, tt.IncRef())
	fmMust(t, tt.DecRef())
	if err := tt.Apply(g, result); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed tactic accepted", err)
	}
}

func TestLiteTacticParamsCleanupTranslationAndNoOpSurfaces(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	ctx := f.r.config.Context
	p, err := ctx.NewValue()
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, ctx.Del(&p)) })
	key := "ignore_patterns_on_ground_qbody"
	fmMust(t, ctx.SetBoolName(&p, &key, false))
	tt, err := NewLiteTactic(m, &p, f.r.config)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, tt.Close()) })
	fmMust(t, ctx.SetBoolName(&p, &key, true))
	fmMust(t, tt.UpdateParams(&p))
	value, err := ctx.GetBoolName(&tt.implementation.der.params, &key, true)
	fmMust(t, err)
	if value {
		t.Fatal("UpdateParams changed active implementation")
	}
	fmMust(t, tt.Cleanup())
	value, err = ctx.GetBoolName(&tt.implementation.der.params, &key, false)
	fmMust(t, err)
	if !value {
		t.Fatal("Cleanup did not reconstruct from stored parameters")
	}
	target, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: m.Symbols, Numerals: m.NumeralService(), Budget: m.Budget()})
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, target.Close()) })
	translated, err := tt.Translate(target)
	fmMust(t, err)
	copy := translated.(*LiteTactic)
	t.Cleanup(func() { fmMust(t, copy.Close()) })
	translation, err := ast.NewASTTranslation(m, target, true)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, translation.Close()) })
	g := f.goal(false, false)
	fmMust(t, g.Assert(f.quantifiedDefinition()))
	g2, err := g.Translate(translation)
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, g2.Close()) })
	r := f.result()
	fmMust(t, copy.Apply(g2, r))
	if n, err := g2.Size(); err != nil || n != 1 {
		t.Fatal("translated goal", n, err)
	}
	if len(statistics.Sums(tt.CollectStatistics)) != 0 {
		t.Fatal("source empty statistics")
	}
	fmMust(t, tt.CollectParamDescrs(nil))
	fmMust(t, tt.ResetStatistics())
	fmMust(t, tt.SetLogic(usymbol.Symbol{}))
	fmMust(t, tt.SetProgressCallback(nil))
	fmMust(t, tt.Reset())
}

func TestLiteTacticProofNullGuardAndNonQuantifiedSkip(t *testing.T) {
	f := newFMFixtureProofs(t, true)
	tt := f.liteTactic()
	g := f.goal(true, false)
	r := f.result()
	q := f.quantifiedDefinition()
	fmMust(t, g.Assert(q))
	err := tt.Apply(g, r)
	if !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("source null-proof guard", err)
	}
	h, err := g.Form(0)
	fmMust(t, err)
	if h != q {
		t.Fatal("guard changed assertion")
	}
	d, err := g.Depth()
	fmMust(t, err)
	if d != 0 || len(r.Goals) != 0 {
		t.Fatal("guard committed a result")
	}
	plain := f.goal(true, false)
	atom := f.constant("tactic-plain", f.boolean)
	fmMust(t, plain.Assert(atom))
	fmMust(t, tt.Apply(plain, r))
	if len(r.Goals) != 1 || r.Goals[0] != plain {
		t.Fatal("nonquantified proof-enabled goal was not skipped")
	}
}
