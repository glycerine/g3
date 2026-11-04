package base_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// Source: dl_rule.cpp mk_rule_rewrite_proof applies the rewrite to the old
// proof and transfers the resulting modus-ponens proof to the new rule.
func TestRuleRewriteProofReferenceFailureReleasesIntermediates(t *testing.T) {
	for _, free := range []int{1, 2} {
		t.Run(fmt.Sprint(free), func(t *testing.T) { testRewriteProofReferenceFailure(t, free) })
	}
}

func testRewriteProofReferenceFailure(t *testing.T, free int) {
	f := newRuleSetFixture(t, "P", "Q")
	m := f.manager
	oldRule, err := f.rules.NewRule(f.app["P"], nil, nil, "rewrite-old")
	ruleMust(t, err)
	defer func() { ruleMust(t, oldRule.Close()) }()
	target, err := f.rules.NewRule(f.app["Q"], nil, nil, "rewrite-new")
	ruleMust(t, err)
	defer func() { ruleMust(t, target.Close()) }()
	ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := m.MkProofAsserted(f.app["P"])
	ruleMust(t, err)
	ruleMust(t, oldRule.SetProof(proof))
	ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	// Initialize proof declarations before measuring temporary ownership.
	ruleMust(t, f.rules.MkRuleRewriteProof(oldRule, target))
	ruleMust(t, target.SetProof(ast.ASTHandle{}))
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
	for i := 0; i < free; i++ {
		last := len(pressure) - 1
		ruleMust(t, m.DelRef(&pressure[last]))
		pressure = pressure[:last]
	}
	before := m.Stats()
	m.Budget().FailAfter(0)
	cause := f.rules.MkRuleRewriteProof(oldRule, target)
	m.Budget().FailAfter(-1)
	if !errors.Is(cause, arena.ErrInjectedFailure) {
		t.Fatal("expected proof reference failure", cause)
	}
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("rewrite proof leaked", before, after)
	}
	ruleMust(t, f.rules.MkRuleRewriteProof(oldRule, target))
	got, found, err := target.Proof()
	ruleMust(t, err)
	if !found {
		t.Fatal("retry lost proof")
	}
	fact, err := m.GetProofFact(got)
	ruleMust(t, err)
	if fact != f.app["Q"] {
		t.Fatal("retry proof establishes wrong fact", fact)
	}
}
