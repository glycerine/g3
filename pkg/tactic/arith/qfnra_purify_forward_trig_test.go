package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestGeneralPurifierForwardTrigMixedModelAndProof(t *testing.T) {
	runner, m := qfnraPurifier(t, true, false)
	theta := fixDLConst(t, m, false, "mixed-theta")
	x := fixDLConst(t, m, false, "mixed-x")
	y := fixDLConst(t, m, false, "mixed-y")
	sin := fixDLApp(t, m, forwardTrigSin, theta)
	cos := fixDLApp(t, m, forwardTrigCos, theta)
	division := fixDLApp(t, m, qfnraPurifyDiv, x, y)
	goal, err := tactic.NewGoal(m, true, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	for _, term := range []ast.ASTHandle{sin, cos, division} {
		zero := fixDLNumber(t, m, false, "0")
		formula, e := m.MkEq(term, zero)
		equationMust(t, e)
		fixDLPin(t, m, formula)
		proof, e := m.MkProofAsserted(formula)
		equationMust(t, e)
		equationMust(t, goal.AssertWithProof(formula, proof, ast.ASTDependency{}))
	}
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	size, err := goal.Size()
	equationMust(t, err)
	if size != 6 || result.ModelConverter == nil {
		t.Fatalf("got %d formulas, converter %v", size, result.ModelConverter)
	}
	var decls []ast.ASTHandle
	for j := uint32(0); j < 3; j++ {
		formula, e := goal.Form(j)
		equationMust(t, e)
		fresh, e := m.GetArg(formula, 0)
		equationMust(t, e)
		decl, e := m.GetDecl(fresh)
		equationMust(t, e)
		decls = append(decls, decl)
	}
	if decls[0] == decls[1] || decls[0] == decls[2] {
		t.Fatal("basis or division auxiliaries alias")
	}
	circle, err := goal.Form(3)
	equationMust(t, err)
	for _, row := range []struct {
		sin, cos string
		valid    bool
	}{{"0", "1", true}, {"3/5", "4/5", true}, {"0", "0", false}, {"1", "1", false}} {
		md, e := model.NewModel(m)
		equationMust(t, e)
		equationMust(t, md.CoreField.RegisterConstant(decls[0], fixDLNumber(t, m, false, row.sin)))
		equationMust(t, md.CoreField.RegisterConstant(decls[1], fixDLNumber(t, m, false, row.cos)))
		ev, e := model.NewModelEvaluator(&md.CoreField, runner.services)
		equationMust(t, e)
		out, e := m.NewRef(ast.ASTHandle{})
		equationMust(t, e)
		equationMust(t, ev.Evaluate(circle, out))
		value, e := m.RefGet(out)
		equationMust(t, e)
		valid := m.IsTrue(value)
		if !valid && !m.IsFalse(value) {
			t.Fatal("circle constraint did not evaluate to a Boolean")
		}
		equationMust(t, m.DelRef(&out))
		equationMust(t, ev.Close())
		equationMust(t, md.Close())
		if valid != row.valid {
			t.Fatalf("circle sin=%s cos=%s evaluated %v", row.sin, row.cos, valid)
		}
	}
	for j := uint32(0); j < size; j++ {
		p, e := goal.Proof(j)
		equationMust(t, e)
		fact, e := m.GetProofFact(p)
		equationMust(t, e)
		formula, e := goal.Form(j)
		equationMust(t, e)
		if fact != formula {
			t.Fatalf("proof %d conclusion", j)
		}
	}
	md, err := model.NewModel(m)
	equationMust(t, err)
	zero := fixDLNumber(t, m, false, "0")
	for _, decl := range decls {
		equationMust(t, md.CoreField.RegisterConstant(decl, zero))
	}
	var output tactic.ModelRef
	equationMust(t, output.Set(&md.CoreField))
	t.Cleanup(func() { equationMust(t, output.Close()) })
	equationMust(t, result.ModelConverter.ApplyModel(&output, 0))
	filtered, err := output.Get()
	equationMust(t, err)
	for _, decl := range decls {
		present, e := filtered.HasInterpretation(decl)
		equationMust(t, e)
		if present {
			t.Fatal("arithmetic auxiliary escaped model filtering")
		}
	}
	thetaDecl, err := m.GetDecl(theta)
	equationMust(t, err)
	present, err := filtered.HasInterpretation(thetaDecl)
	equationMust(t, err)
	if !present {
		t.Fatal("angle was not reconstructed")
	}
}

func TestGeneralPurifierForwardTrigRejectsUnsafeAngle(t *testing.T) {
	runner, m := qfnraPurifier(t, false, false)
	theta := fixDLConst(t, m, false, "unsafe-general-theta")
	sin := fixDLApp(t, m, forwardTrigSin, theta)
	zero := fixDLNumber(t, m, false, "0")
	first, err := m.MkEq(sin, zero)
	equationMust(t, err)
	second, err := m.MkEq(theta, zero)
	equationMust(t, err)
	fixDLPin(t, m, first)
	fixDLPin(t, m, second)
	goal, err := tactic.NewGoal(m, false, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.Assert(first))
	equationMust(t, goal.Assert(second))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	size, err := goal.Size()
	equationMust(t, err)
	got, err := goal.Form(0)
	equationMust(t, err)
	if size != 2 || got != first || result.ModelConverter == nil {
		t.Fatal("unsafe angle was rewritten", size)
	}
	assertEmptyPurifyFilter(t, result.ModelConverter)
}

func TestGeneralPurifierForwardTrigSeesEarlierRewrittenAngleUse(t *testing.T) {
	runner, m := qfnraPurifier(t, false, false)
	theta := fixDLConst(t, m, false, "early-division-theta")
	y := fixDLConst(t, m, false, "early-division-y")
	zero := fixDLNumber(t, m, false, "0")
	division := fixDLApp(t, m, qfnraPurifyDiv, theta, y)
	sine := fixDLApp(t, m, forwardTrigSin, theta)
	first, err := m.MkEq(division, zero)
	equationMust(t, err)
	second, err := m.MkEq(sine, zero)
	equationMust(t, err)
	fixDLPin(t, m, first)
	fixDLPin(t, m, second)
	goal, err := tactic.NewGoal(m, false, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	equationMust(t, goal.Assert(first))
	equationMust(t, goal.Assert(second))
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	got, err := goal.Form(1)
	equationMust(t, err)
	if got != second {
		t.Fatal("angle used by an earlier rewritten division was incorrectly purified")
	}
}

func TestGeneralPurifierForwardTrigSharedAcrossQuantifiers(t *testing.T) {
	runner, m := qfnraPurifier(t, true, true)
	theta := fixDLConst(t, m, false, "quantified-theta")
	sort, err := m.GetSort(theta)
	equationMust(t, err)
	zero := fixDLNumber(t, m, false, "0")
	goal, err := tactic.NewGoal(m, true, true, false)
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, goal.Close()) })
	for _, op := range []int32{forwardTrigSin, forwardTrigCos} {
		term := fixDLApp(t, m, op, theta)
		body, e := m.MkEq(term, zero)
		equationMust(t, e)
		fixDLPin(t, m, body)
		q, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: body})
		equationMust(t, e)
		fixDLPin(t, m, q)
		p, e := m.MkProofAsserted(q)
		equationMust(t, e)
		equationMust(t, goal.AssertWithProof(q, p, ast.ASTDependency{}))
	}
	var result tactic.TacticResult
	t.Cleanup(func() { equationMust(t, result.Close()) })
	equationMust(t, runner.Apply(goal, &result))
	size, err := goal.Size()
	equationMust(t, err)
	if size != 2 || result.ModelConverter == nil {
		t.Fatalf("quantified trig result: %d", size)
	}
	var circles []ast.ASTHandle
	for j := uint32(0); j < 2; j++ {
		q, e := goal.Form(j)
		equationMust(t, e)
		body, e := m.GetQuantifierBody(q)
		equationMust(t, e)
		kind, e := m.GetKind(body)
		equationMust(t, e)
		if kind != ast.ASTApp {
			t.Fatal("basis variables should remain goal-wide")
		}
		count, e := m.GetNumArgs(body)
		equationMust(t, e)
		if count != 2 {
			t.Fatal("circle constraint escaped quantifier", count)
		}
		circle, e := m.GetArg(body, 0)
		equationMust(t, e)
		circles = append(circles, circle)
		p, e := goal.Proof(j)
		equationMust(t, e)
		fact, e := m.GetProofFact(p)
		equationMust(t, e)
		if fact != q {
			t.Fatal("quantified trig proof conclusion")
		}
	}
	if circles[0] != circles[1] {
		t.Fatal("sine and cosine did not share the same basis")
	}
}

