package arith

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestRecover01ModelReconstruction(t *testing.T) {
	base, m := fixDLFixture(t, false)
	sort, err := m.MkBoolSort()
	equationMust(t, err)
	symbol, err := m.Symbols.Intern("model-p")
	equationMust(t, err)
	p, err := m.MkConst(symbol, sort)
	equationMust(t, err)
	fixDLPin(t, m, p)
	x := fixDLConst(t, m, true, "model-x")
	goal := fixDLGoal(t, m, false, true, false)
	sub, err := m.NewExprSubstitution()
	equationMust(t, err)
	defer func() { equationMust(t, m.DelExprSubstitution(&sub)) }()
	d := &recover01Definitions{manager: m, goal: goal, sub: sub}
	defer func() { equationMust(t, d.Close()) }()
	equationMust(t, d.build(x, []ast.ASTHandle{p}, []big.Rat{*big.NewRat(3, 1)}))
	fresh, err := m.RefGet(d.booleans[0].fresh)
	equationMust(t, err)
	freshDecl, err := m.GetDecl(fresh)
	equationMust(t, err)
	pDecl, err := m.GetDecl(p)
	equationMust(t, err)
	xDecl, err := m.GetDecl(x)
	equationMust(t, err)
	var out tactic.TacticResult
	equationMust(t, out.Prepare(m))
	defer func() { equationMust(t, out.Close()) }()
	equationMust(t, d.publishModel(base.services, &out))
	md, err := model.NewModel(m)
	equationMust(t, err)
	one := fixDLNumber(t, m, true, "1")
	equationMust(t, md.CoreField.RegisterConstant(freshDecl, one))
	var ref tactic.ModelRef
	equationMust(t, ref.Set(&md.CoreField))
	defer func() { equationMust(t, ref.Close()) }()
	equationMust(t, out.ModelConverter.ApplyModel(&ref, 0))
	converted, err := ref.Get()
	equationMust(t, err)
	pValue, err := converted.ConstantInterpretation(pDecl)
	equationMust(t, err)
	truth, err := m.MkTrue()
	equationMust(t, err)
	if pValue != truth {
		t.Fatal("Boolean value not reconstructed")
	}
	xValue, err := converted.ConstantInterpretation(xDecl)
	equationMust(t, err)
	text, _, ok, err := m.ArithmeticNumeralText(xValue)
	equationMust(t, err)
	if !ok || text != "3" {
		t.Fatalf("arithmetic value: %s", text)
	}
	present, err := converted.HasInterpretation(freshDecl)
	equationMust(t, err)
	if present {
		t.Fatal("fresh bit was not filtered")
	}
}

func TestRecover01ModelReconstructsRealDefinitionFromNegatedBoolean(t *testing.T) {
	base, m := fixDLFixture(t, false)
	boolSort, err := m.MkBoolSort()
	equationMust(t, err)
	name, err := m.Symbols.Intern("real-model-p")
	equationMust(t, err)
	p, err := m.MkConst(name, boolSort)
	equationMust(t, err)
	fixDLPin(t, m, p)
	notP, err := m.MkNot(p)
	equationMust(t, err)
	fixDLPin(t, m, notP)
	x := fixDLConst(t, m, false, "real-model-x")
	goal := fixDLGoal(t, m, false, true, false)
	sub, err := m.NewExprSubstitution()
	equationMust(t, err)
	defer func() { equationMust(t, m.DelExprSubstitution(&sub)) }()
	defs := &recover01Definitions{manager: m, goal: goal, sub: sub}
	defer func() { equationMust(t, defs.Close()) }()
	equationMust(t, defs.build(x, []ast.ASTHandle{notP}, []big.Rat{*big.NewRat(1, 2)}))
	fresh, err := m.RefGet(defs.booleans[0].fresh)
	equationMust(t, err)
	freshDecl, err := m.GetDecl(fresh)
	equationMust(t, err)
	pDecl, err := m.GetDecl(p)
	equationMust(t, err)
	xDecl, err := m.GetDecl(x)
	equationMust(t, err)
	var out tactic.TacticResult
	equationMust(t, out.Prepare(m))
	defer func() { equationMust(t, out.Close()) }()
	equationMust(t, defs.publishModel(base.services, &out))
	md, err := model.NewModel(m)
	equationMust(t, err)
	zero := fixDLNumber(t, m, true, "0")
	equationMust(t, md.CoreField.RegisterConstant(freshDecl, zero))
	var ref tactic.ModelRef
	equationMust(t, ref.Set(&md.CoreField))
	defer func() { equationMust(t, ref.Close()) }()
	equationMust(t, out.ModelConverter.ApplyModel(&ref, 0))
	converted, err := ref.Get()
	equationMust(t, err)
	pValue, err := converted.ConstantInterpretation(pDecl)
	equationMust(t, err)
	falsehood, err := m.MkFalse()
	equationMust(t, err)
	if pValue != falsehood {
		t.Fatal("negated Boolean definition did not reconstruct false")
	}
	xValue, err := converted.ConstantInterpretation(xDecl)
	equationMust(t, err)
	text, _, ok, err := m.ArithmeticNumeralText(xValue)
	equationMust(t, err)
	if !ok || text != "1/2" {
		t.Fatalf("real arithmetic value: %q", text)
	}
	if has, err := converted.HasInterpretation(freshDecl); err != nil || has {
		t.Fatalf("recovered integer helper should be filtered, has=%v err=%v", has, err)
	}
}
