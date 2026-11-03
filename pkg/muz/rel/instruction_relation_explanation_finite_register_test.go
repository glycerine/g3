package rel

// Source: dl_instruction.cpp relation-level programs keep proof-free magic
// predicates in the ordinary relation register file.

import "testing"

func TestRelationExplanationRegisterInstructionMovesProofFreeFiniteAuxiliary(t *testing.T) {
	context := NewExecutionContext(nil)
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	if err := context.SetFiniteRegister(3, instructionTable(t, 4, 5)); err != nil {
		t.Fatal(err)
	}

	instruction, err := relationExplanationInstructionFor(NewMoveInstruction(3, 7))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := instruction.Close(); err != nil {
			t.Fatal(err)
		}
	})
	complete, err := instruction.Perform(context)
	if err != nil || !complete {
		t.Fatal("move proof-free auxiliary", complete, err)
	}
	if source, err := context.FiniteRegister(3); err != nil || source != nil {
		t.Fatal("move source", source, err)
	}
	target, err := context.FiniteRegister(7)
	if err != nil || target == nil {
		t.Fatal("move target", target, err)
	}
	if count, err := target.Len(); err != nil || count != 2 {
		t.Fatal("move target rows", count, err)
	}
}
