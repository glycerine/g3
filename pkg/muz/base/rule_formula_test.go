package base_test

// Source: src/muz/base/dl_rule.cpp already-Horn extraction, compound
// predicate hoisting, negated predicate recognition, and HNF refusal.

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestRuleManagerBuildsHornRuleFromQuantifiedFormula(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	manager, err := context.Manager()
	ruleMust(t, err)
	rules, err := base.NewRuleManager(manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rules.Close()) })
	boolean, err := manager.MkBoolSort()
	ruleMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	declarations := make(map[string]ast.ASTHandle)
	for _, name := range []string{"formula-P", "formula-Q"} {
		symbol, e := manager.Symbols.Intern(name)
		ruleMust(t, e)
		declarations[name], e = manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, e)
		ruleMust(t, rules.RegisterPredicate(declarations[name]))
	}
	x, err := manager.MkVar(0, integer)
	ruleMust(t, err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	ruleMust(t, err)
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	ruleMust(t, err)
	sum, err := manager.MkArithmeticAdd(x, one)
	ruleMust(t, err)
	pSum, err := manager.MkApp(declarations["formula-P"], []ast.ASTHandle{sum})
	ruleMust(t, err)
	qX, err := manager.MkApp(declarations["formula-Q"], []ast.ASTHandle{x})
	ruleMust(t, err)
	lower, err := manager.MkArithmeticGE(x, zero)
	ruleMust(t, err)
	antecedent, err := manager.MkAnd(pSum, lower)
	ruleMust(t, err)
	implication, err := manager.MkImplies(antecedent, qX)
	ruleMust(t, err)
	name, err := manager.Symbols.Intern("x")
	ruleMust(t, err)
	formula, err := manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{name}, Body: implication})
	ruleMust(t, err)
	formulaRef, err := manager.NewRef(formula)
	ruleMust(t, err)
	defer func() { ruleMust(t, manager.DelRef(&formulaRef)) }()

	rule, err := rules.NewRuleFromFormula(formula, "from-formula")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	if positive, err := rule.PositiveTailSize(); err != nil || positive != 1 {
		t.Fatal("positive tail count", positive, err)
	}
	if predicates, err := rule.UninterpretedTailSize(); err != nil || predicates != 1 {
		t.Fatal("predicate tail count", predicates, err)
	}
	if size, err := rule.TailSize(); err != nil || size != 3 {
		t.Fatal("hoisted tail size", size, err)
	}
	predicate, negated, err := rule.Tail(0)
	ruleMust(t, err)
	if negated {
		t.Fatal("positive predicate became negated")
	}
	argument, err := manager.GetArg(predicate, 0)
	ruleMust(t, err)
	if index, err := manager.GetVarIndex(argument); err != nil || index != 1 {
		t.Fatal("hoisted predicate variable", index, err)
	}
	equality, negated, err := rule.Tail(1)
	ruleMust(t, err)
	if operator, err := manager.GetDeclKind(equality); err != nil || operator != ast.ASTOpEq || negated {
		t.Fatal("hoisted equality", operator, negated, err)
	}
	if trailing, _, err := rule.Tail(2); err != nil || trailing != lower {
		t.Fatal("interpreted tail reversal", trailing, lower, err)
	}
}

func TestRuleManagerBuildsHornRuleAndCarriesFormulaProof(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	manager, err := context.Manager()
	ruleMust(t, err)
	rules, err := base.NewRuleManager(manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rules.Close()) })
	boolean, err := manager.MkBoolSort()
	ruleMust(t, err)
	arithmetic, err := manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	decl := make(map[string]ast.ASTHandle)
	for _, name := range []string{"proof-formula-P", "proof-formula-Q"} {
		symbol, e := manager.Symbols.Intern(name)
		ruleMust(t, e)
		decl[name], e = manager.MkFuncDecl(symbol, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		ruleMust(t, e)
		ruleMust(t, rules.RegisterPredicate(decl[name]))
	}
	functionName, err := manager.Symbols.Intern("proof-formula-f")
	ruleMust(t, err)
	function, err := manager.MkFuncDecl(functionName, []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	x, err := manager.MkVar(0, integer)
	ruleMust(t, err)
	fX, err := manager.MkApp(function, []ast.ASTHandle{x})
	ruleMust(t, err)
	pFX, err := manager.MkApp(decl["proof-formula-P"], []ast.ASTHandle{fX})
	ruleMust(t, err)
	qX, err := manager.MkApp(decl["proof-formula-Q"], []ast.ASTHandle{x})
	ruleMust(t, err)
	implication, err := manager.MkImplies(pFX, qX)
	ruleMust(t, err)
	name, err := manager.Symbols.Intern("proof-formula-x")
	ruleMust(t, err)
	formula, err := manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []usymbol.Symbol{name}, Body: implication})
	ruleMust(t, err)
	if err = manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
		t.Fatal(err)
	}
	proof, err := manager.MkProofAsserted(formula)
	ruleMust(t, err)
	if err = manager.ToggleProofMode(ast.ASTProofDisabled); err != nil {
		t.Fatal(err)
	}
	rule, err := rules.NewRuleFromFormulaWithProof(formula, proof, "proof-formula")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	if mode, e := manager.ProofMode(); e != nil || mode != ast.ASTProofDisabled {
		t.Fatal("formula normalization did not restore proof mode", mode, e)
	}
	translated, found, err := rule.Proof()
	ruleMust(t, err)
	if !found {
		t.Fatal("normalized rule lost its proof")
	}
	proofFact, err := manager.GetProofFact(translated)
	ruleMust(t, err)
	resultFormula, err := rule.ToFormula()
	ruleMust(t, err)
	defer func() { ruleMust(t, manager.DelRef(&resultFormula)) }()
	result, err := manager.RefGet(resultFormula)
	ruleMust(t, err)
	equal, err := manager.AreEqual(proofFact, result)
	ruleMust(t, err)
	if !equal || result == formula {
		t.Fatal("translated proof does not establish the rewritten rule formula", equal, result, formula)
	}
}

func TestRuleManagerFormulaNegationAndNonHornRefusal(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	notP, err := f.manager.MkNot(f.app["P"])
	ruleMust(t, err)
	implication, err := f.manager.MkImplies(notP, f.app["Q"])
	ruleMust(t, err)
	formulaRef, err := f.manager.NewRef(implication)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&formulaRef)) }()
	rule, err := f.rules.NewRuleFromFormula(implication, "negative")
	ruleMust(t, err)
	if negation, err := rule.HasNegation(); err != nil || !negation {
		t.Fatal("negative predicate not recognized", negation, err)
	}
	ruleMust(t, rule.Close())

	disjunction, err := f.manager.MkOr(f.app["P"], f.app["Q"])
	ruleMust(t, err)
	nonHorn, err := f.manager.MkImplies(disjunction, f.app["P"])
	ruleMust(t, err)
	nonHornRef, err := f.manager.NewRef(nonHorn)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&nonHornRef)) }()
	if _, err = f.rules.NewRuleFromFormula(nonHorn, "non-horn"); err == nil || !strings.Contains(err.Error(), "normal-form") {
		t.Fatal("non-Horn formula accepted", err)
	}
}