func TestGeneralPurifierQuantifiedTrigAndDivisionScopes(t *testing.T) {
	runner, m := qfnraPurifier(t, true, false)
	theta := fixDLConst(t, m, false, "quantified-mixed-theta")
	sort, err := m.GetSort(theta)
	equationMust(t, err)
	x, err := m.MkVar(0, sort)
	equationMust(t, err)
	fixDLPin(t, m, x)
	two := fixDLNumber(t, m, false, "2")
	zero := fixDLNumber(t, m, false, "0")
	sin := fixDLApp(t, m, forwardTrigSin, theta)
	division := fixDLApp(t, m, qfnraPurifyDiv, x, two)
	sum, err := m.MkArithmeticAdd(sin, division)
	equationMust(t, err)
	fixDLPin(t, m, sum)
	body, err := m.MkEq(sum, zero)
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
	if size != 1 || result.ModelConverter == nil {
		t.Fatal("quantified mixed result", size)
	}
	outer, err := goal.Form(0)
	equationMust(t, err)
	inner, err := m.GetQuantifierBody(outer)
	equationMust(t, err)
	count, err := m.GetNumDeclarations(inner)
	equationMust(t, err)
	if count != 1 {
		t.Fatalf("division must bind one auxiliary, got %d", count)
	}
	conjunction, err := m.GetQuantifierBody(inner)
	equationMust(t, err)
	count, err = m.GetNumArgs(conjunction)
	equationMust(t, err)
	if count != 3 {
		t.Fatalf("mixed quantified conjuncts = %d", count)
	}
	divisionConstraint, err := m.GetArg(conjunction, 1)
	equationMust(t, err)
	equality, err := m.GetArg(divisionConstraint, 1)
	equationMust(t, err)
	outerVar, err := m.GetArg(equality, 1)
	equationMust(t, err)
	index, err := m.GetVarIndex(outerVar)
	equationMust(t, err)
	if index != 1 {
		t.Fatalf("outer variable index = %d", index)
	}
	proof, err := goal.Proof(0)
	equationMust(t, err)
	fact, err := m.GetProofFact(proof)
	equationMust(t, err)
	if fact != outer {
		t.Fatal("mixed quantified proof conclusion")
	}
}
