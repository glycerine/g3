package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// An arrangement may require the overlap term. This fixture forces exactly
// that consequence before checking, while using the actual String callbacks.
type smtStringOverlapTestTheory struct {
	*SMTStringTheory
	forceOverlap bool
}

func (t *smtStringOverlapTestTheory) AddTheoryAssumptions(input []ast.ASTHandle) ([]ast.ASTHandle, error) {
	all, err := t.SMTStringTheory.AddTheoryAssumptions(input)
	if err != nil || !t.forceOverlap {
		return all, err
	}
	term, err := t.state.manager.RefGet(t.overlapAssumption)
	if err != nil {
		return all, err
	}
	return all, t.state.Assert(term, ast.ASTHandle{}, 0)
}

func TestSMTStringOverlapAssumptionCoreDowngradeAndFreshChecks(t *testing.T) {
	for _, force := range []bool{false, true} {
		m := astTestManager(t)
		k := smtTestKernel(t, m)
		family, err := m.SequenceFamilyID()
		astTestMust(t, err)
		owner, err := NewSMTStringTheory(family, nil)
		astTestMust(t, err)
		theory := &smtStringOverlapTestTheory{SMTStringTheory: owner, forceOverlap: force}
		astTestMust(t, k.internalizer.RegisterTheory(theory))
		a := smtRelevancyBool(t, m, "string-independent-unsat")
		notA, err := m.MkNot(a)
		astTestMust(t, err)
		astTestPin(t, m, notA)
		if !force {
			astTestMust(t, k.Assert(a, ast.ASTHandle{}))
		}
		previous := ast.ASTHandle{}
		for attempt := 0; attempt < 2; attempt++ {
			var result lbool.LBool
			if force {
				result, err = k.Check()
			} else {
				result, err = k.CheckAssumptions([]ast.ASTHandle{notA})
			}
			astTestMust(t, err)
			want := lbool.False
			if force {
				want = lbool.Undef
			}
			if result != want || k.internalizer.scopeLevel != 0 {
				t.Fatal("overlap core was not distinguished from an independent contradiction", force, result)
			}
			term, err := m.RefGet(owner.overlapAssumption)
			astTestMust(t, err)
			if term == previous || term.IsNull() {
				t.Fatal("overlap assumption was not refreshed on the next check", force)
			}
			previous = term
			if force {
				negative, err := m.MkNot(term)
				astTestMust(t, err)
				if len(k.UnsatCore()) != 1 || k.UnsatCore()[0] != negative {
					t.Fatal("unknown result lost its actual overlap core", k.UnsatCore())
				}
			} else if len(k.UnsatCore()) != 1 || k.UnsatCore()[0] != notA {
				t.Fatal("independent unsat core was polluted by an unused overlap assumption", k.UnsatCore())
			}
		}
	}
}

func TestSMTStringOverlapValidationUsesEnodeIdentity(t *testing.T) {
	m := astTestManager(t)
	theory, state := stringTheoryFixture(t, m)
	assumptions, err := theory.AddTheoryAssumptions(nil)
	astTestMust(t, err)
	astTestPin(t, m, assumptions[0])
	other := smtRelevancyBool(t, m, "different-overlap-assumption")
	astTestMust(t, state.Internalize(assumptions[0], false))
	astTestMust(t, state.Internalize(other, false))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.addEquality(state.context.enodes.get(assumptions[0]), state.context.enodes.get(other), NewSMTEqualityAxiom()))
	if state.context.enodes.get(assumptions[0]).Root() != state.context.enodes.get(other).Root() {
		t.Fatal("identity regression did not establish congruent Boolean terms")
	}
	if result, err := theory.ValidateUnsatCore([]ast.ASTHandle{other}); err != nil || result != lbool.False {
		t.Fatal("String core validation compared roots instead of source enode identity", result, err)
	}
	if result, err := theory.ValidateUnsatCore(assumptions); err != nil || result != lbool.Undef {
		t.Fatal("exact overlap assumption was not recognized", result, err)
	}
	astTestMust(t, state.PopScopes(1))
}
