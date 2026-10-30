package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleManagerAddsProofsForDisjunctiveHornSplit(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R")
	disjunction, err := f.manager.MkOr(f.app["Q"], f.app["R"])
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(disjunction, f.app["P"])
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "split-proof"); err != nil {
		t.Fatal(err)
	}
	if mode, e := f.manager.ProofMode(); e != nil || mode != ast.ASTProofDisabled {
		t.Fatal("proof mode was not restored", mode, e)
	}
	if count, e := set.Len(); e != nil || count != 2 {
		t.Fatal("normalized rule count", count, e)
	}
	checker, err := proofs.NewChecker(f.manager)
	ruleMust(t, err)
	for i := uint32(0); i < 2; i++ {
		rule, e := set.Rule(int(i))
		ruleMust(t, e)
		fact, e := rule.ToFormula()
		ruleMust(t, e)
		defer func() { ruleMust(t, f.manager.DelRef(&fact)) }()
		factHandle, e := f.manager.RefGet(fact)
		ruleMust(t, e)
		translated, found, e := rule.Proof()
		ruleMust(t, e)
		if !found {
			t.Fatal("normalized Horn rule lost its proof", i)
		}
		proved, e := f.manager.GetProofFact(translated)
		ruleMust(t, e)
		if proved != factHandle {
			t.Fatal("proof fact does not match normalized rule", i, proved, factHandle)
		}
		valid, side, e := checker.Check(translated)
		ruleMust(t, e)
		if !valid {
			t.Fatal("normalized Horn proof failed proof checking", i)
		}
		ruleMust(t, checker.ReleaseSideConditions(side))
		ruleMust(t, rule.Close())
	}
}

func TestRuleManagerRejectsUnsupportedHNFProofBeforeAddingRules(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R", "H")
	disjunction, err := f.manager.MkOr(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	body, err := f.manager.MkAnd(disjunction, f.app["R"])
	ruleMust(t, err)
	formula, err := f.manager.MkImplies(body, f.app["H"])
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "unsupported-proof"); err == nil {
		t.Fatal("unsupported HNF definition proof was accepted")
	}
	if count, e := set.Len(); e != nil || count != 0 {
		t.Fatal("failed proof ingestion published partial rules", count, e)
	}
	if mode, e := f.manager.ProofMode(); e != nil || mode != ast.ASTProofDisabled {
		t.Fatal("proof mode was not restored after rejection", mode, e)
	}
}
