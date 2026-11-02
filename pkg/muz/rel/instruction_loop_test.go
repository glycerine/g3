package rel

// Source: src/muz/rel/dl_instruction.cpp instr_while_loop.

import (
	"bytes"
	"testing"
)

func TestWhileInstructionExecutesUntilControlsEmpty(t *testing.T) {
	context := NewExecutionContext(nil)
	t.Cleanup(func() { _ = context.Close() })
	if err := context.SetRegister(0, instructionRows(t, []uint64{2}, Fact{1})); err != nil {
		t.Fatal(err)
	}
	body := NewInstructionBlock()
	if err := body.Append(NewDeallocateInstruction(0)); err != nil {
		t.Fatal(err)
	}
	loop := NewWhileLoopInstruction([]uint32{0}, body)
	outer := NewInstructionBlock()
	t.Cleanup(func() { _ = outer.Close() })
	if err := outer.Append(loop); err != nil {
		t.Fatal(err)
	}
	complete, err := outer.Perform(context)
	if err != nil || !complete {
		t.Fatal("while instruction", complete, err)
	}
	value, err := context.Register(0)
	if err != nil || value != nil {
		t.Fatal("control register was not emptied", value, err)
	}
	if err = outer.MakeAnnotations(context); err != nil {
		t.Fatal(err)
	}
	annotation, found, err := context.RegisterAnnotation(0)
	if err != nil || !found || annotation != "alloc" {
		t.Fatal("nested annotation", annotation, found, err)
	}
	var display bytes.Buffer
	if err = loop.DisplayHead(&display); err != nil || display.String() != "while[0]" {
		t.Fatal("while display", display.String(), err)
	}
}

func TestWhileInstructionAcceptsSparseFixpointControl(t *testing.T) {
	signature, err := NewSignature([]uint64{2})
	if err != nil {
		t.Fatal(err)
	}
	control, err := NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = control.Add(Fact{1}); err != nil {
		t.Fatal(err)
	}
	context := NewExecutionContext(nil)
	t.Cleanup(func() { _ = context.Close() })
	if err = context.SetFiniteRegister(0, control); err != nil {
		t.Fatal(err)
	}
	body := NewInstructionBlock()
	if err = body.Append(NewDeallocateInstruction(0)); err != nil {
		t.Fatal(err)
	}
	loop := NewWhileLoopInstruction([]uint32{0}, body)
	t.Cleanup(func() { _ = loop.Close() })
	complete, err := loop.Perform(context)
	if err != nil || !complete {
		t.Fatal("sparse fixpoint control", complete, err)
	}
	if table, err := context.FiniteRegister(0); err != nil || table != nil {
		t.Fatal("sparse fixpoint control remained nonempty", table, err)
	}
}

func TestWhileInstructionPropagatesBodyTerminationAndOwnsBody(t *testing.T) {
	context := NewExecutionContext(func() bool { return true })
	t.Cleanup(func() { _ = context.Close() })
	if err := context.SetRegister(0, instructionRows(t, []uint64{2}, Fact{1})); err != nil {
		t.Fatal(err)
	}
	body := NewInstructionBlock()
	if err := body.Append(NewDeallocateInstruction(0)); err != nil {
		t.Fatal(err)
	}
	loop := NewWhileLoopInstruction([]uint32{0}, body)
	complete, err := loop.Perform(context)
	if err != nil || complete {
		t.Fatal("terminated loop", complete, err)
	}
	value, err := context.Register(0)
	if err != nil || value == nil {
		t.Fatal("terminated body changed control", value, err)
	}
	if err = loop.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = body.Len(); err == nil {
		t.Fatal("owned body remained open")
	}
}
