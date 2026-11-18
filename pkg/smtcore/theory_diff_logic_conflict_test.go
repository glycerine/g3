package smtcore

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTDifferenceContextShortcutConflictProof(t *testing.T) {
	for _, integer := range []bool{false, true} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			m := astTestManager(t)
			astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			theory, err := NewSMTMixedDiffTheory(family)
			astTestMust(t, err)
			astTestMust(t, state.RegisterTheory(theory))
			sortKind := int32(0)
			if integer {
				sortKind = 1
			}
			sort, err := m.MkFamilySort(family, sortKind, nil)
			astTestMust(t, err)
			variables := []ast.ASTHandle{}
			for i := 0; i < 5; i++ {
				x, err := m.MkConst(astTestSymbol(t, m, fmt.Sprint("diff-shortcut-", i)), sort)
				astTestMust(t, err)
				astTestMust(t, state.Internalize(x, false))
				variables = append(variables, x)
			}
			assert := func(source, target int, weight int64) SMTLiteral {
				minusOne, err := bigTestArithNumeral(m, big.NewRat(-1, 1), integer)
				astTestMust(t, err)
				negative, err := m.MkFamilyApp(family, smtArithOpMul, nil, []ast.ASTHandle{minusOne, variables[source]})
				astTestMust(t, err)
				difference, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{variables[target], negative})
				astTestMust(t, err)
				n, err := bigTestArithNumeral(m, big.NewRat(weight, 1), integer)
				astTestMust(t, err)
				formula, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{difference, n})
				astTestMust(t, err)
				astTestMust(t, state.Internalize(formula, true))
				literal, err := state.Literal(formula)
				astTestMust(t, err)
				astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
				astTestMust(t, state.PropagateTheoryAssignments())
				astTestMust(t, theory.Propagate())
				return literal
			}
			for i := 0; i < 3; i++ {
				assert(i, i+1, 0)
			}
			lastChain := assert(3, 4, 0)
			assert(1, 3, 1)
			full := assert(0, 3, 2)
			closing := assert(4, 0, -3)
			original, err := theory.realGraph.NegativeCycleEdges()
			astTestMust(t, err)
			if len(original) != 5 {
				t.Fatal("missing original five-edge parent cycle", original)
			}
			conflict, found := state.Conflict()
			if !found {
				t.Fatal("missing shortcut arithmetic conflict")
			}
			owner, _ := conflict.Justification()
			extended, ok := owner.(*SMTExtTheoryConflictJustification)
			if !ok {
				t.Fatal("missing extended difference conflict")
			}
			if !reflect.DeepEqual(extended.Literals, []SMTLiteral{closing, lastChain, full}) {
				t.Fatal("conflict did not use the shorter source antecedents", extended.Literals)
			}
			parameters := extended.Parameters()
			if len(parameters) != 5 {
				t.Fatal("shortened proof retained obsolete parent-edge coefficients", len(parameters))
			}
			for _, parameter := range parameters[1:] {
				want, err := bigTestParameter(m, big.NewRat(1, 1))
				astTestMust(t, err)
				equal, err := m.ParameterEqual(parameter, want)
				astTestMust(t, err)
				astTestMust(t, m.DelParameter(&want))
				if !equal {
					t.Fatal("binary shortcut lost coefficient one")
				}
			}
			resolver, err := NewSMTConflictResolver(state)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, resolver.Close()) })
			proof, err := resolver.conflictProof(conflict, SMTNullLiteral, nil)
			astTestMust(t, err)
			if proof.IsNull() {
				t.Fatal("shortened conflict did not build a proof")
			}
		})
	}
}

func TestSMTDifferenceConflictParameterAllocationFrontiers(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 128 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			f := newSMTRealDiffFixture(t)
			m, state := f.manager, f.state
			astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			expression := smtRelevancyBool(t, m, "conflict-frontier-p")
			astTestMust(t, state.Internalize(expression, true))
			literal, err := state.Literal(expression)
			astTestMust(t, err)
			astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
			astTestMust(t, state.PushScope())
			before := m.Stats()
			m.Budget().FailAfter(stop)
			err = f.theory.setNegativeCycleConflict([]SMTLiteral{literal})
			m.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
			} else {
				if !errors.Is(err, arena.ErrInjectedFailure) {
					t.Fatal("unexpected allocation refusal", err)
				}
				after := m.Stats()
				if state.Inconsistent() || after.LiveParameters != before.LiveParameters || after.LiveReferences != before.LiveReferences {
					t.Fatal("failed conflict published state or leaked an owner", before, after)
				}
				astTestMust(t, f.theory.setNegativeCycleConflict([]SMTLiteral{literal}))
			}
			conflict, found := state.Conflict()
			if !found {
				t.Fatal("completed conflict missing")
			}
			owner, _ := conflict.Justification()
			extended, ok := owner.(*SMTExtTheoryConflictJustification)
			if !ok || len(extended.Parameters()) != 3 {
				t.Fatal("conflict parameter shape")
			}
			astTestMust(t, state.PopScopes(1))
			if m.Stats().LiveParameters != before.LiveParameters {
				t.Fatal("conflict parameters survived scope pop")
			}
		})
		if t.Failed() {
			t.Fatalf("conflict frontier %d", stop)
		}
	}
	if !succeeded {
		t.Fatal("no successful conflict allocation frontier")
	}
}

