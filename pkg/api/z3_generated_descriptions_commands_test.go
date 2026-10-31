package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedParameterDescriptionsReplay(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 13\n= 0x42\nR\nP 0x41\nP 0x42\nC 14\n" +
		"R\nP 0x41\nP 0x42\n$ |timeout|\nU 100\nC 17\nR\nP 0x41\nC 408\n= 0x43\nR\nP 0x41\nP 0x43\nC 416\n" +
		"R\nP 0x41\nP 0x43\nC 414\n= 0x44\nR\nP 0x41\nP 0x44\nC 22\n" +
		"R\nP 0x41\nP 0x42\nP 0x44\nC 21\nR\nP 0x41\nP 0x44\n$ |timeout|\nC 24\n" +
		"R\nP 0x41\nP 0x44\nC 25\nR\nP 0x41\nP 0x44\nU 0\nC 26\nR\nP 0x41\nP 0x44\n$ |timeout|\nC 27\nR\nP 0x41\nP 0x44\nC 28\n" +
		"R\nP 0x41\nP 0x44\nC 900\nR\nP 0x41\nP 0x44\nC 23\nR\nP 0x41\nP 0x43\nC 417\nR\nP 0x41\nP 0x42\nC 15\nR\nP 0x41\nC 8\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, register := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedParameterCommands, RegisterZ3GeneratedSolverCommands} {
		if err = register(r); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.RegisterCommand(900, "check-descriptions", func(r *Z3Replayer) error {
		d, e := replayDescriptionArgument(r)
		if e != nil {
			return e
		}
		count, e := d.value.Size()
		if e != nil || count == 0 {
			t.Fatalf("description count %d: %v", count, e)
		}
		kind, e := d.value.Kind("timeout")
		if e != nil || kind != ParameterUInt {
			t.Fatalf("timeout kind %v: %v", kind, e)
		}
		name, e := d.value.symbols.Intern("TIMEOUT")
		if e != nil {
			return e
		}
		exact, e := d.value.value.Kind(name)
		if e != nil {
			return e
		}
		if apiParameterKind(exact) != ParameterInvalid {
			t.Fatal("descriptor lookup changed symbol case")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if len(r.descriptions) != 0 || len(r.parameterSets) != 0 || len(r.solvers) != 0 {
		t.Fatal("descriptor objects were not cleaned up")
	}
}
