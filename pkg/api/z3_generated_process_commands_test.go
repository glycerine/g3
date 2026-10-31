package api

import (
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func TestGeneratedProcessReplayVersionAndWarnings(t *testing.T) {
	warnings := diagnostic.BorrowProcessWarningService()
	old := warnings.Enabled()
	defer func() {
		if err := warnings.Enable(old); err != nil {
			t.Error(err)
		}
	}()
	log := "R\nI 0\nC 327\nC 900\nR\nU 0\nU 0\nU 0\nU 0\nC 342\nC 901\nR\nC 343\nR\nI 1\nC 327\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = RegisterZ3GeneratedProcessCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-warnings", func(*Z3Replayer) error {
		if warnings.Enabled() {
			t.Fatal("warnings not disabled")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-version", func(r *Z3Replayer) error {
		v := Version()
		for i, want := range []uint32{v.Major, v.Minor, v.Build, v.Revision} {
			got, e := r.Uint(uint32(i))
			if e != nil {
				return e
			}
			if got != want {
				t.Fatal("version", i, got, want)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !warnings.Enabled() {
		t.Fatal("warnings not enabled")
	}
}

func TestGeneratedProcessReplayTraceControls(t *testing.T) {
	const name = "g3-replay-trace"
	tags := diagnostic.BorrowProcessTraceTags()
	DisableTrace(name)
	defer DisableTrace(name)
	log := "R\nS \"" + name + "\"\nC 344\nC 900\nC 345\nC 901\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = RegisterZ3GeneratedProcessCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-enabled", func(*Z3Replayer) error {
		if !tags.IsEnabled(name) {
			t.Fatal("trace not enabled")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-disabled", func(*Z3Replayer) error {
		if tags.IsEnabled(name) {
			t.Fatal("trace not disabled")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}
