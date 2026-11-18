package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// TestKarrRelationPluginFreesRationals runs the Karr relation plugin's
// functors with numerals too large for the inline rational representation
// and checks that closing the relations (and releasing the functors'
// state) returns every numeral cell they allocated.
func TestKarrRelationPluginFreesRationals(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	astTestPin(t, m, integer)
	own := func(h ast.ASTHandle, err error) ast.ASTHandle { astTestMust(t, err); astTestPin(t, m, h); return h }
	big1 := own(m.ArithmeticIntegerNumeralText("123456789012345678901234567890"))
	big2 := own(m.ArithmeticIntegerNumeralText("-98765432109876543210987654321"))
	three := own(m.ArithmeticIntegerNumeralText("3"))
	x, y := own(m.MkVar(0, integer)), own(m.MkVar(1, integer))
	conditions := []ast.ASTHandle{
		own(m.MkArithmeticLE(x, big1)),
		own(m.MkArithmeticGE(own(m.MkArithmeticMul(big2, y)), x)),
		own(m.MkOrArgs([]ast.ASTHandle{own(m.MkEq(x, big1)), own(m.MkEq(x, big2))})),
	}

	manager, err := NewRelationManager(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, manager.Close()) }()
	p := NewKarrRelationPlugin(manager, nil)
	z := m.NumeralService().Manager.Z
	before := z.StorageStats().LiveCells

	sorts := []ast.ASTHandle{integer, integer}
	mark := manager.fnMark()
	a, err := p.MkFull(ast.ASTHandle{}, sorts)
	astTestMust(t, err)
	for _, condition := range conditions {
		filter, err := p.MkFilterInterpretedFn(a, condition)
		astTestMust(t, err)
		astTestMust(t, filter(a))
	}
	b, err := p.MkEmpty(sorts)
	astTestMust(t, err)
	astTestMust(t, b.(*karrRelationValue).AddFactAST([]ast.ASTHandle{big1, three}))
	equal, err := p.MkFilterEqualFn(b, big2, 1)
	astTestMust(t, err)
	astTestMust(t, equal(b))
	join, err := p.MkJoinFn(a, b, []int{0}, []int{1})
	astTestMust(t, err)
	joined, err := join(a, b)
	astTestMust(t, err)
	rename, err := p.MkRenameFn(joined, []int{0, 2})
	astTestMust(t, err)
	renamed, err := rename(joined)
	astTestMust(t, err)
	clone, err := renamed.CloneRelation()
	astTestMust(t, err)
	formula, err := clone.ToFormula()
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&formula))
	if z.StorageStats().LiveCells == before {
		t.Fatal("the test numerals did not allocate rational cells")
	}
	for _, r := range []RelationBase{clone, renamed, joined, b, a} {
		astTestMust(t, r.Close())
	}
	astTestMust(t, manager.takeFnResources(mark).release())
	if after := z.StorageStats().LiveCells; after != before {
		t.Fatalf("Karr relations leaked %d rational cells", int(after)-int(before))
	}
}
