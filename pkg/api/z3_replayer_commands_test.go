package api

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestZ3ReplayerCommandCanMutateOutArguments(t *testing.T) {
	r, _ := NewZ3Replayer(strings.NewReader("R\nI 1\nU 2\nP 0\nC 5\nC 6\n"), new(bytes.Buffer))
	defer r.Close()
	if err := r.RegisterCommand(5, "set", func(r *Z3Replayer) error {
		if e := r.SetInt64(0, -9); e != nil {
			return e
		}
		if e := r.SetUint64(1, 12); e != nil {
			return e
		}
		return r.SetObject(2, 44)
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterCommand(6, "read", func(r *Z3Replayer) error {
		i, _ := r.Int64(0)
		u, _ := r.Uint64(1)
		o, _ := r.Object(2)
		if i != -9 || u != 12 || o != 44 {
			t.Fatal(i, u, o)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Parse(); err != nil {
		t.Fatal(err)
	}
}

func TestZ3ReplayerCommandAndTypeErrorsArePositioned(t *testing.T) {
	r, _ := NewZ3Replayer(strings.NewReader("R\nU 1\nC 2\n"), new(bytes.Buffer))
	defer r.Close()
	if err := r.RegisterCommand(2, "bad", func(r *Z3Replayer) error { _, err := r.String(0); return err }); err != nil {
		t.Fatal(err)
	}
	err := r.Parse()
	var positioned *Z3ReplayError
	if !errors.As(err, &positioned) || positioned.Line != 3 || !strings.Contains(err.Error(), "expecting string") {
		t.Fatal(err)
	}
}

func TestZ3ReplayerRecoverableCommandExceptionContinues(t *testing.T) {
	var output bytes.Buffer
	r, _ := NewZ3Replayer(strings.NewReader("R\nC 1\nM \"after\"\n"), &output)
	defer r.Close()
	if err := r.RegisterCommand(1, "throws", func(*Z3Replayer) error {
		return &Z3ReplayCommandError{Message: "boom"}
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Parse(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "[z3 exception]: boom\nafter\n" {
		t.Fatal(output.String())
	}
}
