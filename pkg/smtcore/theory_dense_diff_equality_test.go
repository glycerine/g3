package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h:223-245 and
// src/smt/arith_eq_adapter.cpp:114-227.

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type smtDenseEqualityFixture struct {
	manager *ast.ASTManager
	state   *SMTInternalizerState
	theory  *SMTDenseDiffTheory
	family  ast.FamilyID
	sort    ast.ASTHandle
	x, y    ast.ASTHandle
	integer bool
}

func newSMTDenseEqualityFixture(t *testing.T, integer bool, config SMTArithState) smtDenseEqualityFixture {
	t.Helper()
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTDenseDiffTheory(family, smtDenseMI)
	astTestMust(t, err)
	astTestMust(t, theory.ConfigureArithmetic(config))
	astTestMust(t, state.RegisterTheory(theory))
	kind := int32(0)
	if integer {
		kind = 1
	}
	sort, err := m.MkFamilySort(family, kind, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "dense-equality-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "dense-equality-y"), sort)
	astTestMust(t, err)
	return smtDenseEqualityFixture{m, state, theory, family, sort, x, y, integer}
}

func (f smtDenseEqualityFixture) equality(t *testing.T, left, right ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	equality, err := f.manager.MkEq(left, right)
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(equality, true))
	return equality
}

func TestSMTDenseDifferenceEagerEqualityAxiomControl(t *testing.T) {
	for _, integer := range []bool{false, true} {
		for _, test := range []struct {
			name        string
			config      SMTArithState
			wantAtoms   int
			wantClauses int
			wantAxioms  uint64
		}{
			{"disabled", SMTArithState{}, 0, 0, 0},
			{"eager", SMTArithState{EagerEqAxioms: true}, 2, 3, 1},
			{"binary", SMTArithState{EagerEqAxioms: true, AddBinaryBounds: true}, 2, 4, 1},
		} {
			t.Run(fmt.Sprintf("%t/%s", integer, test.name), func(t *testing.T) {
				f := newSMTDenseEqualityFixture(t, integer, test.config)
				xv, err := f.theory.term(f.x)
				if err != nil {
					t.Fatal(err)
				}
				yv, err := f.theory.term(f.y)
				if err != nil {
					t.Fatal(err)
				}
				f.equality(t, f.x, f.y)
				if len(f.theory.atoms) != test.wantAtoms || len(f.state.testClauses()) != test.wantClauses || f.theory.adapter.NumAxioms() != test.wantAxioms {
					t.Fatal("dense eager equality policy", len(f.theory.atoms), len(f.state.testClauses()), f.theory.adapter.NumAxioms())
				}
				if f.theory.adapter.binary != test.config.AddBinaryBounds {
					t.Fatal("dense binary-bound adapter configuration")
				}
				astTestMust(t, f.theory.NewEquality(xv, yv))
				wantClauses := test.wantClauses
				if test.wantAxioms == 0 {
					wantClauses = 3
				}
				if len(f.theory.atoms) != 2 || len(f.state.testClauses()) != wantClauses || f.theory.adapter.NumAxioms() != 1 {
					t.Fatal("dense equality-event axioms", len(f.theory.atoms), len(f.state.testClauses()), f.theory.adapter.NumAxioms())
				}
				astTestMust(t, f.theory.NewDisequality(xv, yv))
				if len(f.theory.atoms) != 2 || len(f.state.testClauses()) != wantClauses || f.theory.adapter.NumAxioms() != 1 {
					t.Fatal("dense duplicate relation axioms", len(f.theory.atoms), len(f.state.testClauses()), f.theory.adapter.NumAxioms())
				}
			})
		}
	}
}

func TestSMTDenseDifferenceForcesDifferenceEqualityWithNumeral(t *testing.T) {
	for _, integer := range []bool{false, true} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			f := newSMTDenseEqualityFixture(t, integer, SMTArithState{})
			minusOne, err := bigTestArithNumeral(f.manager, big.NewRat(-1, 1), integer)
			astTestMust(t, err)
			negativeY, err := f.manager.MkFamilyApp(f.family, smtArithOpMul, nil, []ast.ASTHandle{minusOne, f.y})
			astTestMust(t, err)
			difference, err := f.manager.MkFamilyApp(f.family, smtArithOpAdd, nil, []ast.ASTHandle{f.x, negativeY})
			astTestMust(t, err)
			two, err := bigTestArithNumeral(f.manager, big.NewRat(2, 1), integer)
			astTestMust(t, err)
			f.equality(t, difference, two)
			if len(f.theory.atoms) != 2 || len(f.state.testClauses()) != 3 || f.theory.adapter.NumAxioms() != 1 {
				t.Fatal("forced difference equality axioms", len(f.theory.atoms), len(f.state.testClauses()), f.theory.adapter.NumAxioms())
			}
			first, second := f.theory.ext.atomText(0, 0), f.theory.ext.atomText(1, 0)
			if first != "2" || second != "-2" {
				t.Fatal("forced difference equality bounds", first, second)
			}
		})
	}
}

