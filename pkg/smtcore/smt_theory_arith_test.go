package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type smtTheoryArithFixture struct {
	m      *ast.ASTManager
	family ast.FamilyID
	state  *SMTInternalizerState
	theory *SMTTheoryArith
}

func newSMTTheoryArithFixture(t *testing.T, integerExt bool) *smtTheoryArithFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTTheoryArith(family, integerExt)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	t.Cleanup(func() { astTestMust(t, theory.Close()) })
	return &smtTheoryArithFixture{m: m, family: family, state: state, theory: theory}
}

func (f *smtTheoryArithFixture) constant(t *testing.T, name string, isInt bool) ast.ASTHandle {
	t.Helper()
	kind := int32(smtArithRealSort)
	if isInt {
		kind = smtArithIntSort
	}
	sort, err := f.m.MkFamilySort(f.family, kind, nil)
	astTestMust(t, err)
	c, err := f.m.MkConst(astTestSymbol(t, f.m, name), sort)
	astTestMust(t, err)
	return c
}

func (f *smtTheoryArithFixture) app(t *testing.T, op int32, args ...ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	h, err := f.m.MkFamilyApp(f.family, op, nil, args)
	astTestMust(t, err)
	return h
}

func (f *smtTheoryArithFixture) num(t *testing.T, text string, isInt bool) ast.ASTHandle {
	t.Helper()
	var h ast.ASTHandle
	var err error
	if isInt {
		h, err = f.m.ArithmeticIntegerNumeralText(text)
	} else {
		h, err = f.m.ArithmeticRealNumeralText(text)
	}
	astTestMust(t, err)
	return h
}

// assertAtom internalizes the atom and assigns it true.
func (f *smtTheoryArithFixture) assertAtom(t *testing.T, atom ast.ASTHandle) {
	t.Helper()
	astTestMust(t, f.state.Internalize(atom, true))
	bv, found := f.state.BoolVar(atom)
	if !found {
		t.Fatal("atom has no Boolean variable")
	}
	astTestMust(t, f.theory.Assign(bv, true))
}

func TestSMTTheoryArithTermRowValue(t *testing.T) {
	f := newSMTTheoryArithFixture(t, false)
	x := f.constant(t, "arith-port-x", false)
	sum := f.app(t, smtArithOpAdd, x, f.num(t, "1", false))
	astTestMust(t, f.state.Internalize(sum, false))
	f.assertAtom(t, f.app(t, smtArithOpGE, x, f.num(t, "2", false)))
	ok, err := f.theory.propagateCore()
	if err != nil || !ok {
		t.Fatal("propagate", ok, err)
	}
	v := f.state.context.enodes.get(sum).TheoryVar(f.family)
	if got := f.theory.getValue(v); got.rational.Cmp(big.NewRat(3, 1)) != 0 || got.infinitesimal.Sign() != 0 {
		t.Fatalf("x + 1 with x >= 2: got %s + %s eps", got.rational.RatString(), got.infinitesimal.RatString())
	}
	if f.theory.stats.addRows != 0 || f.theory.stats.assertLower != 1 {
		t.Fatalf("statistics: add rows %d, assert lower %d", f.theory.stats.addRows, f.theory.stats.assertLower)
	}
}

func TestSMTTheoryArithBoundConflict(t *testing.T) {
	f := newSMTTheoryArithFixture(t, false)
	x := f.constant(t, "arith-port-cx", false)
	y := f.constant(t, "arith-port-cy", false)
	sum := f.app(t, smtArithOpAdd, x, y)
	f.assertAtom(t, f.app(t, smtArithOpGE, x, f.num(t, "3", false)))
	f.assertAtom(t, f.app(t, smtArithOpGE, y, f.num(t, "4", false)))
	f.assertAtom(t, f.app(t, smtArithOpLE, sum, f.num(t, "6", false)))
	ok, err := f.theory.propagateCore()
	if err != nil || ok {
		t.Fatal("x>=3, y>=4, x+y<=6 must conflict", ok, err)
	}
	if !f.state.Inconsistent() || f.theory.stats.conflicts != 1 {
		t.Fatal("conflict not set on the context", f.theory.stats.conflicts)
	}
}

// Two variables that become fixed at the same value are equated by
// fixed_var_eh as soon as the second bound is asserted.
func TestSMTTheoryArithFixedVarEqualityReachesCore(t *testing.T) {
	f := newSMTTheoryArithFixture(t, true)
	x := f.constant(t, "arith-port-fx", true)
	y := f.constant(t, "arith-port-fy", true)
	seven := f.num(t, "7", true)
	for _, v := range []ast.ASTHandle{x, y} {
		f.assertAtom(t, f.app(t, smtArithOpGE, v, seven))
		f.assertAtom(t, f.app(t, smtArithOpLE, v, seven))
	}
	ok, err := f.theory.propagateCore()
	if err != nil || !ok {
		t.Fatal("propagate", ok, err)
	}
	if f.theory.stats.fixedEqs != 1 {
		t.Fatalf("fixed_var_eh equalities: %d", f.theory.stats.fixedEqs)
	}
	_, err = f.state.PropagateCore()
	astTestMust(t, err)
	if f.state.context.enodes.get(x).Root() != f.state.context.enodes.get(y).Root() {
		t.Fatal("x and y, both fixed at 7, were not merged")
	}
}

func TestSMTTheoryArithPopRestoresBounds(t *testing.T) {
	f := newSMTTheoryArithFixture(t, false)
	x := f.constant(t, "arith-port-px", false)
	atom := f.app(t, smtArithOpGE, x, f.num(t, "5", false))
	astTestMust(t, f.state.Internalize(atom, true)) // the variable predates the scope
	astTestMust(t, f.state.PushScope())
	f.assertAtom(t, atom)
	if ok, err := f.theory.propagateCore(); err != nil || !ok {
		t.Fatal(ok, err)
	}
	v := f.state.context.enodes.get(x).TheoryVar(f.family)
	if f.theory.lower(v) == nil {
		t.Fatal("lower bound missing")
	}
	astTestMust(t, f.state.PopScopes(1))
	if f.theory.lower(v) != nil || len(f.theory.boundTrail) != 0 {
		t.Fatal("pop must restore the bound and unwind the trail")
	}
}
