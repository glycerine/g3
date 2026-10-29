package arith

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestForwardTrigPurifyRejectsUnsafeAndCompoundAngles(t *testing.T) {
	for _, row := range []struct {
		name              string
		compound, outside bool
	}{{"outside", false, true}, {"compound", true, false}} {
		t.Run(row.name, func(t *testing.T) {
			r, m, _ := forwardTrigFixture(t, false)
			theta := fixDLConst(t, m, false, "unsafe-theta")
			angle := theta
			if row.compound {
				angle = fixDLApp(t, m, 6, theta, fixDLNumber(t, m, false, "1"))
			}
			sin := fixDLApp(t, m, forwardTrigSin, angle)
			formula := forwardTrigEq(t, m, sin, "0")
			forms := []ast.ASTHandle{formula}
			if row.outside {
				forms = append(forms, forwardTrigEq(t, m, theta, "2"))
			}
			g := forwardTrigGoal(t, m, false, false, forms...)
			forwardTrigApply(t, r, g)
			h, e := g.Form(0)
			equationMust(t, e)
			if h != formula {
				t.Fatal("unsafe angle converted")
			}
		})
	}
}
