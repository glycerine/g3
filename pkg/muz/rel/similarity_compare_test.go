package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"testing"
)

func TestSimilarityRuleComparisons(t *testing.T) {
	m := astTestManager(t)
	family, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	integer, e := m.MkFamilySort(family, 1, nil)
	astTestMust(t, e)
	boolean, e := m.MkBoolSort()
	astTestMust(t, e)
	decl := func(name string) ast.ASTHandle {
		h, e := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{integer, integer}, boolean, ast.ASTFuncDeclInfo{})
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	p, q := decl("p"), decl("q")
	rm, e := base.NewRuleManager(m)
	astTestMust(t, e)
	t.Cleanup(func() { astTestMust(t, rm.Close()) })
	astTestMust(t, rm.RegisterPredicate(p))
	astTestMust(t, rm.RegisterPredicate(q))
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, m, h); return h }
	x := own(m.MkVar(0, integer))
	y := own(m.MkVar(1, integer))
	one := own(m.ArithmeticIntegerNumeralText("1"))
	two := own(m.ArithmeticIntegerNumeralText("2"))
	three := own(m.ArithmeticIntegerNumeralText("3"))
	app := func(d, a, b ast.ASTHandle) ast.ASTHandle { return own(m.MkApp(d, []ast.ASTHandle{a, b})) }
	rule := func(h, tail ast.ASTHandle) *base.Rule {
		r, e := rm.NewRule(h, []ast.ASTHandle{tail}, nil, "")
		astTestMust(t, e)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		return r
	}
	a := rule(app(p, x, one), app(q, x, two))
	b := rule(app(p, x, three), app(q, x, two))
	c := rule(app(p, x, one), app(q, x, three))
	d := rule(app(p, y, one), app(q, x, two))
	for _, r := range []*base.Rule{b, c} {
		cmp, e := similarityRoughCompare(a, r)
		astTestMust(t, e)
		if cmp != 0 {
			t.Fatal("constants changed rough class", cmp)
		}
	}
	cmp, e := similarityRoughCompare(a, d)
	astTestMust(t, e)
	if cmp == 0 {
		t.Fatal("different variable index accepted")
	}
	cmp, e = similarityTotalCompare(a, b, -1)
	astTestMust(t, e)
	if cmp == 0 {
		t.Fatal("head constant ignored")
	}
	cmp, e = similarityTotalCompare(a, b, 0)
	astTestMust(t, e)
	if cmp != 0 {
		t.Fatal("head skip", cmp)
	}
	cmp, e = similarityTotalCompare(a, c, 1)
	astTestMust(t, e)
	if cmp != 0 {
		t.Fatal("tail skip", cmp)
	}
	cmp, e = similarityTotalCompare(a, c, 0)
	astTestMust(t, e)
	if cmp == 0 {
		t.Fatal("wrong skip suppressed tail")
	}
	_, e = similarityTotalCompare(a, d, -1)
	if e == nil {
		t.Fatal("different rough classes accepted")
	}

}
