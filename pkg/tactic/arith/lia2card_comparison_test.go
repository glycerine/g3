package arith

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestLIA2CardComparisonMatchesArithmeticTruthTables(t *testing.T) {
	base, m := fixDLFixture(t, false)
	equationMust(t, m.RegisterDeclarationPlugins())
	p := cardinalityBool(t, m, "card-compare-p")
	x := fixDLConst(t, m, true, "card-compare-x")
	y := fixDLConst(t, m, true, "card-compare-y")
	n := func(value string) ast.ASTHandle { return fixDLNumber(t, m, true, value) }
	twox := fixDLApp(t, m, 9, n("2"), x)
	choice := cardinalityIte(t, m, p, n("-1"), n("2"))
	left := fixDLApp(t, m, 6, twox, choice)
	right := fixDLApp(t, m, 6, y, n("1"))
	for _, tc := range []struct{ equality, compile bool }{{false, false}, {true, false}, {true, true}} {
		converted, ok, err := lia2cardComparison(m, left, right, map[ast.ASTHandle]bool{x: true, y: true}, tc.equality, tc.compile)
		equationMust(t, err)
		if !ok {
			t.Fatal("supported comparison rejected", tc)
		}
		term, err := m.RefGet(converted)
		equationMust(t, err)
		for assignment := 0; assignment < 8; assignment++ {
			md, e := model.NewModel(m)
			equationMust(t, e)
			for index, variable := range []ast.ASTHandle{p, x, y} {
				decl, e := m.GetDecl(variable)
				equationMust(t, e)
				var value ast.ASTHandle
				if index == 0 {
					if assignment&1 != 0 {
						value, e = m.MkTrue()
					} else {
						value, e = m.MkFalse()
					}
				} else if assignment&(1<<index) != 0 {
					value = n("1")
				} else {
					value = n("0")
				}
				equationMust(t, e)
				equationMust(t, md.Core().RegisterConstant(decl, value))
			}
			evaluator, e := model.NewModelEvaluator(md.Core(), base.services)
			equationMust(t, e)
			value, e := evaluator.EvaluateValue(term)
			equationMust(t, e)
			actual, e := m.RefGet(value)
			equationMust(t, e)
			lhs := 2*((assignment>>1)&1) + 2
			if assignment&1 != 0 {
				lhs -= 3
			}
			rhs := ((assignment >> 2) & 1) + 1
			want := lhs <= rhs
			if tc.equality {
				want = lhs == rhs
			}
			if (want && !m.IsTrue(actual)) || (!want && !m.IsFalse(actual)) {
				t.Fatalf("comparison %+v assignment %d: got %v want %v", tc, assignment, actual, want)
			}
			equationMust(t, m.DelRef(&value))
			equationMust(t, evaluator.Close())
			equationMust(t, md.Close())
		}
		equationMust(t, m.DelRef(&converted))
	}
}

func TestLIA2CardComparisonTrivialAndUnsupported(t *testing.T) {
	_, m := fixDLFixture(t, false)
	one := fixDLNumber(t, m, true, "1")
	two := fixDLNumber(t, m, true, "2")
	x := fixDLConst(t, m, true, "unbounded-comparison-x")
	for _, tc := range []struct{ equality, compile, want bool }{{false, false, true}, {true, false, false}, {true, true, false}} {
		ref, ok, err := lia2cardComparison(m, one, two, nil, tc.equality, tc.compile)
		equationMust(t, err)
		if !ok {
			t.Fatal("constant comparison rejected")
		}
		value, err := m.RefGet(ref)
		equationMust(t, err)
		if (tc.want && !m.IsTrue(value)) || (!tc.want && !m.IsFalse(value)) {
			t.Fatal("constant comparison", tc, value)
		}
		equationMust(t, m.DelRef(&ref))
	}
	ref, ok, err := lia2cardComparison(m, x, one, nil, false, false)
	equationMust(t, err)
	if ok || ref != (ast.ASTRef{}) {
		t.Fatal("unbounded variable accepted")
	}
}
