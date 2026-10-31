package arith

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func fmTacticFixture(t *testing.T, proofs bool) (*FMTactic, *ast.ASTManager) {
	t.Helper()
	base, m := fixDLFixture(t, proofs)
	fm, err := NewFMTactic(m, base.context, base.globals, nil, base.services)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, fm.Close()) })
	return fm, m
}
func fmResult(t *testing.T) *tactic.TacticResult {
	t.Helper()
	r := new(tactic.TacticResult)
	t.Cleanup(func() { equationMust(t, r.Close()) })
	return r
}
func fmPredicate(t *testing.T, m *ast.ASTManager, x ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	sort, err := m.GetSort(x)
	equationMust(t, err)
	boolean, err := m.MkBoolSort()
	equationMust(t, err)
	name, err := m.Symbols.Intern("P")
	equationMust(t, err)
	decl, err := m.MkFuncDecl(name, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
	equationMust(t, err)
	fixDLPin(t, m, decl)
	app, err := m.MkApp(decl, []ast.ASTHandle{x})
	equationMust(t, err)
	fixDLPin(t, m, app)
	return app
}
func TestFMTacticContradictionAndCost(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	x := fixDLConst(t, m, false, "x")
	g := fixDLGoal(t, m, false, true, false, fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "1")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "0")))
	r := fmResult(t)
	equationMust(t, fm.Apply(g, r))
	n, err := r.Goals[0].Size()
	equationMust(t, err)
	form, err := r.Goals[0].Form(0)
	equationMust(t, err)
	isFalse := m.IsFalse(form)
	depth, err := r.Goals[0].Depth()
	equationMust(t, err)
	if n != 1 || !isFalse || depth != 1 || r.ModelConverter != nil || fm.run.eliminated != 1 || fm.run.counter != 6 {
		t.Fatalf("false=%v n=%d depth=%d eliminated=%d cost=%d", isFalse, n, depth, fm.run.eliminated, fm.run.counter)
	}
	size, err := g.Size()
	equationMust(t, err)
	if size != 2 {
		t.Fatal("input changed", size)
	}
}
func TestFMTacticModelSurvivesCleanup(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	x := fixDLConst(t, m, false, "x")
	g := fixDLGoal(t, m, false, true, false, fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "2")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4")))
	r := fmResult(t)
	equationMust(t, fm.Apply(g, r))
	n, err := r.Goals[0].Size()
	equationMust(t, err)
	if n != 0 || r.ModelConverter == nil {
		t.Fatal("missing elimination/model", n)
	}
	equationMust(t, fm.Cleanup())
	md, err := model.NewModel(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	core := fmApply(t, r.ModelConverter.(*FMModelConverter), md)
	if got := fmValue(t, m, core, x); got != "3" {
		t.Fatal("midpoint", got)
	}
}
func TestFMTacticLimitPreservesBoundsInSourceOrder(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	name := "fm_limit"
	equationMust(t, fm.context.SetUIntName(&fm.params, &name, 0))
	equationMust(t, fm.UpdateParams(&fm.params))
	x := fixDLConst(t, m, false, "x")
	lower := fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "2"))
	upper := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "4"))
	g := fixDLGoal(t, m, false, false, false, lower, upper)
	r := fmResult(t)
	equationMust(t, fm.Apply(g, r))
	size, err := r.Goals[0].Size()
	equationMust(t, err)
	if size != 2 || fm.run.eliminated != 0 {
		t.Fatal("cutoff ignored", size)
	}
	first, err := r.Goals[0].Form(0)
	equationMust(t, err)
	if first != upper {
		t.Fatal("source upper-first order", first, upper)
	}
}
func TestFMTacticForbidsSharedTermsAndPublishesDeferredResolvent(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	x := fixDLConst(t, m, false, "x")
	y := fixDLConst(t, m, false, "y")
	pred := fmPredicate(t, m, y)
	lower := fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "0"))
	sum := fixDLApp(t, m, 6, x, y)
	upper := fixDLApp(t, m, 2, sum, fixDLNumber(t, m, false, "3"))
	g := fixDLGoal(t, m, false, false, false, pred, lower, upper)
	r := fmResult(t)
	equationMust(t, fm.Apply(g, r))
	size, err := r.Goals[0].Size()
	equationMust(t, err)
	if size != 2 || fm.run.eliminated != 1 {
		t.Fatal("lost deferred resolvent", size, fm.run.eliminated)
	}
	expected := fixDLApp(t, m, 2, y, fixDLNumber(t, m, false, "3"))
	a, err := r.Goals[0].Form(0)
	equationMust(t, err)
	b, err := r.Goals[0].Form(1)
	equationMust(t, err)
	if a != pred || b != expected {
		t.Fatal("wrong shared-variable projection", a, b, expected)
	}
}
func TestFMTacticIntegerPolicyAndReuseAfterCancel(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	x := fixDLConst(t, m, true, "x")
	g := fixDLGoal(t, m, false, false, false, fixDLApp(t, m, 3, x, fixDLNumber(t, m, true, "1")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, true, "0")))
	r := fmResult(t)
	equationMust(t, fm.Apply(g, r))
	size, err := r.Goals[0].Size()
	equationMust(t, err)
	if size != 2 {
		t.Fatal("default integer policy", size)
	}
	name := "fm_real_only"
	equationMust(t, fm.context.SetBoolName(&fm.params, &name, false))
	equationMust(t, fm.UpdateParams(&fm.params))
	m.ResourceLimit().Cancel()
	err = fm.Apply(g, r)
	m.ResourceLimit().ResetCancel()
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatal("missing cancellation", err)
	}
	equationMust(t, fm.Apply(g, r))
	form, err := r.Goals[0].Form(0)
	equationMust(t, err)
	no := m.IsFalse(form)
	if !no {
		t.Fatal("integer unit-bound contradiction")
	}
}
func TestFMTacticRejectsProofProduction(t *testing.T) {
	fm, m := fmTacticFixture(t, true)
	g := fixDLGoal(t, m, true, false, false)
	err := fm.Apply(g, fmResult(t))
	if err == nil || !strings.Contains(err.Error(), "does not support proof production") || (fm.run == nil || fm.run.parser != nil) {
		t.Fatal("proof guard", err)
	}
}
func TestFMEliminationUnsignedCutoffProduct(t *testing.T) {
	config := FMEliminationConfig{CutoffSide: math.MaxUint32, CutoffProduct: 0}
	if !config.permits(65536, 65536) {
		t.Fatal("C++ unsigned product wrap")
	}
}

