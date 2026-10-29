package api

import (
	"bytes"
	"strings"
	"testing"
)

func TestZ3ReplayerResultHeapSurvivesReset(t *testing.T) {
	log := "R\nC 1\n= 0x10\nR\nP 10\nP 0\np 2\nC 2\n@ 20 0 0\nR\nP 20\nC 3\n"
	r, _ := NewZ3Replayer(strings.NewReader(log), new(bytes.Buffer))
	defer r.Close()
	if err := r.RegisterCommand(1, "result", func(r *Z3Replayer) error { return r.StoreResult(77) }); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterCommand(2, "array", func(r *Z3Replayer) error {
		values, e := r.ObjectArray(0)
		if e != nil || len(values) != 2 || values[0] != 77 || values[1] != 0 {
			t.Fatal(values, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterCommand(3, "saved", func(r *Z3Replayer) error {
		value, e := r.Object(0)
		if e != nil || value != 77 {
			t.Fatal(value, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Parse(); err != nil {
		t.Fatal(err)
	}
}

func TestZ3ReplayerRejectsUnknownPointer(t *testing.T) {
	r, _ := NewZ3Replayer(strings.NewReader("R\nP ff\n"), new(bytes.Buffer))
	defer r.Close()
	if err := r.Parse(); err == nil || !strings.Contains(err.Error(), "invalid pointer") {
		t.Fatal(err)
	}
}
