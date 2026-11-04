package base_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// Source mk_rule_asserted_proof stores the new proof on the rule while its
// formula is still rooted. Failed publication must not strand the proof.
func TestRuleAssertedProofReferenceFailureReleasesProof(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	m := f.manager
	rule, err := f.rules.NewRule(f.app["P"], nil, nil, "asserted-proof-failure")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := m.MkProofAsserted(f.app["P"])
	ruleMust(t, err)
	ref, err := m.NewRef(proof)
	ruleMust(t, err)
	ruleMust(t, m.DelRef(&ref))
	ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	var pressure []ast.ASTRef
	defer func() {
		for i := range pressure {
			ruleMust(t, m.DelRef(&pressure[i]))
		}
	}()
	for {
		m.Budget().FailAfter(0)
		pin, cause := m.NewRef(ast.ASTHandle{})
		m.Budget().FailAfter(-1)
		if cause != nil {
			if !errors.Is(cause, arena.ErrInjectedFailure) {
				t.Fatal(cause)
			}
			break
		}
		pressure = append(pressure, pin)
		if len(pressure) > 4096 {
			t.Fatal("reference page did not fill")
		}
	}
	last := len(pressure) - 1
	ruleMust(t, m.DelRef(&pressure[last]))
	pressure = pressure[:last]
	before := m.Stats()
	m.Budget().FailAfter(0)
	cause := f.rules.MkRuleAssertedProof(rule)
	m.Budget().FailAfter(-1)
	if !errors.Is(cause, arena.ErrInjectedFailure) {
		t.Fatal("expected proof reference failure", cause)
	}
	after := m.Stats()
	if after.LiveReferences != before.LiveReferences || after.LiveNodes != before.LiveNodes {
		t.Fatal("asserted proof leaked", before, after)
	}
	ruleMust(t, f.rules.MkRuleAssertedProof(rule))
	_, found, err := rule.Proof()
	ruleMust(t, err)
	if !found {
		t.Fatal("retry did not attach proof")
	}
}
