package arith

// Source: src/tactic/arith/fm_tactic.cpp:67-77,158-194.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"testing"
)

func TestFMModelConverterConditionalBooleanClause(t *testing.T) {
	for _, truth := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "satisfied"}[truth], func(t *testing.T) {
			c, m := fmModelFixture(t)
			x := fixDLConst(t, m, false, "fm-cond-x")
			boolean, e := m.MkBoolSort()
			equationMust(t, e)
			name, e := m.Symbols.Intern("fm-cond-p")
			equationMust(t, e)
			p, e := m.MkConst(name, boolean)
			equationMust(t, e)
			three := fixDLNumber(t, m, false, "3")
			bound, e := m.MkArithmeticLE(x, three)
			equationMust(t, e)
			clause, e := m.MkOr(p, bound)
			equationMust(t, e)
			xd, e := m.GetDecl(x)
			equationMust(t, e)
			equationMust(t, c.Insert(xd, []ast.ASTHandle{clause}))
			md, e := model.NewModel(m)
			equationMust(t, e)
			if truth {
				pd, e := m.GetDecl(p)
				equationMust(t, e)
				tv, e := m.MkTrue()
				equationMust(t, e)
				equationMust(t, md.CoreField.RegisterConstant(pd, tv))
			}
			got := fmApply(t, c, md)
			want := "2"
			if truth {
				want = "0"
			}
			if value := fmValue(t, m, got, x); value != want {
				t.Fatal("conditional witness", value, want)
			}
		})
	}
}
