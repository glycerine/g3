package rel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// muz_rel_03-F2: instr_filter_interpreted::perform ends with
// if (r.fast_empty()) ctx.make_empty(m_reg); sparse tables are not exempt, so
// the following union sees a null source and neither counts nor allocates.
func TestFilterInterpretedEmptiesSparseRegister(t *testing.T) {
	f := newTableRelationFixture(t)
	signature, err := NewSignature([]uint64{2})
	if err != nil {
		t.Fatal(err)
	}
	table, err := NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = table.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	context, err := NewExecutionContextWithRelations(f.relations, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = context.Close() })
	if err = context.SetFiniteRegister(0, table); err != nil {
		t.Fatal(err)
	}
	filter, err := NewFilterInterpretedInstruction(f.ast, f.relations, tableRelationTheoryRewriter(t, f.ast), 0, []ast.ASTHandle{f.falsehood}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	block := NewInstructionBlock()
	t.Cleanup(func() { _ = block.Close() })
	if err = block.Append(filter); err != nil {
		t.Fatal(err)
	}
	if err = block.Append(NewUnionInstruction(0, 1, VoidRegister)); err != nil {
		t.Fatal(err)
	}
	complete, err := block.Perform(context)
	if err != nil || !complete {
		t.Fatal("filter and union", complete, err)
	}
	filtered, err := context.FiniteRegister(0)
	if err != nil || filtered != nil {
		t.Fatalf("empty sparse filter result kept in register: %T %v", filtered, err)
	}
	stats, err := context.Statistics()
	if err != nil || stats.Filter != 1 || stats.Union != 0 {
		t.Fatal("filter/union counters", stats, err)
	}
}
