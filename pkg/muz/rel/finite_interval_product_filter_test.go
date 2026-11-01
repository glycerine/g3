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
	if _, err = p.Empty(); err != nil {
		t.Fatal(err)
	}
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

func TestFiniteIntervalProductTableInterpretedFilter(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
	one := own(f.ast.ArithmeticIntegerNumeralText("1"))
	p, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{integer, f.boolean}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	astTestMust(t, p.AddFact([]ast.ASTHandle{one, f.truth}))
	astTestMust(t, p.AddFact([]ast.ASTHandle{one, f.falsehood}))
	variable := own(f.ast.MkVar(1, f.boolean))
	condition := own(f.ast.MkEq(variable, f.truth))
	theory := tableRelationTheoryRewriter(t, f.ast)
	astTestMust(t, p.FilterInterpretedTable(condition, theory))
	if found, err := p.ContainsFact([]ast.ASTHandle{one, f.truth}); err != nil || !found {
		t.Fatal("matching finite row removed", found, err)
	}
	if found, err := p.ContainsFact([]ast.ASTHandle{one, f.falsehood}); err != nil || found {
		t.Fatal("nonmatching finite row kept", found, err)
	}
	innerVariable := own(f.ast.MkVar(0, integer))
	innerCondition := own(f.ast.MkArithmeticLE(innerVariable, one))
	if err = p.FilterInterpretedTable(innerCondition, theory); err == nil {
		t.Fatal("interval condition accepted by table-only path")
	}
	groundFalse := own(f.ast.MkEq(f.truth, f.falsehood))
	astTestMust(t, p.FilterInterpretedTable(groundFalse, theory))
	if empty, err := p.Empty(); err != nil || !empty {
		t.Fatal("ground false condition did not empty product", empty, err)
	}
}

func TestFiniteIntervalProductMixedInterpretedFilter(t *testing.T) {
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
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.truth, one}))
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.falsehood, three}))
	b := own(f.ast.MkVar(0, f.boolean))
	x := own(f.ast.MkVar(1, integer))
	comparison := own(f.ast.MkArithmeticLE(x, two))
	mixed := own(f.ast.MkIte(b, comparison, f.truth))
	astTestMust(t, p.FilterInterpreted(mixed, nil))
	// The interval plugin does not recognize the substituted ITE, and thus
	// conservatively keeps both source rows.
	for _, fact := range [][]ast.ASTHandle{{f.truth, one}, {f.falsehood, three}} {
		found, e := p.ContainsFact(fact)
		if e != nil || !found {
			t.Fatal("mixed filter lost tuple", fact, found, e)
		}
	}
	astTestMust(t, p.FilterInterpreted(comparison, nil))
	if found, err := p.ContainsFact([]ast.ASTHandle{f.falsehood, three}); err != nil || !found {
		t.Fatal("source lookup before empty-row collection", found, err)
	}
	astTestMust(t, p.FilterInterpreted(mixed, nil))
	if count, err := p.table.Len(); err != nil || count != 1 {
		t.Fatal("mixed filter retained empty inner relation", count, err)
	}
	if found, err := p.ContainsFact([]ast.ASTHandle{f.falsehood, three}); err != nil || found {
		t.Fatal("empty row survived mixed filter", found, err)
	}
	theory := tableRelationTheoryRewriter(t, f.ast)
	condition := own(f.ast.MkEq(b, f.truth))
	astTestMust(t, p.FilterInterpreted(condition, theory))
	if found, err := p.ContainsFact([]ast.ASTHandle{f.truth, one}); err != nil || !found {
		t.Fatal("table dispatch lost tuple", found, err)
	}
}

func TestFiniteIntervalProductMixedFilterSubstitutesEachTableRow(t *testing.T) {
	f := newTableRelationFixture(t)
	family, err := f.ast.RegisterArithmeticPlugin()
	astTestMust(t, err)
	integer, err := f.ast.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, e error) ast.ASTHandle { astTestMust(t, e); astTestPin(t, f.ast, h); return h }
	one := own(f.ast.ArithmeticIntegerNumeralText("1"))
	two := own(f.ast.ArithmeticIntegerNumeralText("2"))
	three := own(f.ast.ArithmeticIntegerNumeralText("3"))
	p, err := NewFiniteIntervalProduct(f.relations, []ast.ASTHandle{f.boolean, integer}, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	for _, fact := range [][]ast.ASTHandle{{f.truth, one}, {f.truth, three}, {f.falsehood, three}} {
		astTestMust(t, p.AddFact(fact))
	}
	b := own(f.ast.MkVar(0, f.boolean))
	x := own(f.ast.MkVar(1, integer))
	belowTwo := own(f.ast.MkArithmeticLE(x, two))
	aboveTwo := own(f.ast.MkArithmeticLE(two, x))
	condition := own(f.ast.MkIte(b, belowTwo, aboveTwo))
	astTestMust(t, p.FilterInterpreted(condition, nil))
	for _, tc := range []struct {
		fact []ast.ASTHandle
		want bool
	}{
		{[]ast.ASTHandle{f.truth, one}, true},
		{[]ast.ASTHandle{f.truth, three}, false},
		{[]ast.ASTHandle{f.falsehood, three}, true},
	} {
		found, err := p.ContainsFact(tc.fact)
		if err != nil || found != tc.want {
			t.Fatalf("mixed row substitution for %v: got %v, want %v (%v)", tc.fact, found, tc.want, err)
		}
	}
}
