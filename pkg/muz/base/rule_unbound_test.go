package base_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestRemoveDuplicateRuleTailsUsesSourceSwapWithLastOrder(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	defer func() { ruleMust(t, context.Close()) }()
	manager, err := context.Manager()
	ruleMust(t, err)
	boolean, err := manager.MkBoolSort()
	ruleMust(t, err)
	a, err := manager.MkVar(0, boolean)
	ruleMust(t, err)
	b, err := manager.MkVar(1, boolean)
	ruleMust(t, err)
	c, err := manager.MkVar(2, boolean)
	ruleMust(t, err)
	tails := []ast.ASTHandle{a, b, a, c, b, a}
	negative := []bool{false, false, false, false, true, true}
	got, signs, err := base.RemoveDuplicateRuleTails(tails, negative)
	ruleMust(t, err)
	want := []ast.ASTHandle{a, b, a, c, b}
	wantSigns := []bool{false, false, true, false, true}
	if len(got) != len(want) {
		t.Fatalf("tail count %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] || signs[index] != wantSigns[index] {
			t.Fatalf("tail %d = (%v,%v), want (%v,%v)", index, got[index], signs[index], want[index], wantSigns[index])
		}
	}
	if _, _, err = base.RemoveDuplicateRuleTails(got, signs[:len(signs)-1]); err == nil {
		t.Fatal("mismatched parallel tail vectors accepted")
	}
}

type unboundFixture struct {
	manager *ast.ASTManager
	rules   *base.RuleManager
	lite    *qe.Lite
}

func newUnboundFixture(t *testing.T) *unboundFixture {
	t.Helper()
	context, err := cmd_context.NewStandaloneContext()
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, context.Close()) })
	manager, err := context.Manager()
	ruleMust(t, err)
	globals, err := context.GlobalParameters()
	ruleMust(t, err)
	rules, err := base.NewRuleManager(manager)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, rules.Close()) })
	cooperation, err := cooperate.NewCooperation(manager.Budget())
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, cooperation.Close()) })
	lite, err := qe.NewLite(manager, nil, false, qe.Config{Context: globals.Context, Globals: globals, Cooperation: cooperation})
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, lite.Close()) })
	return &unboundFixture{manager: manager, rules: rules, lite: lite}
}

func (f *unboundFixture) predicate(t *testing.T, name string, domain []ast.ASTHandle, arguments []ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	symbol, err := f.manager.Symbols.Intern(name)
	ruleMust(t, err)
	declaration, err := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, f.rules.RegisterPredicate(declaration))
	application, err := f.manager.MkApp(declaration, arguments)
	ruleMust(t, err)
	return application
}

func TestRuleManagerReducesUnboundInterpretedEquality(t *testing.T) {
	f := newUnboundFixture(t)
	arithmetic, err := f.manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := f.manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	x, err := f.manager.MkVar(0, integer)
	ruleMust(t, err)
	y, err := f.manager.MkVar(1, integer)
	ruleMust(t, err)
	head := f.predicate(t, "reduce-head", []ast.ASTHandle{integer}, []ast.ASTHandle{x})
	bound := f.predicate(t, "reduce-bound", []ast.ASTHandle{integer}, []ast.ASTHandle{x})
	three, err := f.manager.ArithmeticIntegerNumeralText("3")
	ruleMust(t, err)
	definition, err := f.manager.MkEq(y, three)
	ruleMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{bound, definition}, nil, "reduce-unbound")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	reduced, changed, err := f.rules.ReduceUnboundVariables(rule, f.lite)
	ruleMust(t, err)
	defer func() { ruleMust(t, reduced.Close()) }()
	if !changed {
		t.Fatal("unbound definition was not reduced")
	}
	if size, err := reduced.TailSize(); err != nil || size != 1 {
		t.Fatal("reduced tail size", size, err)
	}
	if name, err := reduced.Name(); err != nil || name != "reduce-unbound" {
		t.Fatal("reduced rule name", name, err)
	}
}

