package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteIntervalProductInnerInterpretedFilter(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
	one, two, three := own(f.ast.ArithmeticIntegerNumeralText("1")), own(f.ast.ArithmeticIntegerNumeralText("2")), own(f.ast.ArithmeticIntegerNumeralText("3"))
	p, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{f.truth, one}, {f.truth, three}, {f.falsehood, three}} {
		astTestMust(t, p.AddFact(fact))
	}
	x := own(f.ast.MkVar(1, integer))
	condition := own(f.ast.MkArithmeticLE(x, two))
	astTestMust(t, p.FilterInterpretedInner(condition))
	for _, tc := range []struct {
		fact []ast.ASTHandle
		want bool
	}{
		{[]ast.ASTHandle{f.truth, two}, true},
		{[]ast.ASTHandle{f.truth, three}, false},
		{[]ast.ASTHandle{f.falsehood, three}, false},
	} {
		found, e := p.ContainsFact(tc.fact)
		if e != nil || found != tc.want {
			t.Fatalf("filter fact %v: %v, %v", tc.fact, found, e)
		}
	}
	finite := own(f.ast.MkVar(0, f.boolean))
	finiteCondition := own(f.ast.MkNot(finite))
	if err = p.FilterInterpretedInner(finiteCondition); err == nil {
		t.Fatal("finite-column condition accepted by inner-only path")
	}
}