func TestFMTacticFactorySimplifiesBeforeElimination(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	factory, err := MakeFMTactic(m, fm.context, fm.globals, nil, fm.services)
	equationMust(t, err)
	equationMust(t, factory.IncRef())
	t.Cleanup(func() { equationMust(t, factory.DecRef()) })
	x := fixDLConst(t, m, false, "x")
	// The raw parser requires numeral right-hand sides and normalized monomials.
	lhs := fixDLApp(t, m, 6, x, x, fixDLNumber(t, m, false, "1"))
	g := fixDLGoal(t, m, false, true, false, fixDLApp(t, m, 2, lhs, fixDLNumber(t, m, false, "5")))
	r := fmResult(t)
	equationMust(t, factory.Apply(g, r))
	n, err := r.Goals[0].Size()
	equationMust(t, err)
	if n != 0 || r.ModelConverter == nil {
		t.Fatal("factory normalization/model", n)
	}
	var ref tactic.ModelRef
	md, err := model.NewModel(m)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, md.Close()) })
	equationMust(t, ref.Set(&md.CoreField))
	t.Cleanup(func() { equationMust(t, ref.Close()) })
	equationMust(t, r.ModelConverter.ApplyModel(&ref, 0))
	core, err := ref.Get()
	equationMust(t, err)
	if got := fmValue(t, m, core, x); got != "1" {
		t.Fatal("upper-bound model", got)
	}
}

func TestFMModelConverterDisplaysSavedClausesAndWriterErrors(t *testing.T) {
	c, m := fmModelFixture(t)
	var out bytes.Buffer
	equationMust(t, c.Display(&out))
	if out.String() != "(fm-model-converter)\n" {
		t.Fatal(out.String())
	}
	x := fixDLConst(t, m, false, "x")
	decl, err := m.GetDecl(x)
	equationMust(t, err)
	bound := fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "3"))
	equationMust(t, c.Insert(decl, []ast.ASTHandle{bound}))
	out.Reset()
	equationMust(t, c.Display(&out))
	if out.String() != "(fm-model-converter\n(x\n  (<= x 3.0)))\n" {
		t.Fatal(out.String())
	}
	if err = c.Display(fmShortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short write", err)
	}
}

type fmShortWriter struct{}

func (fmShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestFMTacticMemoryCheckpointAndEmptyGoal(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	name := "max_memory"
	equationMust(t, fm.context.SetUIntName(&fm.params, &name, 0))
	equationMust(t, fm.UpdateParams(&fm.params))
	g := fixDLGoal(t, m, false, false, false)
	r := fmResult(t)
	equationMust(t, fm.Apply(g, r))
	x := fixDLConst(t, m, false, "x")
	equationMust(t, g.Assert(fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "1"))))
	if err := fm.Apply(g, r); err == nil || !strings.Contains(err.Error(), "max. memory exceeded") {
		t.Fatal("memory checkpoint", err)
	}
	equationMust(t, fm.Cleanup())
}

func TestFMTacticContradictionPreservesBothCoreDependencies(t *testing.T) {
	fm, m := fmTacticFixture(t, false)
	x := fixDLConst(t, m, false, "x")
	g := fixDLGoal(t, m, false, false, true, fixDLApp(t, m, 3, x, fixDLNumber(t, m, false, "1")), fixDLApp(t, m, 2, x, fixDLNumber(t, m, false, "0")))
	boolean, err := m.MkBoolSort()
	equationMust(t, err)
	labels := make([]ast.ASTHandle, 2)
	for i, name := range []string{"lower", "upper"} {
		sym, err := m.Symbols.Intern(name)
		equationMust(t, err)
		labels[i], err = m.MkConst(sym, boolean)
		equationMust(t, err)
		fixDLPin(t, m, labels[i])
		dep, err := m.DependencyLeaf(labels[i])
		equationMust(t, err)
		form, err := g.Form(uint32(i))
		equationMust(t, err)
		equationMust(t, g.Update(uint32(i), form, ast.ASTHandle{}, dep))
	}
	r := fmResult(t)
	equationMust(t, fm.Apply(g, r))
	equationMust(t, fm.Cleanup())
	dep, err := r.Goals[0].Dependency(0)
	equationMust(t, err)
	for _, label := range labels {
		found, err := m.DependencyContains(dep, label)
		equationMust(t, err)
		if !found {
			t.Fatal("lost premise after cleanup", label)
		}
	}
}