func TestRuleManagerFixesResidualUnboundPredicateExistentially(t *testing.T) {
	f := newUnboundFixture(t)
	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	y, err := f.manager.MkVar(1, boolean)
	ruleMust(t, err)
	head := f.predicate(t, "fix-head", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	symbol, err := f.manager.Symbols.Intern("fix-opaque")
	ruleMust(t, err)
	opaqueDecl, err := f.manager.MkFuncDecl(symbol, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	opaque, err := f.manager.MkApp(opaqueDecl, []ast.ASTHandle{y})
	ruleMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{opaque}, nil, "fix-unbound")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	fixed, changed, err := f.rules.FixUnboundVariables(rule, f.lite, true)
	ruleMust(t, err)
	defer func() { ruleMust(t, fixed.Close()) }()
	if !changed {
		t.Fatal("residual unbound predicate was not fixed")
	}
	if size, err := fixed.TailSize(); err != nil || size != 1 {
		t.Fatal("fixed tail size", size, err)
	}
	tail, negative, err := fixed.Tail(0)
	ruleMust(t, err)
	if negative {
		t.Fatal("fixed interpreted tail became negative")
	}
	if operation, err := f.manager.GetDeclKind(tail); err != nil || operation != ast.ASTOpIff {
		t.Fatal("quantified Boolean tail wrapper", operation, err)
	}
	left, err := f.manager.GetArg(tail, 0)
	ruleMust(t, err)
	if kind, err := f.manager.GetKind(left); err != nil || kind != ast.ASTQuantifier {
		t.Fatal("fixed tail did not contain existential", kind, err)
	}
	if forall, err := f.manager.IsForall(left); err != nil || forall {
		t.Fatal("fixed quantifier is not existential", forall, err)
	}
}

func TestRuleManagerAddFormulaAppliesUnboundVariableReductionAndFix(t *testing.T) {
	f := newUnboundFixture(t)
	arithmetic, err := f.manager.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := f.manager.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	x, err := f.manager.MkVar(1, integer)
	ruleMust(t, err)
	y, err := f.manager.MkVar(0, integer)
	ruleMust(t, err)
	head := f.predicate(t, "ingest-head", []ast.ASTHandle{integer}, []ast.ASTHandle{x})
	bound := f.predicate(t, "ingest-bound", []ast.ASTHandle{integer}, []ast.ASTHandle{x})
	three, err := f.manager.ArithmeticIntegerNumeralText("3")
	ruleMust(t, err)
	equality, err := f.manager.MkEq(y, three)
	ruleMust(t, err)
	body, err := f.manager.MkAnd(bound, equality)
	ruleMust(t, err)
	implication, err := f.manager.MkImplies(body, head)
	ruleMust(t, err)
	xName, err := f.manager.Symbols.Intern("x")
	ruleMust(t, err)
	yName, err := f.manager.Symbols.Intern("y")
	ruleMust(t, err)
	formula, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer, integer}, Names: []usymbol.Symbol{xName, yName}, Body: implication})
	ruleMust(t, err)
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(formula)
	ruleMust(t, err)
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofDisabled))
	if err = f.rules.AddFormulaWithProofAndUnboundFix(set, formula, proof, "reduce-ingest", f.lite, false); err != nil {
		t.Fatal(err)
	}
	rule, err := set.Rule(0)
	ruleMust(t, err)
	if size, e := rule.TailSize(); e != nil || size != 1 {
		t.Fatal("QE-lite ingestion did not remove the unbound numeral equality", size, e)
	}
	translated, found, err := rule.Proof()
	ruleMust(t, err)
	if !found {
		t.Fatal("QE-lite reduction lost the rule proof")
	}
	checker, err := proofs.NewChecker(f.manager)
	ruleMust(t, err)
	valid, side, err := checker.Check(translated)
	ruleMust(t, err)
	if !valid {
		t.Fatal("reduced rule proof failed proof checking")
	}
	ruleMust(t, checker.ReleaseSideConditions(side))
	ruleMust(t, rule.Close())

	boolean, err := f.manager.MkBoolSort()
	ruleMust(t, err)
	bx, err := f.manager.MkVar(1, boolean)
	ruleMust(t, err)
	by, err := f.manager.MkVar(0, boolean)
	ruleMust(t, err)
	boolHead := f.predicate(t, "existential-ingest-head", []ast.ASTHandle{boolean}, []ast.ASTHandle{bx})
	opaqueName, err := f.manager.Symbols.Intern("existential-ingest-opaque")
	ruleMust(t, err)
	opaqueDecl, err := f.manager.MkFuncDecl(opaqueName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	opaque, err := f.manager.MkApp(opaqueDecl, []ast.ASTHandle{by})
	ruleMust(t, err)
	boolImplication, err := f.manager.MkImplies(opaque, boolHead)
	ruleMust(t, err)
	boolXName, err := f.manager.Symbols.Intern("bx")
	ruleMust(t, err)
	boolYName, err := f.manager.Symbols.Intern("by")
	ruleMust(t, err)
	boolFormula, err := f.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{boolean, boolean}, Names: []usymbol.Symbol{boolXName, boolYName}, Body: boolImplication})
	ruleMust(t, err)
	if err = f.rules.AddFormulaWithUnboundFix(set, boolFormula, "fix-ingest", f.lite, true); err != nil {
		t.Fatal(err)
	}
	fixed, err := set.Rule(1)
	ruleMust(t, err)
	defer func() { ruleMust(t, fixed.Close()) }()
	tail, negative, err := fixed.Tail(0)
	ruleMust(t, err)
	if negative {
		t.Fatal("fixed ingestion tail became negative")
	}
	operator, err := f.manager.GetDeclKind(tail)
	if err != nil || operator != ast.ASTOpIff {
		t.Fatal("expected equivalence-wrapped existential tail", operator, err)
	}
	quantified, err := f.manager.GetArg(tail, 0)
	ruleMust(t, err)
	if kind, e := f.manager.GetKind(quantified); e != nil || kind != ast.ASTQuantifier {
		t.Fatal("residual unbound variable was not quantified", kind, e)
	}
	if forall, e := f.manager.IsForall(quantified); e != nil || forall {
		t.Fatal("residual tail quantifier is not existential", forall, e)
	}
}
