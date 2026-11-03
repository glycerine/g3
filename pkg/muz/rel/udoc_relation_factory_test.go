package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestUDocRelationFactoryCapabilitiesCacheAndConstruction(t *testing.T) {
	m := docASTManager(t)
	budget := arena.NewBudget(1 << 20)
	factory, err := NewUDocRelationFactory(m, arena.VectorConfig{PageSize: 8, Descriptors: arena.Config{PageSize: 8, Budget: budget}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(factory.Close)
	bv3, err := m.BitVectorSort(3)
	if err != nil {
		t.Fatal(err)
	}
	finite, err := m.MkDatalogFiniteSort(astTestSymbol(t, m, "udoc-finite"), 5)
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	arithmetic, err := m.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		sorts []ast.ASTHandle
		want  bool
	}{
		{name: "nullary", want: true},
		{name: "bv", sorts: []ast.ASTHandle{bv3}, want: true},
		{name: "finite", sorts: []ast.ASTHandle{finite}, want: true},
		{name: "mixed", sorts: []ast.ASTHandle{bv3, finite}, want: true},
		{name: "bool-source-refusal", sorts: []ast.ASTHandle{boolean}},
		{name: "int-refusal", sorts: []ast.ASTHandle{integer}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, e := factory.CanHandleSignature(tc.sorts)
			if e != nil || got != tc.want {
				t.Fatalf("capability got %v err %v want %v", got, e, tc.want)
			}
		})
	}
	widths, err := factory.SignatureWidths([]ast.ASTHandle{bv3, finite})
	if err != nil || len(widths) != 2 || widths[0] != 3 || widths[1] != 3 {
		t.Fatal("source widths", widths, err)
	}
	empty, err := factory.NewEmpty([]ast.ASTHandle{bv3, finite})
	if err != nil {
		t.Fatal(err)
	}
	defer empty.Close()
	full, err := factory.NewFull([]ast.ASTHandle{bv3, finite})
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	if empty.Core().Manager() != full.Core().Manager() {
		t.Fatal("same total width did not share source manager cache")
	}
	if !empty.Core().FastEmpty() || full.Core().FastEmpty() {
		t.Fatal("empty/full construction", empty.Core().FastEmpty(), full.Core().FastEmpty())
	}
	if got := full.Sorts(); len(got) != 2 || got[0] != bv3 || got[1] != finite {
		t.Fatal("typed signature", got)
	}
}

func TestUDocRelationFactoryCloseInvalidatesBorrowers(t *testing.T) {
	m := docASTManager(t)
	factory, err := NewUDocRelationFactory(m, arena.VectorConfig{})
	if err != nil {
		t.Fatal(err)
	}
	bv, err := m.BitVectorSort(1)
	if err != nil {
		t.Fatal(err)
	}
	relation, err := factory.NewEmpty([]ast.ASTHandle{bv})
	if err != nil {
		t.Fatal(err)
	}
	factory.Close()
	if relation.Core() != nil || relation.Sorts() != nil {
		t.Fatal("factory lifetime did not invalidate borrowed relation")
	}
	if err = relation.Close(); err == nil {
		t.Fatal("relation close after owner destruction did not report closed manager")
	}
}
