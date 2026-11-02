package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestNewRelationExplanationTotalUndefinedAndZeroProofIdentity(t *testing.T) {
	f := newTableRelationFixture(t)
	for _, width := range []int{0, 2} {
		total, err := NewRelationExplanationTotal(f.relations, width)
		must(t, err)
		row, err := total.explanation.Data()
		must(t, err)
		if len(row) != width {
			t.Fatal("total proof width", width, row)
		}
		for _, value := range row {
			if !value.IsNull() {
				t.Fatal("total proof slot defined", width, row)
			}
		}
		facts, err := total.table.Facts()
		must(t, err)
		empty, err := total.Empty()
		must(t, err)
		if empty || len(facts) != 1 || len(facts[0]) != 0 {
			t.Fatal("nullary total identity", width, empty, facts)
		}
		must(t, total.Close())
	}
	if total, err := NewRelationExplanationTotal(f.relations, -1); err == nil || total != nil {
		t.Fatal("negative proof width accepted", total, err)
	}
}

func TestRelationExplanationFromSparseCopiesRowsAndZeroProofIdentity(t *testing.T) {
	f := newTableRelationFixture(t)
	signature, err := NewSignature([]uint64{2})
	must(t, err)
	source, err := NewSparseTable(signature, 0)
	must(t, err)
	_, err = source.Add(Fact{0})
	must(t, err)
	_, err = source.Add(Fact{1})
	must(t, err)
	product, err := RelationExplanationFromFinite(f.relations, []ast.ASTHandle{f.boolean}, source)
	must(t, err)
	must(t, source.Reset())
	must(t, source.Close())
	defer func() { must(t, product.Close()) }()

	facts, err := product.table.Facts()
	must(t, err)
	proof, err := product.explanation.Data()
	must(t, err)
	if len(facts) != 2 || len(proof) != 0 {
		t.Fatal("finite conversion", facts, proof)
	}
	empty, err := product.Empty()
	must(t, err)
	if empty {
		t.Fatal("zero-proof finite conversion became empty")
	}
}

func TestRelationExplanationRenameFiniteOwnsProofClone(t *testing.T) {
	f := newTableRelationFixture(t)
	proof := relationExplanationProof(t, f.ast, "relation-rename-proof")
	source := relationExplanationProductWithProofRow(t, f, []ast.ASTHandle{f.boolean, f.bits}, []Fact{{1, 2}}, []ast.ASTHandle{proof})
	renamed, err := source.RenameFinite([]int{0, 1})
	must(t, err)
	must(t, source.Close())
	defer func() { must(t, renamed.Close()) }()

	facts, err := renamed.table.Facts()
	must(t, err)
	row, err := renamed.explanation.Data()
	must(t, err)
	if len(facts) != 1 || len(facts[0]) != 2 || facts[0][0] != 2 || facts[0][1] != 1 {
		t.Fatal("finite rename", facts)
	}
	if len(renamed.finiteSorts) != 2 || renamed.finiteSorts[0] != f.bits || renamed.finiteSorts[1] != f.boolean || len(row) != 1 || row[0] != proof {
		t.Fatal("renamed signature/proof", renamed.finiteSorts, row)
	}
}

func TestRelationExplanationEmptyUsesEitherComponent(t *testing.T) {
	f := newTableRelationFixture(t)
	proof := relationExplanationProof(t, f.ast, "relation-empty-proof")
	finiteEmpty := relationExplanationProductWithProofRow(t, f, []ast.ASTHandle{f.boolean}, nil, []ast.ASTHandle{proof})
	defer func() { must(t, finiteEmpty.Close()) }()
	empty, err := finiteEmpty.Empty()
	must(t, err)
	if !empty {
		t.Fatal("empty finite component reported nonempty")
	}

	total, err := NewRelationExplanationTotal(f.relations, 1)
	must(t, err)
	defer func() { must(t, total.Close()) }()
	must(t, total.explanation.Reset())
	empty, err = total.Empty()
	must(t, err)
	if !empty {
		t.Fatal("empty explanation component reported nonempty")
	}
}
