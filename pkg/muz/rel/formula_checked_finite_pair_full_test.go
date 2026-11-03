package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFormulaCheckedFinitePairFullUnaryLifecycleAndMismatch(t *testing.T) {
	f := newTableRelationFixture(t)
	source := newFormulaCheckedFinitePairFixture(t, f)
	full, err := source.FullLike()
	astTestMust(t, err)
	if rows, e := source.Facts(); e != nil || len(rows) != 0 {
		t.Fatal("nested full mutated source", rows, e)
	}
	astTestMust(t, source.Close())
	for _, fact := range []Fact{{0}, {1}} {
		if found, e := full.Contains(fact); e != nil || !found {
			t.Fatal("nested unary full missing row", fact, found, e)
		}
	}
	if rows, e := full.Facts(); e != nil || len(rows) != 2 {
		t.Fatal("nested unary full rows", rows, e)
	}
	astTestMust(t, full.Close())

	corrupt := newFormulaCheckedFinitePairFixture(t, f, Fact{0})
	if inserted, e := corrupt.pair.primary.Add(Fact{1}); e != nil || !inserted {
		t.Fatal("nested full corruption setup", inserted, e)
	}
	if result, e := corrupt.FullLike(); result != nil || !errors.Is(e, ErrCheckTable) {
		t.Fatal("nested full accepted mismatched checked table", result, e)
	}
	if removed, e := corrupt.pair.primary.(*SparseTable).Remove(Fact{1}); e != nil || !removed {
		t.Fatal("nested full corruption repair", removed, e)
	}
	astTestMust(t, corrupt.Close())
}

func TestFormulaCheckedFinitePairFullKeepsWiderComplementExplicit(t *testing.T) {
	f := newTableRelationFixture(t)
	signature, err := NewSignature([]uint64{2, 2})
	astTestMust(t, err)
	factory, err := NewFiniteCheckedFactory("sparse", "sparse")
	astTestMust(t, err)
	pair, err := factory.New(signature)
	astTestMust(t, err)
	nested, err := NewFormulaCheckedFinitePair(f.relations, []ast.ASTHandle{f.boolean, f.boolean}, pair, formulaCheckerConfig(t, f.ast))
	astTestMust(t, err)
	if full, e := nested.FullLike(); full != nil || !errors.Is(e, ErrCheckTable) {
		t.Fatal("nested wider full did not remain explicit", full, e)
	}
	astTestMust(t, nested.Close())
}
