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

func TestIntervalBoundProductUnionAndDelta(t *testing.T) {
	fact := func(n int64) *IntervalBoundProduct {
		p := NewIntervalBoundProduct(1, true)
		astTestMust(t, p.AddFact([]*big.Rat{big.NewRat(n, 1)}))
		return p
	}
	target, source, delta := fact(1), fact(3), fact(10)
	changed, err := target.Union(source, delta, false)
	astTestMust(t, err)
	if !changed || !target.ContainsFact([]*big.Rat{big.NewRat(2, 1)}) || !delta.ContainsFact([]*big.Rat{big.NewRat(10, 1)}) || !delta.ContainsFact([]*big.Rat{big.NewRat(1, 1)}) {
		t.Fatal("union or accumulated delta omitted facts")
	}
	changed, err = target.Union(NewIntervalBoundProduct(1, true), delta, false)
	astTestMust(t, err)
	if changed || !delta.ContainsFact([]*big.Rat{big.NewRat(10, 1)}) {
		t.Fatal("empty source changed target or cleared accumulated delta")
	}
	if _, err = target.Union(source, target, false); err == nil {
		t.Fatal("aliased delta accepted")
	}
	if _, err = target.Union(NewIntervalBoundProduct(2, false), nil, false); err == nil {
		t.Fatal("mismatched arity accepted")
	}
}

func TestIntervalBoundProductWiden(t *testing.T) {
	target, source := NewIntervalBoundProduct(1, true), NewIntervalBoundProduct(1, true)
	astTestMust(t, target.AddFact([]*big.Rat{big.NewRat(1, 1)}))
	astTestMust(t, source.AddFact([]*big.Rat{big.NewRat(3, 1)}))
	changed, err := target.Union(source, nil, true)
	astTestMust(t, err)
	if !changed || !target.ContainsFact([]*big.Rat{big.NewRat(100, 1)}) || target.ContainsFact([]*big.Rat{big.NewRat(0, 1)}) {
		t.Fatal("widening did not extend the increasing upper endpoint")
	}
}
