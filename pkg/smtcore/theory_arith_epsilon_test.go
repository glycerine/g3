package smtcore

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTArithmeticSharedEpsilonPreservesDistinctValues(t *testing.T) {
	m, cfg := smtModelGeneratorFixture(t)
	s, err := NewSMTSolver(m, nil, "QF_UFLRA", SMTSolverConfig{Params: cfg.Context, ModelEvaluator: cfg})
	astTestMust(t, err)
	// Exercise the retained legacy tableau algorithm explicitly.
	smtDisableAutoConfig(t, s)
	t.Cleanup(func() { astTestMust(t, s.Close()) })
	s.kernel.preprocess = false
	astTestMust(t, s.SetProduceModels(true))
	family, err := m.ArithmeticFamilyID()
	astTestMust(t, err)
	sort, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	x, err := m.MkConst(astTestSymbol(t, m, "epsilon-x"), sort)
	astTestMust(t, err)
	y, err := m.MkConst(astTestSymbol(t, m, "epsilon-y"), sort)
	astTestMust(t, err)
	f, err := m.MkFuncDecl(astTestSymbol(t, m, "epsilon-f"), []ast.ASTHandle{sort}, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, err)
	fx, err := m.MkApp(f, []ast.ASTHandle{x})
	astTestMust(t, err)
	fy, err := m.MkApp(f, []ast.ASTHandle{y})
	astTestMust(t, err)
	eq, err := m.MkEq(fx, fy)
	astTestMust(t, err)
	neq, err := m.MkNot(eq)
	astTestMust(t, err)
	astTestMust(t, s.Assert(neq))
	for _, bound := range []struct {
		term  ast.ASTHandle
		op    int32
		value *big.Rat
	}{{x, -1, big.NewRat(19, 2)}, {y, smtArithOpGT, big.NewRat(9, 1)}, {y, smtArithOpLT, big.NewRat(10, 1)}} {
		numeral, err := m.ArithmeticNumeralRat(bound.value, false)
		astTestMust(t, err)
		var formula ast.ASTHandle
		if bound.op == -1 {
			formula, err = m.MkEq(bound.term, numeral)
		} else {
			formula, err = m.MkFamilyApp(family, bound.op, nil, []ast.ASTHandle{bound.term, numeral})
		}
		astTestMust(t, err)
		astTestMust(t, s.Assert(formula))
	}
	status, err := s.CheckSat(nil)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	model, err := s.AcquireModel()
	astTestMust(t, err)
	var modelValues [2]string
	for i, term := range []ast.ASTHandle{x, y} {
		decl, err := m.GetDecl(term)
		astTestMust(t, err)
		value, err := model.ConstantInterpretation(decl)
		astTestMust(t, err)
		text, _, matched, err := m.ArithmeticNumeralText(value)
		astTestMust(t, err)
		if !matched {
			t.Fatal("missing arithmetic model value")
		}
		modelValues[i] = text
	}
	astTestMust(t, model.DecRef())
	if modelValues[0] == modelValues[1] {
		t.Fatal("returned model violates disequality of UF applications", modelValues)
	}
	theory := s.kernel.internalizer.theories[family].(*SMTLinearArithmeticTheory)
	// Reproduce the source bug with distinct symbolic values 19/2 and
	// 10-epsilon that collide at the initial model epsilon of 1/2.
	columns := make([]uint32, 0, 4)
	for _, expr := range []ast.ASTHandle{x, y, fx, fy} {
		node := theory.state.context.enodes.get(expr)
		columns = append(columns, theory.simplexVariables[node.TheoryVar(family)])
	}
	theory.solver = simplex.NewSolver(nil)
	for i, column := range columns {
		value := simplex.RationalInfinitesimal(big.NewRat(19, 2))
		switch i {
		case 1:
			value.Main.SetInt64(10)
			value.Epsilon.SetInt64(-1)
		case 2:
			value.Main.SetInt64(9)
		case 3:
			value.Main.SetInt64(11)
		}
		theory.solver.SetValue(column, value)
	}
	// Include a bound so the initial epsilon is exactly 1/2.
	astTestMust(t, theory.solver.SetLower(columns[1], simplex.RationalInfinitesimal(big.NewRat(19, 2))))
	astTestMust(t, theory.computeModelDelta())
	if theory.delta.Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal("epsilon was not refined after shared-value collision", &theory.delta)
	}
	if theory.arithmeticColumnValue(columns[0]).Cmp(theory.arithmeticColumnValue(columns[1])) == 0 {
		t.Fatal("distinct shared symbolic values collapsed in rational model")
	}
	// Symbolically equal values must remain equal and must not cause an
	// endless refinement loop.
	theory.solver.SetValue(columns[1], theory.solver.Value(columns[0]))
	astTestMust(t, theory.computeModelDelta())
	if theory.delta.Cmp(big.NewRat(1, 1)) != 0 || theory.arithmeticColumnValue(columns[0]).Cmp(theory.arithmeticColumnValue(columns[1])) != 0 {
		t.Fatal("equal symbolic values were separated")
	}
}

func TestSMTArithmeticModelEpsilonUsesNativeBoundQuotient(t *testing.T) {
	for _, test := range []struct {
		name         string
		main, eps    int64
		lower, upper *simplex.Infinitesimal
		want         *big.Rat
	}{
		{"lower", 10, -2, smtEpsilonBound(9, 0), nil, big.NewRat(1, 2)},
		{"upper", 0, 3, nil, smtEpsilonBound(1, 0), big.NewRat(1, 3)},
		{"both", 10, -2, smtEpsilonBound(9, 0), smtEpsilonBound(11, -5), big.NewRat(1, 3)},
		{"cap-at-one", 10, -1, smtEpsilonBound(8, 0), nil, big.NewRat(1, 1)},
		{"equal-main", 10, 2, smtEpsilonBound(10, 1), nil, big.NewRat(1, 1)},
		{"ordered-epsilon", 10, 2, smtEpsilonBound(9, 1), nil, big.NewRat(1, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			// No enodes are shared here; the separate collision regression
			// exercises refine_epsilon after this bound calculation.
			theory := &SMTLinearArithmeticTheory{solver: simplex.NewSolver(nil)}
			theory.solver.SetValue(0, *smtEpsilonBound(test.main, test.eps))
			if test.lower != nil {
				astTestMust(t, theory.solver.SetLower(0, *test.lower))
			}
			if test.upper != nil {
				astTestMust(t, theory.solver.SetUpper(0, *test.upper))
			}
			astTestMust(t, theory.computeModelDelta())
			if theory.delta.Cmp(test.want) != 0 {
				t.Fatal("native model epsilon", &theory.delta, "want", test.want)
			}
		})
	}
}

func smtEpsilonBound(main, epsilon int64) *simplex.Infinitesimal {
	value := simplex.RationalInfinitesimal(big.NewRat(main, 1))
	value.Epsilon.SetInt64(epsilon)
	return &value
}
