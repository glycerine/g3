package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// TestIntervalRelationPluginFreesRationals runs the interval relation
// plugin's functors on numerals too large for the inline rational
// representation and checks that closing the relations (and releasing the
// functors' state) returns every numeral cell they allocated.
func TestIntervalRelationPluginFreesRationals(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	astTestPin(t, m, integer)
	own := func(h ast.ASTHandle, err error) ast.ASTHandle { astTestMust(t, err); astTestPin(t, m, h); return h }
	big1 := own(m.ArithmeticIntegerNumeralText("123456789012345678901234567890"))
	big2 := own(m.ArithmeticIntegerNumeralText("-98765432109876543210987654321"))
	big3 := own(m.ArithmeticIntegerNumeralText("55555555555555555555555555555"))
	x, y := own(m.MkVar(0, integer)), own(m.MkVar(1, integer))
	le := own(m.MkArithmeticLE(x, big1))
	ge := own(m.MkArithmeticGE(y, big2))
	lt := own(m.MkArithmeticLT(y, x))
	eq := own(m.MkEq(own(m.MkArithmeticAdd(x, big3)), y))

	manager, err := NewRelationManager(m)
	astTestMust(t, err)
	defer func() { astTestMust(t, manager.Close()) }()
	p := NewIntervalRelationPlugin(manager)
	z := m.NumeralService().Manager.Z
	before := z.StorageStats().LiveCells

	sorts := []ast.ASTHandle{integer, integer}
	mark := manager.fnMark()
	a, err := p.MkFull(ast.ASTHandle{}, sorts)
	astTestMust(t, err)
	for _, condition := range []ast.ASTHandle{le, ge, lt} {
		filter, err := p.MkFilterInterpretedFn(a, condition)
		astTestMust(t, err)
		astTestMust(t, filter(a))
	}
	b, err := p.MkEmpty(sorts)
	astTestMust(t, err)
	astTestMust(t, b.(*intervalRelationValue).AddFactAST([]ast.ASTHandle{big1, big3}))
	astTestMust(t, b.(*intervalRelationValue).AddFactAST([]ast.ASTHandle{big2, big3}))
	filter, err := p.MkFilterInterpretedFn(b, eq)
	astTestMust(t, err)
	astTestMust(t, filter(b))
	equal, err := p.MkFilterEqualFn(b, big3, 1)
	astTestMust(t, err)
	astTestMust(t, equal(b))
	found, err := b.(*intervalRelationValue).value.ContainsFactExpressions(m, []ast.ASTHandle{big1, big3})
	astTestMust(t, err)
	_ = found
	delta, err := p.MkEmpty(sorts)
	astTestMust(t, err)
	union, err := p.MkUnionFn(a, b, delta)
	astTestMust(t, err)
	astTestMust(t, union(a, b, delta))
	widen, err := p.MkWidenFn(a, b, nil)
	astTestMust(t, err)
	astTestMust(t, widen(a, b, nil))
	join, err := p.MkJoinFn(a, b, []int{0}, []int{1})
	astTestMust(t, err)
	joined, err := join(a, b)
	astTestMust(t, err)
	project, err := p.MkProjectFn(joined, []int{1})
	astTestMust(t, err)
	projected, err := project(joined)
	astTestMust(t, err)
	rename, err := p.MkRenameFn(projected, []int{0, 2})
	astTestMust(t, err)
	renamed, err := rename(projected)
	astTestMust(t, err)
	identical, err := p.MkFilterIdenticalFn(renamed, []int{0, 1})
	astTestMust(t, err)
	astTestMust(t, identical(renamed))
	clone, err := renamed.CloneRelation()
	astTestMust(t, err)
	formula, err := clone.ToFormula()
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&formula))
	if z.StorageStats().LiveCells == before {
		t.Fatal("the test numerals did not allocate rational cells")
	}
	for _, r := range []RelationBase{clone, renamed, projected, joined, delta, b, a} {
		astTestMust(t, r.Close())
	}
	astTestMust(t, manager.takeFnResources(mark).release())
	if after := z.StorageStats().LiveCells; after != before {
		t.Fatalf("interval relations leaked %d rational cells", int(after)-int(before))
	}
}