func TestSMTDenseDifferenceEqualityPhaseAndProofHint(t *testing.T) {
	f := newSMTDenseEqualityFixture(t, true, SMTArithState{EagerEqAxioms: true})
	astTestMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	if _, err := f.theory.term(f.x); err != nil {
		t.Fatal(err)
	}
	if _, err := f.theory.term(f.y); err != nil {
		t.Fatal(err)
	}
	equality, err := f.manager.MkEq(f.x, f.y)
	astTestMust(t, err)
	variable, err := f.state.MkBoolVar(equality)
	astTestMust(t, err)
	if err = f.state.setScopedTrueFirst(variable); err != nil {
		t.Fatal(err)
	}
	astTestMust(t, f.theory.InternalizeEquality(equality, variable))
	// arith_eq_adapter::mk_axioms passes its triangle-eq proof hint to each
	// clause; the clauses are checked below.
	if len(f.theory.atoms) != 2 || len(f.state.testClauses()) != 3 {
		t.Fatal("dense proof equality shape", len(f.theory.atoms), len(f.state.testClauses()))
	}
	for _, atom := range f.theory.atoms {
		data, err := f.state.BooleanData(atom.variable)
		astTestMust(t, err)
		if !data.TryTrueFirst() {
			t.Fatal("equality true-first phase was not copied to a dense bound")
		}
	}
	for _, entry := range f.state.testClauses() {
		justification, ok := entry.clause.Justification().(*SMTTheoryAxiomJustification)
		if !ok || len(justification.Parameters()) != 1 {
			t.Fatal("dense triangle proof hint missing")
		}
		name, err := f.manager.ParameterSymbol(justification.Parameters()[0])
		astTestMust(t, err)
		text, _, err := f.manager.Symbols.BareString(name)
		astTestMust(t, err)
		if text != "triangle-eq" {
			t.Fatal("dense triangle proof hint", text)
		}
	}
}

func TestSMTDenseDifferenceEqualityRestartRecreatesEagerDisabledAxioms(t *testing.T) {
	f := newSMTDenseEqualityFixture(t, true, SMTArithState{})
	xv, err := f.theory.term(f.x)
	astTestMust(t, err)
	yv, err := f.theory.term(f.y)
	astTestMust(t, err)
	astTestMust(t, f.state.InitializeTheorySearch())
	astTestMust(t, f.state.beginSearchPersistence(0, SMTParamsState{AgilityFactor: 0.9999, RestartAdaptive: true}))
	t.Cleanup(func() { astTestMust(t, f.state.endSearchPersistence()) })
	astTestMust(t, f.state.PushScope())
	astTestMust(t, f.theory.NewEquality(xv, yv))
	if len(f.theory.adapter.restart) != 1 || len(f.state.testClauses()) != 3 {
		t.Fatal("dense equality was not queued for restart", len(f.theory.adapter.restart), len(f.state.testClauses()))
	}
	astTestMust(t, f.state.PopScopes(1))
	if len(f.state.testClauses()) != 0 || len(f.theory.atoms) != 0 {
		t.Fatal("dense child equality survived backtracking", len(f.state.testClauses()), len(f.theory.atoms))
	}
	astTestMust(t, f.state.RestartTheories())
	if len(f.theory.adapter.restart) != 0 || f.theory.adapter.NumAxioms() != 2 || len(f.state.testClauses()) != 3 || len(f.theory.atoms) != 2 {
		t.Fatal("dense restart did not recreate equality axioms", len(f.theory.adapter.restart), f.theory.adapter.NumAxioms(), len(f.state.testClauses()), len(f.theory.atoms))
	}
}

// AUDIT.md smtcore_10-F2: internalize_atom matches (+ t (* -1 s)) and
// internalizes s and t as whole terms, so an attached offset term is a
// valid operand.
func TestSMTDenseDifferenceAtomAcceptsAttachedOffsetTerm(t *testing.T) {
	f := newSMTDenseEqualityFixture(t, true, SMTArithState{})
	m := f.manager
	one, err := bigTestArithNumeral(m, big.NewRat(1, 1), true)
	astTestMust(t, err)
	minusOne, err := bigTestArithNumeral(m, big.NewRat(-1, 1), true)
	astTestMust(t, err)
	zero, err := bigTestArithNumeral(m, new(big.Rat), true)
	astTestMust(t, err)
	offset, err := m.MkFamilyApp(f.family, smtArithOpAdd, nil, []ast.ASTHandle{one, f.x})
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(offset, false))
	astTestMust(t, f.state.Internalize(f.y, false))
	negY, err := m.MkFamilyApp(f.family, smtArithOpMul, nil, []ast.ASTHandle{minusOne, f.y})
	astTestMust(t, err)
	difference, err := m.MkFamilyApp(f.family, smtArithOpAdd, nil, []ast.ASTHandle{offset, negY})
	astTestMust(t, err)
	atom, err := m.MkFamilyApp(f.family, smtArithOpLE, nil, []ast.ASTHandle{difference, zero})
	astTestMust(t, err)
	astTestMust(t, f.state.Internalize(atom, true))
	if f.theory.unsupported || len(f.theory.atoms) != 1 {
		t.Fatal("dense atom over an offset term was rejected", f.theory.unsupported, len(f.theory.atoms))
	}
	target := f.state.context.enodes.get(offset).TheoryVar(f.family)
	source := f.state.context.enodes.get(f.y).TheoryVar(f.family)
	if a := f.theory.atoms[0]; a.target != int(target) || a.source != int(source) {
		t.Fatal("atom operands", a.source, a.target, source, target)
	}
}

