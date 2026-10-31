package rel

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalBoundProductFiltersAndTransforms(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	real, err := m.MkFamilySort(family, 0, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, m, h); return h }
	x, y := own(m.MkVar(0, real)), own(m.MkVar(1, real))
	p := NewIntervalBoundProduct(2, false)
	less := own(m.MkArithmeticLT(x, y))
	astTestMust(t, p.FilterInterpreted(m, less))
	astTestMust(t, p.FilterEqual(0, big.NewRat(1, 1)))
	astTestMust(t, p.FilterEqual(1, big.NewRat(2, 1)))
	if p.Empty() || !p.ContainsFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1)}) {
		t.Fatal("component filters")
	}
	formula, err := p.ToFormula(m, []ast.ASTHandle{real, real})
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&formula)) }()
	root, err := m.RefGet(formula)
	astTestMust(t, err)
	contains, err := ast.NewContainsApp(m, less)
	astTestMust(t, err)
	defer func() { astTestMust(t, contains.Close()) }()
	found, err := contains.Check(root)
	astTestMust(t, err)
	if !found {
		t.Fatal("product formula omitted bound component")
	}
	projected, err := p.Project([]uint32{0})
	astTestMust(t, err)
	if !projected.ContainsFact([]*big.Rat{big.NewRat(2, 1)}) {
		t.Fatal("projected product fact")
	}
	renamed, err := p.Rename([]uint32{0, 1})
	astTestMust(t, err)
	if !renamed.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(1, 1)}) {
		t.Fatal("renamed product fact")
	}
	joined, err := JoinIntervalBoundProducts(p, NewIntervalBoundProduct(1, false), []uint32{1}, []uint32{0})
	astTestMust(t, err)
	if !joined.ContainsFact([]*big.Rat{big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(2, 1)}) {
		t.Fatal("joined product fact")
	}
}

func TestIntervalBoundProductFactsAndEmpty(t *testing.T) {
	p := NewIntervalBoundProduct(1, true)
	if !p.Empty() {
		t.Fatal("new empty product")
	}
	astTestMust(t, p.AddFact([]*big.Rat{big.NewRat(3, 1)}))
	if p.Empty() || !p.ContainsFact([]*big.Rat{big.NewRat(3, 1)}) || p.ContainsFact([]*big.Rat{big.NewRat(4, 1)}) {
		t.Fatal("product fact")
	}
	if err := p.AddFact([]*big.Rat{nil}); err == nil {
		t.Fatal("nil rational fact accepted")
	}
}
