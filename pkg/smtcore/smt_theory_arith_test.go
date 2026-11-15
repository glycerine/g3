package smtcore

import (
	"math/big"
	"os"
	"strings"
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

// AUDIT.md smtcore_07-F2: eliminate charges
// (r1_sz + r2.size()) * a_kj.storage_size(), with storage_size the
// mpz size_info of numerator plus denominator.
func TestSMTTheoryArithEliminateChargesStorageSize(t *testing.T) {
	for _, test := range []struct {
		q    *big.Rat
		want uint
	}{
		{big.NewRat(1, 1), 2},
		{new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 40)), 4},
		{new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 40)), 4},
		{big.NewRat(1<<31-1, 1), 2},
		{big.NewRat(1<<32, 1), 4},
		{big.NewRat(-(1 << 31), 1), 2},
	} {
		if got := smtRatStorageSize(test.q); got != test.want {
			t.Fatal("storage size", test.q.RatString(), got, "want", test.want)
		}
	}
	f := newSMTTheoryArithFixture(t, true)
	x := f.constant(t, "arith-storage-x", true)
	y := f.constant(t, "arith-storage-y", true)
	s1 := f.app(t, smtArithOpAdd, x, y)
	s2 := f.app(t, smtArithOpAdd, x, f.app(t, smtArithOpMul, f.num(t, "-1", true), y))
	astTestMust(t, f.state.Internalize(s1, false))
	astTestMust(t, f.state.Internalize(s2, false))
	vs1 := f.state.context.enodes.get(s1).TheoryVar(f.family)
	vx := f.state.context.enodes.get(x).TheoryVar(f.family)
	row := &f.theory.rows[f.theory.getVarRow(vs1)]
	index := row.indexOf(vx)
	if index < 0 {
		t.Fatal("x missing from the row of x + y")
	}
	var coefficient big.Rat
	coefficient.Set(&row.entries[index].coefficient)
	before := f.m.Limit.Count()
	astTestMust(t, f.theory.pivot(true, vs1, vx, &coefficient, false))
	// pivot's limit().inc(r.size()) = 3, then eliminate's (3 + 3) * 2.
	if got := f.m.Limit.Count() - before; got != 15 {
		t.Fatal("pivot resource charge", got, "want 15")
	}
}

// AUDIT.md smtcore_07-F3: set_conflict calls dump_lemmas(false_literal,
// ante), which writes lemma_N.smt2 when smt.arith.dump_lemmas is set.
func TestSMTTheoryArithDumpLemmasOnConflict(t *testing.T) {
	t.Chdir(t.TempDir())
	old := smtLemmaID
	defer func() { smtLemmaID = old }()
	for _, dump := range []bool{false, true} {
		smtLemmaID = 0
		f := newSMTTheoryArithFixture(t, false)
		f.theory.params.DumpLemmas = dump
		x := f.constant(t, "arith-dump-x", false)
		y := f.constant(t, "arith-dump-y", false)
		sum := f.app(t, smtArithOpAdd, x, y)
		f.assertAtom(t, f.app(t, smtArithOpGE, x, f.num(t, "3", false)))
		f.assertAtom(t, f.app(t, smtArithOpGE, y, f.num(t, "4", false)))
		f.assertAtom(t, f.app(t, smtArithOpLE, sum, f.num(t, "6", false)))
		ok, err := f.theory.propagateCore()
		if err != nil || ok {
			t.Fatal("x>=3, y>=4, x+y<=6 must conflict", ok, err)
		}
		text, err := os.ReadFile("lemma_0.smt2")
		if !dump {
			if smtLemmaID != 0 || err == nil {
				t.Fatal("lemma dumped without dump_lemmas", smtLemmaID)
			}
			continue
		}
		astTestMust(t, err)
		if smtLemmaID != 1 || strings.Count(string(text), "(assert ") != 3 {
			t.Fatalf("conflict lemma dump %d:\n%s", smtLemmaID, text)
		}
	}
}
