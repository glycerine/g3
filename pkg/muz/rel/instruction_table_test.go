package rel

// Source: src/muz/rel/dl_instruction.cpp concrete finite relation operations.

import "testing"

func instructionRows(t *testing.T, domains []uint64, rows ...Fact) *HashTable {
	t.Helper()
	signature, err := NewSignature(domains)
	if err != nil {
		t.Fatal(err)
	}
	table := NewHashTable(signature)
	for _, row := range rows {
		if _, err = table.Add(row); err != nil {
			t.Fatal(err)
		}
	}
	return table
}

func instructionFacts(t *testing.T, context *ExecutionContext, register uint32) []Fact {
	t.Helper()
	table, err := context.Register(register)
	if err != nil || table == nil {
		t.Fatal("missing instruction result", register, err)
	}
	facts, err := table.Facts()
	if err != nil {
		t.Fatal(err)
	}
	return facts
}

func instructionFactEqual(left, right Fact) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func instructionWantFacts(t *testing.T, actual []Fact, expected ...Fact) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatal("fact count", actual, expected)
	}
	for i := range expected {
		if !instructionFactEqual(actual[i], expected[i]) {
			t.Fatal("facts", actual, expected)
		}
	}
}

func TestFiniteTableJoinProjectionRenameInstructions(t *testing.T) {
	context := NewExecutionContext(nil)
	t.Cleanup(func() { _ = context.Close() })
	if err := context.SetRegister(0, instructionRows(t, []uint64{4, 4}, Fact{1, 2}, Fact{2, 3})); err != nil {
		t.Fatal(err)
	}
	if err := context.SetRegister(1, instructionRows(t, []uint64{4, 4}, Fact{2, 0}, Fact{3, 1})); err != nil {
		t.Fatal(err)
	}
	block := NewInstructionBlock()
	t.Cleanup(func() { _ = block.Close() })
	for _, instruction := range []Instruction{
		NewJoinInstruction(0, 1, []int{1}, []int{0}, 2),
		NewJoinProjectInstruction(0, 1, []int{1}, []int{0}, []int{1, 2}, 3),
		NewProjectionInstruction(2, []int{1, 2}, 4),
		NewRenameInstruction(4, []int{0, 1}, 5),
		NewFilterEqualInstruction(5, 0, 0),
	} {
		if err := block.Append(instruction); err != nil {
			t.Fatal(err)
		}
	}
	complete, err := block.Perform(context)
	if err != nil || !complete {
		t.Fatal("table instruction block", complete, err)
	}
	instructionWantFacts(t, instructionFacts(t, context, 2), Fact{1, 2, 2, 0}, Fact{2, 3, 3, 1})
	instructionWantFacts(t, instructionFacts(t, context, 3), Fact{1, 0}, Fact{2, 1})
	instructionWantFacts(t, instructionFacts(t, context, 4), Fact{1, 0}, Fact{2, 1})
	instructionWantFacts(t, instructionFacts(t, context, 5), Fact{0, 1})
	stats, err := context.Statistics()
	if err != nil || stats.Join != 1 || stats.JoinProject != 1 || stats.ProjectRename != 2 || stats.FilterEqual != 1 {
		t.Fatal("table instruction statistics", stats, err)
	}
}

func TestHashTableJoinAllowsDifferentMatchedDomains(t *testing.T) {
	left := instructionRows(t, []uint64{2}, Fact{0}, Fact{1})
	right := instructionRows(t, []uint64{3}, Fact{1}, Fact{2})
	joined, err := left.Join(right, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := joined.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{1, 1})
}

func TestFiniteTableUnionDeltaFiltersAndSelectionInstructions(t *testing.T) {
	context := NewExecutionContext(nil)
	t.Cleanup(func() { _ = context.Close() })
	for register, table := range []*HashTable{
		instructionRows(t, []uint64{4}, Fact{1}),
		instructionRows(t, []uint64{4}, Fact{1}, Fact{2}),
		nil,
		instructionRows(t, []uint64{4, 4}, Fact{1, 1}, Fact{1, 2}, Fact{2, 2}),
		instructionRows(t, []uint64{4}, Fact{2}),
		instructionRows(t, []uint64{4, 4}, Fact{1, 2}, Fact{2, 3}),
		nil,
		instructionRows(t, []uint64{4}, Fact{3}),
		instructionRows(t, []uint64{4}, Fact{2}),
		nil,
	} {
		if err := context.SetRegister(uint32(register), table); err != nil {
			t.Fatal(err)
		}
	}
	block := NewInstructionBlock()
	t.Cleanup(func() { _ = block.Close() })
	for _, instruction := range []Instruction{
		NewUnionInstruction(1, 0, 2),
		NewFilterIdenticalInstruction(3, []int{0, 1}),
		NewFilterEqualInstruction(3, 1, 0),
		NewFilterByNegationInstruction(0, 4, []int{0}, []int{0}),
		NewSelectEqualProjectInstruction(5, 1, 0, 6),
		NewWidenInstruction(7, 8, 9),
	} {
		if err := block.Append(instruction); err != nil {
			t.Fatal(err)
		}
	}
	complete, err := block.Perform(context)
	if err != nil || !complete {
		t.Fatal("table instruction block", complete, err)
	}
	instructionWantFacts(t, instructionFacts(t, context, 0), Fact{1})
	instructionWantFacts(t, instructionFacts(t, context, 2), Fact{2})
	instructionWantFacts(t, instructionFacts(t, context, 3), Fact{1, 1})
	instructionWantFacts(t, instructionFacts(t, context, 6), Fact{2})
	instructionWantFacts(t, instructionFacts(t, context, 8), Fact{2}, Fact{3})
	instructionWantFacts(t, instructionFacts(t, context, 9), Fact{3})
	stats, err := context.Statistics()
	if err != nil || stats.Union != 2 || stats.FilterIdentical != 1 || stats.FilterEqual != 1 || stats.FilterByNegation != 1 || stats.SelectEqualProject != 1 {
		t.Fatal("table instruction statistics", stats, err)
	}
}

func TestHashTableFiniteFiltersValidateAndMutate(t *testing.T) {
	table := instructionRows(t, []uint64{3, 3}, Fact{0, 0}, Fact{0, 1}, Fact{2, 2})
	if err := table.FilterIdentical([]int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if err := table.FilterEqual(2, 0); err != nil {
		t.Fatal(err)
	}
	facts, err := table.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{2, 2})
	if err = table.FilterIdentical([]int{0, 2}); err == nil {
		t.Fatal("out-of-range identical filter accepted")
	}
	if err = table.FilterEqual(3, 0); err != nil {
		t.Fatal("out-of-domain equality filter rejected", err)
	}
	facts, err = table.Facts()
	if err != nil || len(facts) != 0 {
		t.Fatal("out-of-domain equality filter did not remove every row", facts, err)
	}

	unequalDomains := instructionRows(t, []uint64{2, 3}, Fact{0, 0}, Fact{1, 1}, Fact{1, 2})
	if err = unequalDomains.FilterIdentical([]int{0, 1}); err != nil {
		t.Fatal("identical filter rejected columns with different domains", err)
	}
	facts, err = unequalDomains.Facts()
	if err != nil {
		t.Fatal(err)
	}
	instructionWantFacts(t, facts, Fact{0, 0}, Fact{1, 1})
}
