package arith

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestForwardTrigPurifySharedSafeBasis(t *testing.T) {
	r, m, _ := forwardTrigFixture(t, false)
	theta := fixDLConst(t, m, false, "safe-theta")
	sin := fixDLApp(t, m, forwardTrigSin, theta)
	cos := fixDLApp(t, m, forwardTrigCos, theta)
	g := forwardTrigGoal(t, m, false, false, forwardTrigEq(t, m, sin, "0"), forwardTrigEq(t, m, cos, "1"))
	forwardTrigApply(t, r, g)
	size, e := g.Size()
	equationMust(t, e)
	if size != 4 {
		t.Fatal("source basis constraints", size)
	}
	first, e := g.Form(0)
	equationMust(t, e)
	second, e := g.Form(1)
	equationMust(t, e)
	x, e := m.GetArg(first, 0)
	equationMust(t, e)
	y, e := m.GetArg(second, 0)
	equationMust(t, e)
	if x == y {
		t.Fatal("sin/cos witnesses aliased")
	}
}

// AUDIT.md tactic_arith_03-F3: the extension model converter walks
// m_sin_cos (obj_map<app, pair>) in table order, not angle creation order.
func TestForwardTrigPurifyExtensionObjMapOrder(t *testing.T) {
	r, m, _ := forwardTrigFixture(t, false)
	var forms, thetas []ast.ASTHandle
	names := map[ast.ASTHandle]string{}
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("order-t%d", i)
		theta := fixDLConst(t, m, false, name)
		names[theta] = name
		thetas = append(thetas, theta)
		forms = append(forms, forwardTrigEq(t, m, fixDLApp(t, m, forwardTrigSin, theta), "0"))
	}
	g := fixDLGoal(t, m, false, true, false, forms...)
	out := forwardTrigApply(t, r, g)
	if out.ModelConverter == nil {
		t.Fatal("no model converter")
	}
	var display bytes.Buffer
	equationMust(t, out.ModelConverter.Display(&display))
	text := display.String()
	table := ast.NewObjMap(m)
	for _, theta := range thetas {
		equationMust(t, table.Insert(theta, [2]ast.ASTHandle{}))
	}
	last, same := -1, true
	for k, entry := range table.Entries() {
		at := strings.Index(text, "("+names[entry.Key]+" (ite")
		if at < 0 || at < last {
			t.Fatalf("%s out of obj_map order:\n%s", names[entry.Key], text)
		}
		last = at
		same = same && entry.Key == thetas[k]
	}
	if same {
		t.Fatal("fixture does not distinguish obj_map order from creation order")
	}
}
