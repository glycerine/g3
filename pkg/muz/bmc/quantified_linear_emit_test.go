package bmc_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
)

// AUDIT.md muz_small_01-F7: qlinear::compile asserts each rule formula and
// each group formula as soon as it is built, and builds the group head
// p#(var) before the group's rules.
func TestQuantifiedLinearCompileEmitsPerRule(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	set, err := base.NewRuleSet(rm)
	must(err)
	defer func() { must(set.Close()) }()
	decl := func(name string) ast.ASTHandle {
		s, e := m.Symbols.Intern(name)
		must(e)
		d, e := m.MkFuncDecl(s, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rm.RegisterPredicate(d))
		return d
	}
	p, q := decl("qemit-p"), decl("qemit-q")
	one, err := m.ArithmeticIntegerNumeralText("1")
	must(err)
	x, err := m.MkVar(0, integer)
	must(err)
	app := func(d, value ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(d, []ast.ASTHandle{value})
		must(e)
		return h
	}
	add := func(head ast.ASTHandle, tails ...ast.ASTHandle) {
		rule, e := rm.NewRule(head, tails, nil, "")
		must(e)
		must(set.Add(rule))
		must(rule.Close())
	}
	add(app(p, one))
	add(app(q, x), app(p, x))
	valid, err := set.Finalize()
	must(err)
	if !valid {
		t.Fatal("rules did not finalize")
	}
	grouped, err := set.GroupedPredicates()
	must(err)
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	var emitted []ast.ASTRef
	defer func() {
		for i := range emitted {
			must(m.DelRef(&emitted[i]))
		}
	}()
	must(compiler.CompileQuantifiedLinearEmit(set, 4, func(formula ast.ASTHandle) error {
		ref, e := m.NewRef(formula)
		if e == nil {
			emitted = append(emitted, ref)
		}
		return e
	}))
	// One rule per predicate: rule formula, then group formula.
	if len(emitted) != 2*len(grouped) {
		t.Fatal("emitted formulas", len(emitted), len(grouped))
	}
	implication := func(ref ast.ASTRef) ast.ASTHandle {
		formula, e := m.RefGet(ref)
		must(e)
		body, e := m.GetQuantifierBody(formula)
		must(e)
		return body
	}
	for i := range grouped {
		ruleFormula, e := m.RefGet(emitted[2*i])
		must(e)
		head, e := m.GetArg(implication(emitted[2*i+1]), 0)
		must(e)
		headID, e := m.GetASTID(head)
		must(e)
		ruleID, e := m.GetASTID(ruleFormula)
		must(e)
		if headID >= ruleID {
			t.Fatalf("group %d head id %d not built before its rule formula id %d", i, headID, ruleID)
		}
	}
}
