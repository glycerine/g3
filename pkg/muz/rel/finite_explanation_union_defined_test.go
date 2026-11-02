package rel

// Source: dl_mk_explanations.cpp explanation_relation_plugin::union_fn's
// no_undefined precondition, reached by finite_product_relation union_fn.

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteExplanationUnionRejectsUndefinedRowsBeforeMutation(t *testing.T) {
	for _, mode := range []string{"source", "target", "delta", "overlap", "empty-source"} {
		t.Run(mode, func(t *testing.T) {
			f := newTableRelationFixture(t)
			name, err := f.ast.Symbols.Intern("defined-union-proof")
			astTestMust(t, err)
			proof, err := f.ast.MkDatalogRule(name, nil)
			astTestMust(t, err)
			astTestPin(t, f.ast, proof)
			makeProduct := func() *FiniteExplanationProduct {
				product, e := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
				astTestMust(t, e)
				t.Cleanup(func() { astTestMust(t, product.Close()) })
				return product
			}
			target, source, delta := makeProduct(), makeProduct(), makeProduct()
			if mode != "empty-source" {
				astTestMust(t, source.AddFact([]ast.ASTHandle{f.falsehood}, proof))
			}
			switch mode {
			case "source":
				// A defined novel row precedes the undefined row. The preflight
				// must reject the whole source before publishing that first row.
				astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth}, ast.ASTHandle{}))
			case "target", "empty-source":
				astTestMust(t, target.AddFact([]ast.ASTHandle{f.truth}, ast.ASTHandle{}))
			case "delta":
				astTestMust(t, delta.AddFact([]ast.ASTHandle{f.truth}, ast.ASTHandle{}))
			case "overlap":
				astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth}, ast.ASTHandle{}))
				astTestMust(t, target.AddFact([]ast.ASTHandle{f.truth}, proof))
			}
			type productState struct {
				rows  []Fact
				inner [][]ast.ASTHandle
			}
			snapshot := func(product *FiniteExplanationProduct) productState {
				rows, e := product.table.Facts()
				astTestMust(t, e)
				state := productState{rows: rows, inner: make([][]ast.ASTHandle, len(product.inner))}
				for index, row := range product.inner {
					state.inner[index], e = row.Data()
					astTestMust(t, e)
				}
				return state
			}
			before := []productState{snapshot(target), snapshot(source), snapshot(delta)}
			if err := target.Union(source, delta); !errors.Is(err, ast.ErrASTDomain) {
				t.Fatal("union accepted an undefined explanation row", mode, err)
			}
			after := []productState{snapshot(target), snapshot(source), snapshot(delta)}
			if !reflect.DeepEqual(before, after) {
				t.Fatal("undefined explanation rejection changed a product", mode, before, after)
			}
		})
	}
}

func TestFiniteExplanationUnionAcceptsAssignedPlaceholder(t *testing.T) {
	f := newTableRelationFixture(t)
	makeProduct := func() *FiniteExplanationProduct {
		product, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, product.Close()) })
		return product
	}
	source, target, delta := makeProduct(), makeProduct(), makeProduct()
	astTestMust(t, source.AddFact([]ast.ASTHandle{f.truth}, ast.ASTHandle{}))
	name, err := f.ast.Symbols.Intern("assigned-union-proof")
	astTestMust(t, err)
	proof, err := f.ast.MkDatalogRule(name, nil)
	astTestMust(t, err)
	astTestPin(t, f.ast, proof)
	sort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	variable, err := f.ast.MkVar(0, sort)
	astTestMust(t, err)
	astTestPin(t, f.ast, variable)
	condition, err := f.ast.MkEq(variable, proof)
	astTestMust(t, err)
	astTestPin(t, f.ast, condition)
	// Undefined head columns remain valid temporary values until the source
	// assignment filter fills them. Only publication through union is guarded.
	astTestMust(t, source.AssignExplanation(condition))
	astTestMust(t, target.Union(source, delta))
	for _, product := range []*FiniteExplanationProduct{target, source, delta} {
		got, found, err := product.Explanation([]ast.ASTHandle{f.truth})
		astTestMust(t, err)
		if !found || got != proof {
			t.Fatal("assigned explanation did not reach the union and delta", got, found, proof)
		}
	}
}

func TestFiniteExplanationUnionAllowsEmptyAndZeroColumnProducts(t *testing.T) {
	f := newTableRelationFixture(t)
	makeProduct := func(columns int) *FiniteExplanationProduct {
		product, err := newFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean}, columns)
		astTestMust(t, err)
		t.Cleanup(func() { astTestMust(t, product.Close()) })
		return product
	}
	emptyTarget, emptySource, emptyDelta := makeProduct(1), makeProduct(1), makeProduct(1)
	astTestMust(t, emptyTarget.Union(emptySource, emptyDelta))
	// Unreachable historical rows do not participate in the source union.
	astTestMust(t, emptySource.AddFact([]ast.ASTHandle{f.truth}, ast.ASTHandle{}))
	rows, err := emptySource.table.Facts()
	astTestMust(t, err)
	_, err = emptySource.table.Remove(rows[0])
	astTestMust(t, err)
	astTestMust(t, emptyTarget.Union(emptySource, emptyDelta))
	target, source, delta := makeProduct(0), makeProduct(0), makeProduct(0)
	astTestMust(t, source.addEncodedRow(Fact{0, 0}, nil))
	astTestMust(t, target.Union(source, delta))
	for _, product := range []*FiniteExplanationProduct{target, source, delta} {
		row, found, err := product.ExplanationRow([]ast.ASTHandle{f.falsehood})
		astTestMust(t, err)
		if !found || len(row) != 0 {
			t.Fatal("zero-column explanation row was rejected", row, found)
		}
	}
}
