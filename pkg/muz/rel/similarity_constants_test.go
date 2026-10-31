package rel

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"testing"
)

func TestSimilarityVaryingConstantsAndEqualParents(t *testing.T) {
	m := astTestManager(t)
	family, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	integer, e := m.MkFamilySort(family, 1, nil)
	astTestMust(t, e)
	boolean, e := m.MkBoolSort()
	astTestMust(t, e)
	decl := func(name string) ast.ASTHandle {
		h, e := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{integer, integer, integer}, boolean, ast.ASTFuncDeclInfo{})
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
	one := own(m.ArithmeticIntegerNumeralText("1"))
	two := own(m.ArithmeticIntegerNumeralText("2"))
	three := own(m.ArithmeticIntegerNumeralText("3"))
	stable := own(m.ArithmeticIntegerNumeralText("9"))
	rule := func(first, second, third ast.ASTHandle) *base.Rule {
		head := own(m.MkApp(p, []ast.ASTHandle{x, first, second}))
		tail := own(m.MkApp(q, []ast.ASTHandle{x, third, stable}))
		r, e := rm.NewRule(head, []ast.ASTHandle{tail}, nil, "")
		astTestMust(t, e)
		t.Cleanup(func() { astTestMust(t, r.Close()) })
		return r
	}
	a, b, c := rule(one, one, one), rule(two, two, two), rule(three, one, three)
	positions, e := similarityVaryingConstants([]*base.Rule{a, b})
	astTestMust(t, e)
	if len(positions) != 3 || positions[0] != (similarityConstant{tail: -1, argument: 1, parent: -1}) || positions[1] != (similarityConstant{tail: -1, argument: 2, parent: 0}) || positions[2] != (similarityConstant{tail: 0, argument: 1, parent: 0}) {
		t.Fatalf("positions %+v", positions)
	}
	positions, e = similarityVaryingConstants([]*base.Rule{a, b, c})
	astTestMust(t, e)
	if len(positions) != 3 || positions[1].parent != -1 || positions[2].parent != 0 {
		t.Fatalf("parents after changed row: %+v", positions)
	}
	positions, e = similarityVaryingConstants([]*base.Rule{a, a})
	astTestMust(t, e)
	if len(positions) != 0 {
		t.Fatal("stable values were not removed", positions)
	}
	if _, e = similarityVaryingConstants([]*base.Rule{a}); e == nil {
		t.Fatal("single-rule class accepted")
	}
}
