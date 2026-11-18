package smtcore

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTBoundedWeightedDisequalitiesModel(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "QF_LIA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	astTestMust(t, s.SetProduceModels(true))
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	sort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	num := func(v int64) ast.ASTHandle {
		h, err := bigTestArithNumeral(m, big.NewRat(v, 1), true)
		astTestMust(t, err)
		return h
	}
	app := func(op int32, args ...ast.ASTHandle) ast.ASTHandle {
		h, err := m.MkFamilyApp(family, op, nil, args)
		astTestMust(t, err)
		return h
	}
	var vars []ast.ASTHandle
	for i := 0; i < 3; i++ {
		x, err := m.MkConst(astTestSymbol(t, m, fmt.Sprintf("weighted-%d", i)), sort)
		astTestMust(t, err)
		vars = append(vars, x)
	}
	sum := app(smtArithOpAdd, vars[0], app(smtArithOpMul, num(2), vars[1]), app(smtArithOpMul, num(4), vars[2]))
	forms := []ast.ASTHandle{app(smtArithOpGE, sum, num(0)), app(smtArithOpLE, sum, num(7))}
	for i := int64(0); i < 6; i++ {
		eq, err := m.MkEq(sum, num(i))
		astTestMust(t, err)
		not, err := m.MkNot(eq)
		astTestMust(t, err)
		forms = append(forms, not)
	}
	for _, v := range vars {
		forms = append(forms, app(smtArithOpLE, num(0), v), app(smtArithOpLE, v, num(1)))
	}
	for _, form := range forms {
		astTestMust(t, s.Assert(form))
	}
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	md, err := s.AcquireModel()
	astTestMust(t, err)
	defer func() { astTestMust(t, md.DecRef()) }()
	var total big.Rat
	for i, v := range vars {
		d, err := m.GetDecl(v)
		astTestMust(t, err)
		n, err := md.ConstantInterpretation(d)
		astTestMust(t, err)
		text, _, found, err := m.ArithmeticNumeralText(n)
		astTestMust(t, err)
		value, ok := new(big.Rat).SetString(text)
		if !found || !ok || !value.IsInt() || value.Sign() < 0 || value.Cmp(big.NewRat(1, 1)) > 0 {
			t.Fatal("nonbinary model value", text)
		}
		total.Add(&total, new(big.Rat).Mul(value, big.NewRat(int64(1<<i), 1)))
	}
	if total.Cmp(big.NewRat(6, 1)) < 0 || total.Cmp(big.NewRat(7, 1)) > 0 {
		t.Fatal("model violates weighted disequalities", total.RatString())
	}
}
