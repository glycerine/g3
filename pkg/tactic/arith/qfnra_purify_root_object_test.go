package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestGeneralPurifierRootCachePolicyAndModel(t *testing.T) {
	runner, m := qfnraPurifier(t, true, true)
	root := rootObjectNumeral(t, m)
	x := fixDLConst(t, m, false, "general-root-x")
	y := fixDLConst(t, m, false, "general-root-y")
	goal, err := tactic.NewGoal(m, true, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	for _, v := range []ast.ASTHandle{x, y} {
		formula, e := m.MkEq(root, v)
		equationMust(t, e)
		fixDLPin(t, m, formula)
		p, e := m.MkProofAsserted(formula)
		equationMust(t, e)
		equationMust(t, goal.AssertWithProof(formula, p, ast.ASTDependency{}))
	}
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	size, err := goal.Size()
	equationMust(t, err)
	if size != 5 || result.ModelConverter == nil {
		t.Fatalf("got %d formulas and converter %v", size, result.ModelConverter)
	}
	first, err := goal.Form(0)
	equationMust(t, err)
	second, err := goal.Form(1)
	equationMust(t, err)
	fresh, err := m.GetArg(first, 0)
	equationMust(t, err)
	again, err := m.GetArg(second, 0)
	equationMust(t, err)
	if fresh == root || fresh != again {
		t.Fatal("shared root did not reuse one fresh constant")
	}
	polynomialConstraint, err := goal.Form(2)
	equationMust(t, err)
	sum, err := m.GetArg(polynomialConstraint, 0)
	equationMust(t, err)
	constant, err := m.GetArg(sum, 0)
	equationMust(t, err)
	constantText, _, matched, err := m.ArithmeticNumeralText(constant)
	equationMust(t, err)
	if !matched || constantText != "-2" {
		t.Fatalf("root polynomial constant = %q", constantText)
	}
	product, err := m.GetArg(sum, 1)
	equationMust(t, err)
	coefficient, err := m.GetArg(product, 0)
	equationMust(t, err)
	coefficientText, _, matched, err := m.ArithmeticNumeralText(coefficient)
	equationMust(t, err)
	power, err := m.GetArg(product, 1)
	equationMust(t, err)
	powerBase, err := m.GetArg(power, 0)
	equationMust(t, err)
	exponent, err := m.GetArg(power, 1)
	equationMust(t, err)
	exponentText, _, exponentMatched, err := m.ArithmeticNumeralText(exponent)
	equationMust(t, err)
	if !matched || coefficientText != "1" || powerBase != fresh || !exponentMatched || exponentText != "2" {
		t.Fatal("root polynomial is not fresh² - 2")
	}
	var value polynomial.Algebraic
	matched, err = m.ArithmeticAlgebraicNumeralValue(root, &value)
	equationMust(t, err)
	if !matched {
		t.Fatal("root was not algebraic")
	}
	am, err := m.ArithmeticAlgebraicManager()
	equationMust(t, err)
	lower, upper, err := am.IrrationalBoundsText(&value)
	equationMust(t, err)
	equationMust(t, am.Del(&value))
	for j, want := range []string{lower, upper} {
		constraint, e := goal.Form(uint32(j + 3))
		equationMust(t, e)
		arg := uint32(0)
		if j == 1 {
			arg = 1
		}
		bound, e := m.GetArg(constraint, arg)
		equationMust(t, e)
		text, _, ok, e := m.ArithmeticNumeralText(bound)
		equationMust(t, e)
		if !ok || text != want {
			t.Fatalf("root bound %d = %q, want %q", j, text, want)
		}
	}
	for j := uint32(0); j < size; j++ {
		p, e := goal.Proof(j)
		equationMust(t, e)
		fact, e := m.GetProofFact(p)
		equationMust(t, e)
		form, e := goal.Form(j)
		equationMust(t, e)
		if fact != form {
			t.Fatalf("proof %d has wrong conclusion", j)
		}
	}
	decl, err := m.GetDecl(fresh)
	equationMust(t, err)
	md, err := model.NewModel(m)
	equationMust(t, err)
	zero := fixDLNumber(t, m, false, "0")
	equationMust(t, md.CoreField.RegisterConstant(decl, zero))
	var out tactic.ModelRef
	equationMust(t, out.Set(&md.CoreField))
	t.Cleanup(func() { equationMust(t, out.Close()) })
	equationMust(t, result.ModelConverter.ApplyModel(&out, 0))
	filtered, err := out.Get()
	equationMust(t, err)
	present, err := filtered.HasInterpretation(decl)
	equationMust(t, err)
	if present {
		t.Fatal("root auxiliary escaped model filtering")
	}
	params, err := runner.context.NewValue()
	equationMust(t, err)
	key := "elim_root_objects"
	equationMust(t, runner.context.SetBoolName(&params, &key, false))
	equationMust(t, runner.UpdateParams(&params))
	equationMust(t, runner.context.Del(&params))
	disabled, err := tactic.NewGoal(m, true, false, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, disabled.Close()) })
	formula, err := m.MkEq(root, x)
	equationMust(t, err)
	p, err := m.MkProofAsserted(formula)
	equationMust(t, err)
	equationMust(t, disabled.AssertWithProof(formula, p, ast.ASTDependency{}))
	var disabledResult tactic.TacticResult
	t.Cleanup(func() { equationMust(t, disabledResult.Close()) })
	equationMust(t, runner.Apply(disabled, &disabledResult))
	got, err := disabled.Form(0)
	equationMust(t, err)
	if got != formula {
		t.Fatal("disabled root elimination changed formula")
	}
}

