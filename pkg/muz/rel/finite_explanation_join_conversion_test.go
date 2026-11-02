package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteExplanationForeignJoinsAgainstTupleOracle(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	explain := func(name string) ast.ASTHandle {
		sym, e := f.ast.Symbols.Intern(name)
		astTestMust(t, e)
		h, e := f.ast.MkConst(sym, ruleSort)
		astTestMust(t, e)
		astTestPin(t, f.ast, h)
		return h
	}
	a, b, c := explain("cross-true"), explain("cross-false"), explain("cross-inner")
	p, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.truth}, a))
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.falsehood}, b))
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()
	for _, value := range []ast.ASTHandle{f.falsehood, f.truth} {
		_, err = table.AddFact([]ast.ASTHandle{value, f.falsehood})
		astTestMust(t, err)
	}
	for _, reverse := range []bool{false, true} {
		var joined *FiniteExplanationProduct
		if reverse {
			joined, err = p.JoinFromTableRelation(table, []int{0}, []int{0})
		} else {
			joined, err = p.JoinTableRelation(table, []int{0}, []int{0})
		}
		astTestMust(t, err)
		for _, value := range []ast.ASTHandle{f.falsehood, f.truth} {
			for _, t0 := range []ast.ASTHandle{f.falsehood, f.truth} {
				for _, t1 := range []ast.ASTHandle{f.falsehood, f.truth} {
					tuple := []ast.ASTHandle{value, t0, t1}
					if reverse {
						tuple = []ast.ASTHandle{t0, t1, value}
					}
					row, found, e := joined.ExplanationRow(tuple)
					astTestMust(t, e)
					want := value == t0 && t1 == f.falsehood
					if found != want {
						t.Fatalf("table reverse=%t tuple=%v found=%t want=%t", reverse, tuple, found, want)
					}
					expected := b
					if value == f.truth {
						expected = a
					}
					if found && (len(row) != 1 || row[0] != expected) {
						t.Fatal("converted table changed explanation", row, expected)
					}
				}
			}
		}
		astTestMust(t, joined.Close())
	}
	inner, err := NewExplanationRelation(f.ast, []ast.ASTHandle{ruleSort, ruleSort})
	astTestMust(t, err)
	astTestMust(t, inner.AddFact([]ast.ASTHandle{c, {}}))
	right, err := p.JoinExplanationRelation(inner)
	astTestMust(t, err)
	defer func() { astTestMust(t, right.Close()) }()
	left, err := p.JoinFromExplanationRelation(inner)
	astTestMust(t, err)
	defer func() { astTestMust(t, left.Close()) }()
	// Converted temporary relations are already closed; result rows must also
	// survive disposal of both original sources, including undefined slots.
	astTestMust(t, inner.Close())
	astTestMust(t, p.Close())
	for _, value := range []ast.ASTHandle{f.falsehood, f.truth} {
		expected := b
		if value == f.truth {
			expected = a
		}
		row, found, e := right.ExplanationRow([]ast.ASTHandle{value})
		astTestMust(t, e)
		if !found || len(row) != 3 || row[0] != expected || row[1] != c || !row[2].IsNull() {
			t.Fatal("right converted explanation row", row, found)
		}
		row, found, e = left.ExplanationRow([]ast.ASTHandle{value})
		astTestMust(t, e)
		if !found || len(row) != 3 || row[0] != c || !row[1].IsNull() || row[2] != expected {
			t.Fatal("left converted explanation row", row, found)
		}
	}
}

func TestFiniteExplanationForeignJoinEmptyAndColumnGuards(t *testing.T) {
	f := newTableRelationFixture(t)
	p, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, table.Close()) }()
	joined, err := p.JoinTableRelation(table, nil, nil)
	astTestMust(t, err)
	count, err := joined.Len()
	astTestMust(t, err)
	if count != 0 {
		t.Fatal("empty table join", count)
	}
	astTestMust(t, joined.Close())
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	empty, err := NewExplanationRelation(f.ast, []ast.ASTHandle{ruleSort})
	astTestMust(t, err)
	defer func() { astTestMust(t, empty.Close()) }()
	joined, err = p.JoinExplanationRelation(empty)
	astTestMust(t, err)
	count, err = joined.Len()
	astTestMust(t, err)
	if count != 0 {
		t.Fatal("empty inner join", count)
	}
	astTestMust(t, joined.Close())
	if got, e := p.JoinTableRelation(table, []int{1}, []int{0}); e == nil || got != nil {
		t.Fatal("invalid finite join column accepted", got, e)
	}
}
