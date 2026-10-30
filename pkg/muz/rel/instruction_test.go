package rel

// Source: src/muz/rel/dl_instruction.cpp execution_context, instruction_block,
// instr_dealloc, and instr_clone_move.

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/util/statistics"
)

func instructionTable(t *testing.T, values ...Element) *HashTable {
	t.Helper()
	signature, err := NewSignature([]uint64{8})
	if err != nil {
		t.Fatal(err)
	}
	table := NewHashTable(signature)
	for _, value := range values {
		if _, err = table.Add(Fact{value}); err != nil {
			t.Fatal(err)
		}
	}
	return table
}

func TestInstructionBlockCloneMoveDeallocateOwnership(t *testing.T) {
	context := NewExecutionContext(nil)
	t.Cleanup(func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	})
	if err := context.SetRegister(0, instructionTable(t, 1, 2)); err != nil {
		t.Fatal(err)
	}
	if err := context.SetRegisterAnnotation(0, "seed"); err != nil {
		t.Fatal(err)
	}
	block := NewInstructionBlock()
	t.Cleanup(func() {
		if err := block.Close(); err != nil {
			t.Fatal(err)
		}
	})
	for _, instruction := range []Instruction{NewCloneInstruction(0, 1), NewMoveInstruction(1, 2), NewDeallocateInstruction(0)} {
		if err := block.Append(instruction); err != nil {
			t.Fatal(err)
		}
	}
	if err := block.MakeAnnotations(context); err != nil {
		t.Fatal(err)
	}
	complete, err := block.Perform(context)
	if err != nil || !complete {
		t.Fatal("instruction block", complete, err)
	}
	if value, _ := context.Register(0); value != nil {
		t.Fatal("deallocate retained source")
	}
	if value, _ := context.Register(1); value != nil {
		t.Fatal("move retained source")
	}
	value, err := context.Register(2)
	if err != nil || value == nil {
		t.Fatal("move lost target", err)
	}
	if count, err := value.Len(); err != nil || count != 2 {
		t.Fatal("clone/move rows", count, err)
	}
	if annotation, found, err := context.RegisterAnnotation(2); err != nil || !found || annotation != "seed" {
		t.Fatal("annotation", annotation, found, err)
	}
	if err = block.ProcessAllCosts(); err != nil {
		t.Fatal(err)
	}
	stats := statistics.New()
	if err = block.CollectStatistics(stats); err != nil || stats.Size() == 0 {
		t.Fatal("instruction statistics", stats.Size(), err)
	}
	var display bytes.Buffer
	if err = block.Display(&display); err != nil || display.String() != "clone 0 into 1\nmove 1 into 2\ndealloc 0\n" {
		t.Fatal("instruction display", display.String(), err)
	}
}

func TestInstructionBlockChecksTerminationBeforePerform(t *testing.T) {
	checks := 0
	context := NewExecutionContext(func() bool {
		checks++
		return checks == 2
	})
	t.Cleanup(func() { _ = context.Close() })
	if err := context.SetRegister(0, instructionTable(t, 3)); err != nil {
		t.Fatal(err)
	}
	block := NewInstructionBlock()
	t.Cleanup(func() { _ = block.Close() })
	if err := block.Append(NewCloneInstruction(0, 1)); err != nil {
		t.Fatal(err)
	}
	if err := block.Append(NewMoveInstruction(1, 2)); err != nil {
		t.Fatal(err)
	}
	complete, err := block.Perform(context)
	if err != nil || complete {
		t.Fatal("termination", complete, err)
	}
	if value, _ := context.Register(1); value == nil {
		t.Fatal("first instruction did not run")
	}
	if value, _ := context.Register(2); value != nil {
		t.Fatal("instruction ran after termination")
	}
}

func TestExecutionContextTimeoutReportingAndRegisterOwnership(t *testing.T) {
	context := NewExecutionContext(nil)
	first := instructionTable(t, 1)
	if err := context.SetRegister(0, first); err != nil {
		t.Fatal("set first register", err)
	}
	if err := context.SetRegister(0, instructionTable(t, 2)); err != nil {
		t.Fatal("replace register", err)
	}
	if _, err := first.Len(); err == nil {
		t.Fatal("replaced register table remained open")
	}
	if err := context.SetRegisterAnnotation(0, "current"); err != nil {
		t.Fatal("annotation", err)
	}
	if err := context.SetRegister(1, nil); err != nil {
		t.Fatal("set empty register", err)
	}
	if err := context.SetRegisterAnnotation(1, "empty"); err != nil {
		t.Fatal("empty register annotation", err)
	}
	if err := context.SetTimeLimit(100); err != nil {
		t.Fatal("set timeout", err)
	}
	if context.ShouldTerminate() {
		t.Fatal("fresh timeout already expired")
	}
	time.Sleep(110 * time.Millisecond)
	if !context.ShouldTerminate() {
		t.Fatal("expired timeout was ignored")
	}
	if err := context.ResetTimeLimit(); err != nil || context.ShouldTerminate() {
		t.Fatal("reset timeout", err)
	}

	var report bytes.Buffer
	if err := context.ReportBigRelations(^uint32(0), &report); err != nil {
		t.Fatal("report relation sizes", err)
	}
	maximum := ^uint32(0)
	want := fmt.Sprintf("bytes %d\nbytes\trows\tannotation\n%d\t%d\tcurrent\n", maximum, maximum, maximum)
	if report.String() != want {
		t.Fatalf("profile output\n got: %q\nwant: %q", report.String(), want)
	}
	report.Reset()
	if err := context.ReportBigRelations(0, &report); err != nil {
		t.Fatal("report empty registers", err)
	}
	want = fmt.Sprintf("bytes %d\nbytes\trows\tannotation\n%d\t%d\tcurrent\n0\t0\tempty\n", maximum, maximum, maximum)
	if report.String() != want {
		t.Fatalf("unfiltered profile output\n got: %q\nwant: %q", report.String(), want)
	}

	owned, _ := context.Register(0)
	if err := context.SetTimeLimit(1); err != nil {
		t.Fatal("set timeout before reset", err)
	}
	if err := context.Reset(); err != nil {
		t.Fatal("reset context", err)
	}
	if context.ShouldTerminate() {
		t.Fatal("reset did not clear the timeout")
	}
	if _, err := owned.Len(); err == nil {
		t.Fatal("reset did not close register table")
	}
	if err := context.Close(); err != nil {
		t.Fatal("close context", err)
	}
	closed := NewExecutionContext(nil)
	last := instructionTable(t, 3)
	if err := closed.SetRegister(0, last); err != nil {
		t.Fatal("set register before close", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal("close owned context", err)
	}
	if _, err := last.Len(); err == nil {
		t.Fatal("close did not retire register table")
	}
}

func TestExecutionContextRejectsZeroTimeout(t *testing.T) {
	context := NewExecutionContext(nil)
	t.Cleanup(func() { _ = context.Close() })
	if err := context.SetTimeLimit(0); err == nil {
		t.Fatal("zero timeout accepted")
	}
}
