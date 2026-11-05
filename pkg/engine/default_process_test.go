package engine

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Source: generated mem_initialize/finalize and debug.cpp:28-35,37-117.
// The API's generated debug owner must carry the same default borrowed
// streams and action service while it replaces the process assertion pointer.
func TestDefaultProcessBindsAssertionServicesAndReleasesOwners(t *testing.T) {
	previous := diagnostic.BorrowProcessDebugContext()
	if err := InitializeDefaultProcess(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := FinalizeDefaultProcess(); err != nil {
			t.Error(err)
		}
	})
	d := apiDefaultProcess
	if d == nil || d.debug == nil || d.streams == nil || d.input == nil || d.actions == nil ||
		diagnostic.BorrowProcessDebugContext() != d.debug || !d.debug.HasAssertionActions() {
		t.Fatal("default generated assertion owner is incomplete")
	}
	stdout, err := d.streams.Stdout()
	if err != nil {
		t.Fatal(err)
	}
	if tied, err := d.input.Tie(); err != nil || tied != stdout {
		t.Fatal("debugger input lost the actual standard output tie", tied, err)
	}
	oldBudget := d.budget
	if err := ResetDefaultProcess(); err != nil {
		t.Fatal(err)
	}
	reset := apiDefaultProcess
	if reset == nil || reset == d || oldBudget.Used() != 0 {
		t.Fatal("memory reset did not replace and release the generated process", reset, oldBudget.Used())
	}
	resetBudget := reset.budget
	if err := FinalizeDefaultProcess(); err != nil {
		t.Fatal(err)
	}
	if apiDefaultProcess != nil || diagnostic.BorrowProcessDebugContext() != previous || oldBudget.Used() != 0 || resetBudget.Used() != 0 {
		t.Fatal("default generated owners or process binding survived close", oldBudget.Used(), resetBudget.Used())
	}
}
