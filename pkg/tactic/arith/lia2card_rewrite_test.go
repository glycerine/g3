package arith

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func TestLIA2CardRewriteAllComparisonDirections(t *testing.T) {
	base, m := fixDLFixture(t, false)
	equationMust(t, m.RegisterDeclarationPlugins())
	x := fixDLConst(t, m, true, "card-directions-x")
	y := fixDLConst(t, m, true, "card-directions-y")
	arg := fixDLApp(t, m, 9, fixDLNumber(t, m, true, "2"), x)
	family, err := m.RegisterArithmeticPlugin()
	equationMust(t, err)
	for _, op := range []int32{2, 3, 4, 5} {
		form := fixDLApp(t, m, op, arg, y)
		walk, e := rewriter.NewTraversal(m, &lia2cardRewriteConfig{manager: m, arithmetic: family, zeroOne: map[ast.ASTHandle]bool{x: true, y: true}})
		equationMust(t, e)
		ref, e := m.NewRef(ast.ASTHandle{})
		equationMust(t, e)
		equationMust(t, walk.Rewrite(form, ref))
		converted, e := m.RefGet(ref)
		equationMust(t, e)
		if converted == form {
			t.Fatal("comparison not rewritten", op)
		}
		for a := 0; a < 2; a++ {
			for b := 0; b < 2; b++ {
				md, e := model.NewModel(m)
				equationMust(t, e)
				for variable, value := range map[ast.ASTHandle]int{x: a, y: b} {
					decl, e := m.GetDecl(variable)
					equationMust(t, e)
					n, e := m.ArithmeticIntegerNumeralText(fmt.Sprint(value))
					equationMust(t, e)
					equationMust(t, md.Core().RegisterConstant(decl, n))
				}
				eval, e := model.NewModelEvaluator(md.Core(), base.services)
				equationMust(t, e)
				v, e := eval.EvaluateValue(converted)
				equationMust(t, e)
				result, e := m.RefGet(v)
				equationMust(t, e)
				want := false
				switch op {
				case 2:
					want = 2*a <= b
				case 3:
					want = 2*a >= b
				case 4:
					want = 2*a < b
				case 5:
					want = 2*a > b
				}
				if (want && !m.IsTrue(result)) || (!want && !m.IsFalse(result)) {
					t.Fatal("incorrect comparison", op, a, b, result)
				}
				equationMust(t, m.DelRef(&v))
				equationMust(t, eval.Close())
				equationMust(t, md.Close())
			}
		}
		equationMust(t, m.DelRef(&ref))
		equationMust(t, walk.Close())
	}
}