// AUDIT.md smtcore_10-F5: new_eq_eh/new_diseq_eh go through
// arith_eq_adapter::mk_axioms, which orders the pair by owner id and builds
// (= t1 t2), t1 - t2 and the le/ge atoms itself.
func TestSMTDenseDifferenceRelationEventOrdersPairByOwnerID(t *testing.T) {
	f := newSMTDenseEqualityFixture(t, true, SMTArithState{})
	m := f.manager
	xv, err := f.theory.term(f.x)
	astTestMust(t, err)
	yv, err := f.theory.term(f.y)
	astTestMust(t, err)
	before := len(f.state.boolExpr)
	astTestMust(t, f.theory.NewEquality(yv, xv))
	equality, err := m.MkEq(f.x, f.y)
	astTestMust(t, err)
	astTestPin(t, m, equality)
	if _, found := f.state.BoolVar(equality); !found {
		t.Fatal("the equality atom is not (= x y)")
	}
	reversed, err := m.MkEq(f.y, f.x)
	astTestMust(t, err)
	astTestPin(t, m, reversed)
	if _, found := f.state.BoolVar(reversed); found {
		t.Fatal("an unordered (= y x) atom was internalized")
	}
	if got := len(f.state.boolExpr) - before; got != 3 {
		t.Fatal("expected the equality and the two bound atoms", got)
	}
	minusOne, err := bigTestArithNumeral(m, big.NewRat(-1, 1), true)
	astTestMust(t, err)
	negativeY, err := m.MkFamilyApp(f.family, smtArithOpMul, nil, []ast.ASTHandle{minusOne, f.y})
	astTestMust(t, err)
	difference, err := m.MkFamilyApp(f.family, smtArithOpAdd, nil, []ast.ASTHandle{f.x, negativeY})
	astTestMust(t, err)
	zero, err := bigTestArithNumeral(m, new(big.Rat), true)
	astTestMust(t, err)
	lower, err := m.MkFamilyApp(f.family, smtArithOpLE, nil, []ast.ASTHandle{difference, zero})
	astTestMust(t, err)
	astTestPin(t, m, lower)
	if _, found := f.state.BoolVar(lower); !found {
		t.Fatal("the lower bound is not (<= (+ x (* -1 y)) 0)")
	}
	count := len(f.state.boolExpr)
	astTestMust(t, f.theory.NewDisequality(xv, yv))
	if len(f.state.boolExpr) != count || f.theory.adapter.NumAxioms() != 1 {
		t.Fatal("an already processed pair created atoms again", len(f.state.boolExpr)-count, f.theory.adapter.NumAxioms())
	}
}

// AUDIT.md smtcore_10-F14: restart_eh calls mk_axioms on the stored enodes,
// whose dense variables may have been popped; the bound atoms recreate them.
func TestSMTDenseDifferenceRestartRecreatesVariables(t *testing.T) {
	f := newSMTDenseEqualityFixture(t, true, SMTArithState{})
	astTestMust(t, f.state.Internalize(f.x, false))
	astTestMust(t, f.state.Internalize(f.y, false))
	astTestMust(t, f.state.InitializeTheorySearch())
	astTestMust(t, f.state.beginSearchPersistence(0, SMTParamsState{AgilityFactor: 0.9999, RestartAdaptive: true}))
	t.Cleanup(func() { astTestMust(t, f.state.endSearchPersistence()) })
	astTestMust(t, f.state.PushScope())
	xv, err := f.theory.term(f.x)
	astTestMust(t, err)
	yv, err := f.theory.term(f.y)
	astTestMust(t, err)
	astTestMust(t, f.theory.NewEquality(xv, yv))
	if len(f.theory.adapter.restart) != 1 || len(f.state.testClauses()) != 3 {
		t.Fatal("dense equality was not queued for restart", len(f.theory.adapter.restart), len(f.state.testClauses()))
	}
	astTestMust(t, f.state.PopScopes(1))
	if f.theory.IsAttachedToVariable(f.state.context.enodes.get(f.x)) || f.theory.IsAttachedToVariable(f.state.context.enodes.get(f.y)) {
		t.Fatal("dense variables survived the pop")
	}
	astTestMust(t, f.state.RestartTheories())
	if len(f.theory.adapter.restart) != 0 || len(f.state.testClauses()) != 3 || len(f.theory.atoms) != 2 {
		t.Fatal("dense restart did not recreate the axioms", len(f.theory.adapter.restart), len(f.state.testClauses()), len(f.theory.atoms))
	}
}
