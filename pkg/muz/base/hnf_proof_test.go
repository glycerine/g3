package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
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

func TestRuleManagerAddsProofForAlreadyHornFormula(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	formula, err := f.manager.MkImplies(f.app["Q"], f.app["P"])
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "already-horn-proof"); err != nil {
		t.Fatal(err)
	}
	if count, e := set.Len(); e != nil || count != 1 {
		t.Fatal("already-Horn rule count", count, e)
	}
	rule, err := set.Rule(0)
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	translated, found, err := rule.Proof()
	ruleMust(t, err)
	if !found {
		t.Fatal("already-Horn rule lost its proof")
	}
	checker, err := proofs.NewChecker(f.manager)
	ruleMust(t, err)
	valid, side, err := checker.Check(translated)
	ruleMust(t, err)
	if !valid {
		t.Fatal("already-Horn proof failed proof checking")
	}
	ruleMust(t, checker.ReleaseSideConditions(side))
}

func TestRuleManagerAddsProofsForHNFDefinitionIntroduction(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q", "R", "S", "H")
	firstDisjunction, err := f.manager.MkOr(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	secondDisjunction, err := f.manager.MkOr(f.app["R"], f.app["S"])
	ruleMust(t, err)
	body, err := f.manager.MkAnd(firstDisjunction, secondDisjunction)
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
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "definition-proof"); err != nil {
		t.Fatal(err)
	}
	if count, e := set.Len(); e != nil || count != 5 {
		t.Fatal("normalized HNF rule count", count, e)
	}
	if mode, e := f.manager.ProofMode(); e != nil || mode != ast.ASTProofDisabled {
		t.Fatal("proof mode was not restored", mode, e)
	}
	checker, err := proofs.NewChecker(f.manager)
	ruleMust(t, err)
	for i := 0; i < 5; i++ {
		rule, e := set.Rule(i)
		ruleMust(t, e)
		fact, e := rule.ToFormula()
		ruleMust(t, e)
		defer func() { ruleMust(t, f.manager.DelRef(&fact)) }()
		factHandle, e := f.manager.RefGet(fact)
		ruleMust(t, e)
		translated, found, e := rule.Proof()
		ruleMust(t, e)
		if !found {
			t.Fatal("HNF definition rule lost its proof", i)
		}
		proved, e := f.manager.GetProofFact(translated)
		ruleMust(t, e)
		if proved != factHandle {
			t.Fatal("HNF definition proof fact does not match rule", i, proved, factHandle)
		}
		valid, side, e := checker.Check(translated)
		ruleMust(t, e)
		if !valid {
			t.Fatal("HNF definition proof failed proof checking", i)
		}
		ruleMust(t, checker.ReleaseSideConditions(side))
		ruleMust(t, rule.Close())
	}
}

func TestRuleManagerAddsProofsForQuantifiedDisjunctiveHornSplit(t *testing.T) {
	f := newRuleSetFixture(t)
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	arithmetic, err := f.manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := f.manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	for _, name := range []string{"P", "Q", "R"} {
		symbol, e := f.manager.Symbols.Intern("quantified-split-" + name)
		ruleMust(t, e)
		f.decl[name], e = f.manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, e)
		ruleMust(t, f.rules.RegisterPredicate(f.decl[name]))
	}
	x, err := f.manager.MkVar(0, integer)
	ruleMust(t, err)
	applications := make(map[string]ast.ASTHandle)
	for _, name := range []string{"P", "Q", "R"} {
		applications[name], err = f.manager.MkApp(f.decl[name], []ast.ASTHandle{x})
		ruleMust(t, err)
	}
	disjunction, err := f.manager.MkOr(applications["Q"], applications["R"])
	ruleMust(t, err)
	body, err := f.manager.MkImplies(disjunction, applications["P"])
	ruleMust(t, err)
	name, err := f.manager.Symbols.Intern("x")
	ruleMust(t, err)
	formula, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{name}, Body: body})
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	if err = f.rules.AddFormulaWithProof(set, formula, proof, "quantified-split-proof"); err != nil {
		t.Fatal(err)
	}
	if count, e := set.Len(); e != nil || count != 2 {
		t.Fatal("normalized quantified rule count", count, e)
	}
	checker, err := proofs.NewChecker(f.manager)
	ruleMust(t, err)
	for i := 0; i < 2; i++ {
		rule, e := set.Rule(i)
		ruleMust(t, e)
		translated, found, e := rule.Proof()
		ruleMust(t, e)
		if !found {
			t.Fatal("quantified normalized Horn rule lost its proof", i)
		}
		valid, side, e := checker.Check(translated)
		ruleMust(t, e)
		if !valid {
			t.Fatal("quantified normalized Horn proof failed proof checking", i)
		}
		ruleMust(t, checker.ReleaseSideConditions(side))
		ruleMust(t, rule.Close())
	}
}
