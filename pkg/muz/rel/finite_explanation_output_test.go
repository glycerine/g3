package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteExplanationProjectionConvertsAndUnionsIntoTables(t *testing.T) {
	f := newTableRelationFixture(t)
	sort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	name, err := f.ast.Symbols.Intern("output-conversion-explanation")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(name, sort)
	astTestMust(t, err)
	astTestPin(t, f.ast, explanation)
	source, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	for _, values := range [][]ast.ASTHandle{{f.falsehood, f.falsehood}, {f.truth, f.falsehood}, {f.truth, f.truth}} {
		astTestMust(t, source.AddFact(values, explanation))
	}
	can, err := source.CanConvertToTableRelation()
	astTestMust(t, err)
	if can {
		t.Fatal("remaining explanation column was convertible")
	}
	if got, e := source.ToTableRelation(); e == nil || got != nil {
		t.Fatal("unprojected explanation converted", got, e)
	}
	projected, err := source.ProjectExplanation([]int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	can, err = projected.CanConvertToTableRelation()
	astTestMust(t, err)
	if !can {
		t.Fatal("fully projected explanations not convertible")
	}
	// An inner filter can leave an empty relation beneath a table key. Source
	// to_table_relation collects that key before removing the functional index.
	key := Fact{1, 1, 0}
	found, err := projected.table.FetchFact(key)
	astTestMust(t, err)
	if !found {
		t.Fatal("missing staged key")
	}
	retired := projected.inner[key[2]]
	astTestMust(t, retired.Reset())
	converted, err := projected.ToTableRelation()
	astTestMust(t, err)
	defer func() { astTestMust(t, converted.Close()) }()
	count, err := projected.table.Len()
	astTestMust(t, err)
	if count != 2 {
		t.Fatal("empty inner key survived conversion", count)
	}
	if !retired.closed {
		t.Fatal("empty inner owner not collected")
	}
	target, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	delta, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean, f.boolean})
	astTestMust(t, err)
	closeTableRelations(t, target, delta)
	for _, row := range []Fact{{0, 0}, {0, 1}} {
		_, err = target.AddTableFact(row)
		astTestMust(t, err)
	}
	_, err = delta.AddTableFact(Fact{1, 1})
	astTestMust(t, err)
	astTestMust(t, projected.UnionIntoTable(target, delta))
	astTestMust(t, projected.UnionIntoTable(target, delta))
	for a := 0; a < 2; a++ {
		for b := 0; b < 2; b++ {
			row := Fact{Element(a), Element(b)}
			got, e := converted.table.Contains(row)
			astTestMust(t, e)
			want := b == 0
			if got != want {
				t.Fatal("converted tuple", row, got, want)
			}
			got, e = target.table.Contains(row)
			astTestMust(t, e)
			want = a == 0 || b == 0
			if got != want {
				t.Fatal("union tuple", row, got, want)
			}
			got, e = delta.table.Contains(row)
			astTestMust(t, e)
			want = a == 1
			if got != want {
				t.Fatal("delta tuple", row, got, want)
			}
		}
	}
	// Conversion and union own their rows independently of the explanation source.
	astTestMust(t, projected.Close())
	astTestMust(t, source.Close())
	astTestMust(t, converted.FilterEqual(f.falsehood, 0))
	requireTableFact(t, target, Fact{1, 0}, true)
	requireTableFact(t, delta, Fact{1, 0}, true)
	requireTableFact(t, converted, Fact{1, 0}, false)
}

func TestFiniteExplanationOutputNullaryAndDomainGuards(t *testing.T) {
	f := newTableRelationFixture(t)
	sort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	name, err := f.ast.Symbols.Intern("nullary-output-fact")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(name, sort)
	astTestMust(t, err)
	astTestPin(t, f.ast, explanation)
	source, err := NewFiniteExplanationProduct(f.relations, nil)
	astTestMust(t, err)
	defer func() { astTestMust(t, source.Close()) }()
	astTestMust(t, source.AddFact(nil, explanation))
	projected, err := source.ProjectExplanation([]int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, projected.Close()) }()
	target, err := NewTableRelation(f.relations, nil)
	astTestMust(t, err)
	delta, err := NewTableRelation(f.relations, nil)
	astTestMust(t, err)
	closeTableRelations(t, target, delta)
	astTestMust(t, projected.UnionIntoTable(target, delta))
	requireTableFact(t, target, Fact{}, true)
	requireTableFact(t, delta, Fact{}, true)
	empty, err := newFiniteExplanationProduct(f.relations, nil, 0)
	astTestMust(t, err)
	defer func() { astTestMust(t, empty.Close()) }()
	converted, err := empty.ToTableRelation()
	astTestMust(t, err)
	closeTableRelations(t, converted)
	if converted.NumRows() != 0 {
		t.Fatal("empty nullary source became true")
	}
	finite, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, finite.Close()) }()
	astTestMust(t, finite.AddFact([]ast.ASTHandle{f.truth}, explanation))
	output, err := finite.ProjectExplanation([]int{0})
	astTestMust(t, err)
	defer func() { astTestMust(t, output.Close()) }()
	targetBool, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	wrong, err := NewTableRelation(f.relations, []ast.ASTHandle{f.bits})
	astTestMust(t, err)
	closeTableRelations(t, targetBool, wrong)
	if err = output.UnionIntoTable(wrong, nil); err == nil {
		t.Fatal("different domain accepted")
	}
	if err = output.UnionIntoTable(targetBool, wrong); err == nil {
		t.Fatal("different delta domain accepted")
	}
	if targetBool.NumRows() != 0 || wrong.NumRows() != 0 {
		t.Fatal("rejected conversion union changed destination")
	}
	if err = source.UnionIntoTable(target, nil); err == nil {
		t.Fatal("union accepted remaining explanation column")
	}
	if target.NumRows() != 1 {
		t.Fatal("rejected source changed existing nullary tuple")
	}
}
