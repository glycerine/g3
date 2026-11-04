package base_test

// Source: dl_rule.cpp mk_horn_rule holds the rule and proof intermediates
// while deriving a proof for a hoisted compound predicate.
import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleFormulaProofAllocationFailureReleasesIntermediates(t *testing.T) {
	failures := 0
	lastFailure := int64(-1)
	completed := false
	for limit := int64(0); limit < 80; limit++ {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			f := newRuleAnalysisFixture(t)
			m := f.manager
			one, err := m.ArithmeticIntegerNumeralText("1")
			ruleMust(t, err)
			sum, err := m.MkArithmeticAdd(f.x, one)
			ruleMust(t, err)
			formula, err := m.MkApp(f.pred, []ast.ASTHandle{sum})
			ruleMust(t, err)
			ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
			proof, err := m.MkProofAsserted(formula)
			ruleMust(t, err)
			proofRef, err := m.NewRef(proof)
			ruleMust(t, err)
			defer func() { ruleMust(t, m.DelRef(&proofRef)) }()
			ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
			before := m.Stats()
			m.Budget().FailAfter(limit)
			rule, cause := f.rules.NewRuleFromFormulaWithProof(formula, proof, "allocation-proof")
			m.Budget().FailAfter(-1)
			if cause != nil {
				if rule != nil || !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("failure", rule, cause)
				}
				failures++
				lastFailure = limit
			} else {
				completed = true
				ruleMust(t, rule.Close())
			}
			mode, err := m.ProofMode()
			ruleMust(t, err)
			if mode != ast.ASTProofDisabled {
				t.Fatal("proof mode was not restored", mode)
			}
			after := m.Stats()
			if after.LiveReferences != before.LiveReferences {
				t.Fatal("proof conversion leaked references", before, after)
			}
			if cause != nil {
				rule, err = f.rules.NewRuleFromFormulaWithProof(formula, proof, "allocation-proof")
				ruleMust(t, err)
				translated, found, err := rule.Proof()
				ruleMust(t, err)
				if !found {
					t.Fatal("retry lost proof")
				}
				fact, err := m.GetProofFact(translated)
				ruleMust(t, err)
				result, err := rule.ToFormula()
				ruleMust(t, err)
				got, err := m.RefGet(result)
				ruleMust(t, err)
				if fact != got {
					t.Fatal("retry proof fact differs from rule formula")
				}
				ruleMust(t, m.DelRef(&result))
				ruleMust(t, rule.Close())
			}
		})
		if completed {
			break
		}
	}
	if !completed {
		t.Fatal("failure campaign did not reach success")
	}
	t.Logf("injected %d failures through allocation %d", failures, lastFailure)
	if failures == 0 {
		t.Fatal("failure campaign did not reach any allocations")
	}
}
