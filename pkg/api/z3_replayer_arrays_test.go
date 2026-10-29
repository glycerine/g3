package api

import (
	"bytes"
	"strings"
	"testing"
)

func TestZ3ReplayerTypedArraysPreserveSourceOrder(t *testing.T) {
	log := "R\nU 4\nU 9\nu 2\n$ |a|\nN\n$ |c|\ns 3\nC 1\n"
	r, _ := NewZ3Replayer(strings.NewReader(log), new(bytes.Buffer))
	defer r.Close()
	if err := r.RegisterCommand(1, "arrays", func(r *Z3Replayer) error {
		u, e := r.UintArray(0)
		if e != nil || len(u) != 2 || u[0] != 4 || u[1] != 9 {
			t.Fatal(u, e)
		}
		s, e := r.SymbolArray(1)
		if e != nil || len(s) != 3 || s[0] != "a" || s[1] != "" || s[2] != "c" {
			t.Fatal(s, e)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Parse(); err != nil {
		t.Fatal(err)
	}
}

func TestZ3ReplayerRejectsMixedAndOversizedArrays(t *testing.T) {
	for _, log := range []string{"R\nU 1\nI 2\nu 2\n", "R\nu 1\n"} {
		r, _ := NewZ3Replayer(strings.NewReader(log), new(bytes.Buffer))
		err := r.Parse()
		_ = r.Close()
		if err == nil {
			t.Fatalf("accepted %q", log)
		}
	}
}
