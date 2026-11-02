package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFormulaCheckedTableFiniteBoundaryChecksEncodedRows(t *testing.T) {
	f := newTableRelationFixture(t)
	table, err := NewTableRelation(f.relations, []ast.ASTHandle{f.boolean})
	if err != nil {
		t.Fatal(err)
	}
	checked, err := NewFormulaCheckedTable(table, formulaCheckerConfig(t, f.ast))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { astTestMust(t, checked.Close()) }()

	var finite FiniteTable = checked
	signature, err := finite.Signature()
	if err != nil || signature.Len() != 1 {
		t.Fatal("signature", signature, err)
	}
	domain, err := signature.Domain(0)
	if err != nil || domain != 2 {
		t.Fatal("signature domain", domain, err)
	}
	if added, err := finite.Add(Fact{1}); err != nil || !added {
		t.Fatal("checked encoded add", added, err)
	}
	if found, err := finite.Contains(Fact{1}); err != nil || !found {
		t.Fatal("checked encoded contains", found, err)
	}
	if found, err := finite.Contains(Fact{0}); err != nil || found {
		t.Fatal("checked encoded absence", found, err)
	}
	if rows, err := finite.Len(); err != nil || rows != 1 {
		t.Fatal("length", rows, err)
	}

	clone, err := finite.CloneFinite()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := clone.(*FormulaCheckedTable); !ok {
		t.Fatalf("clone lost checked representation: %T", clone)
	}
	if found, err := clone.Contains(Fact{1}); err != nil || !found {
		t.Fatal("checked clone membership", found, err)
	}
	if err = clone.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err = finite.Contains(Fact{2}); !errors.Is(err, ErrTable) {
		t.Fatal("out-of-domain encoded row", err)
	}
}

func TestFormulaCheckedTableCloneFiniteNormalizesErrorNil(t *testing.T) {
	var checked *FormulaCheckedTable
	clone, err := checked.CloneFinite()
	if err == nil || clone != nil {
		t.Fatal("closed clone result", clone, err)
	}
}