func TestGeneralPurifierQuantifiedRootStaysScoped(t *testing.T) {
	runner, m := qfnraPurifier(t, true, true)
	root := rootObjectNumeral(t, m)
	sort, err := m.GetSort(root)
	equationMust(t, err)
	x, err := m.MkVar(0, sort)
	equationMust(t, err)
	fixDLPin(t, m, x)
	body, err := m.MkEq(root, x)
	equationMust(t, err)
	fixDLPin(t, m, body)
	q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
	equationMust(t, err)
	fixDLPin(t, m, q)
	p, err := m.MkProofAsserted(q)
	equationMust(t, err)
	goal, err := tactic.NewGoal(m, true, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.AssertWithProof(q, p, ast.ASTDependency{}))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	size, err := goal.Size()
	equationMust(t, err)
	if size != 1 || result.ModelConverter != nil {
		t.Fatal("quantified root escaped its binder", size)
	}
	outer, err := goal.Form(0)
	equationMust(t, err)
	inner, err := m.GetQuantifierBody(outer)
	equationMust(t, err)
	count, err := m.GetNumDeclarations(inner)
	equationMust(t, err)
	if count != 1 {
		t.Fatal("root binder count", count)
	}
	conj, err := m.GetQuantifierBody(inner)
	equationMust(t, err)
	count, err = m.GetNumArgs(conj)
	equationMust(t, err)
	if count != 4 {
		t.Fatal("root constraints", count)
	}
	rewritten, err := m.GetArg(conj, 3)
	equationMust(t, err)
	boundRoot, err := m.GetArg(rewritten, 0)
	equationMust(t, err)
	outerVar, err := m.GetArg(rewritten, 1)
	equationMust(t, err)
	rootIndex, err := m.GetVarIndex(boundRoot)
	equationMust(t, err)
	outerIndex, err := m.GetVarIndex(outerVar)
	equationMust(t, err)
	if rootIndex != 0 || outerIndex != 1 {
		t.Fatal("root and outer indices", rootIndex, outerIndex)
	}
	proof, err := goal.Proof(0)
	equationMust(t, err)
	fact, err := m.GetProofFact(proof)
	equationMust(t, err)
	if fact != outer {
		t.Fatal("root quantifier proof conclusion")
	}
}