func TestSMTUTVPIConflictParametersFollowScopeLifetime(t *testing.T) {
	m := astTestManager(t)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	_, _, state := smtInternalizerFixture(t, m, nil)
	theory, err := NewSMTUTVPITheory(family)
	astTestMust(t, err)
	astTestMust(t, state.RegisterTheory(theory))
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "utvpi-scoped-conflict-x"), sort)
	astTestMust(t, err)
	literals := []SMTLiteral{}
	for i, op := range []int32{smtArithOpLE, smtArithOpGE} {
		n, err := bigTestArithNumeral(m, big.NewRat(int64(i), 1), false)
		astTestMust(t, err)
		formula, err := m.MkFamilyApp(family, op, nil, []ast.ASTHandle{x, n})
		astTestMust(t, err)
		astTestMust(t, state.Internalize(formula, true))
		lit, err := state.Literal(formula)
		astTestMust(t, err)
		literals = append(literals, lit)
	}
	before := m.Stats()
	astTestMust(t, state.PushScope())
	for _, lit := range literals {
		astTestMust(t, state.AssignLiteral(lit, NewSMTBooleanAxiom()))
		astTestMust(t, state.PropagateTheoryAssignments())
		astTestMust(t, theory.Propagate())
	}
	conflict, found := state.Conflict()
	if !found {
		t.Fatal("missing unary contradiction")
	}
	owner, _ := conflict.Justification()
	extended, ok := owner.(*SMTExtTheoryConflictJustification)
	if !ok || len(extended.Parameters()) != 3 {
		t.Fatal("missing weighted conflict parameters")
	}
	astTestMust(t, state.PopScopes(1))
	if m.Stats().LiveParameters != before.LiveParameters || len(extended.Parameters()) != 0 {
		t.Fatal("UTVPI conflict parameters survived backtracking")
	}
}

// AUDIT.md smtcore_10-F8: set_neg_cycle_conflict dumps the conflict with
// display_lemma_as_smt_problem under smt.arith.dump_lemmas, with logic
// QF_LIA while every atom is over Int (m_is_lia) and QF_LRA otherwise.
func TestSMTDifferenceConflictDumpsLemma(t *testing.T) {
	for _, integer := range []bool{true, false} {
		t.Run(fmt.Sprint(integer), func(t *testing.T) {
			t.Chdir(t.TempDir())
			oldID := smtLemmaID
			t.Cleanup(func() { smtLemmaID = oldID })
			smtLemmaID = 0
			m := astTestManager(t)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			_, _, state := smtInternalizerFixture(t, m, nil)
			theory, err := NewSMTMixedDiffTheory(family)
			astTestMust(t, err)
			astTestMust(t, theory.ConfigureArithmetic(SMTArithState{DumpLemmas: true}))
			astTestMust(t, state.RegisterTheory(theory))
			sortKind := int32(0)
			if integer {
				sortKind = 1
			}
			sort, err := m.MkFamilySort(family, sortKind, nil)
			astTestMust(t, err)
			var variables []ast.ASTHandle
			for i := 0; i < 2; i++ {
				x, err := m.MkConst(astTestSymbol(t, m, fmt.Sprint("diff-dump-", i)), sort)
				astTestMust(t, err)
				astTestMust(t, state.Internalize(x, false))
				variables = append(variables, x)
			}
			assert := func(source, target int, weight int64) {
				minusOne, err := bigTestArithNumeral(m, big.NewRat(-1, 1), integer)
				astTestMust(t, err)
				negative, err := m.MkFamilyApp(family, smtArithOpMul, nil, []ast.ASTHandle{minusOne, variables[source]})
				astTestMust(t, err)
				difference, err := m.MkFamilyApp(family, smtArithOpAdd, nil, []ast.ASTHandle{variables[target], negative})
				astTestMust(t, err)
				n, err := bigTestArithNumeral(m, big.NewRat(weight, 1), integer)
				astTestMust(t, err)
				formula, err := m.MkFamilyApp(family, smtArithOpLE, nil, []ast.ASTHandle{difference, n})
				astTestMust(t, err)
				astTestMust(t, state.Internalize(formula, true))
				literal, err := state.Literal(formula)
				astTestMust(t, err)
				astTestMust(t, state.AssignLiteral(literal, NewSMTBooleanAxiom()))
				astTestMust(t, state.PropagateTheoryAssignments())
				astTestMust(t, theory.Propagate())
			}
			assert(0, 1, 0)
			assert(1, 0, -1)
			if !state.Inconsistent() || smtLemmaID != 1 {
				t.Fatal("negative cycle did not publish and dump", state.Inconsistent(), smtLemmaID)
			}
			text, err := os.ReadFile("lemma_0.smt2")
			astTestMust(t, err)
			want := "(set-logic QF_LRA)"
			if integer {
				want = "(set-logic QF_LIA)"
			}
			if !strings.Contains(string(text), want) {
				t.Fatal("dumped lemma logic", string(text))
			}
		})
	}
}
