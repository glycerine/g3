package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedQEReplayProjectionAndVectorMutation(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |p|\nP 0x42\nC 51\n= 0x43\n" +
		"R\nP 0x41\n$ |unused|\nP 0x42\nC 51\n= 0x44\nR\nP 0x41\nC 408\n= 0x45\nR\nP 0x41\nP 0x45\nC 416\n" +
		"R\nP 0x41\nP 0x45\nP 0x43\nC 422\nR\nP 0x41\nP 0x45\nC 427\nR\nP 0x41\nP 0x45\nC 431\n= 0x46\nR\nP 0x41\nP 0x46\nC 296\n" +
		"R\nP 0x41\nC 457\n= 0x47\nR\nP 0x41\nP 0x47\nC 458\nR\nP 0x41\nC 447\n= 0x48\nR\nP 0x41\nP 0x48\nC 448\nR\nP 0x41\nP 0x48\nP 0x44\nC 454\n" +
		"R\nP 0x41\nP 0x46\nU 1\nP 0x43\np 1\nP 0x43\nC 657\n= 0x49\n" +
		"R\nP 0x41\nP 0x46\nU 1\nP 0x43\np 1\nP 0x43\nP 0x47\nC 658\n= 0x50\n" +
		"R\nP 0x41\nP 0x46\nP 0x43\nC 659\n= 0x51\nR\nP 0x41\nP 0x48\nP 0x43\nC 660\n= 0x52\n" +
		"R\nP 0x41\nP 0x46\nC 297\nR\nP 0x41\nP 0x45\nC 417\nR\nP 0x41\nP 0x49\nP 0x50\nP 0x51\nP 0x52\nP 0x43\nP 0x48\nP 0x47\nC 900\n" +
		"R\nP 0x41\nP 0x48\nC 449\nR\nP 0x41\nP 0x47\nC 459\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedSolverCommands, RegisterZ3GeneratedModelCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-projections", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos := uint32(1); pos <= 2; pos++ {
			value, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			if !owner.results.manager.IsTrue(value.handle) {
				t.Fatal("model projection was not true", pos)
			}
		}
		p, e := r.astObject(owner, 5)
		if e != nil {
			return e
		}
		for pos := uint32(3); pos <= 4; pos++ {
			value, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			if value.handle != p.handle {
				t.Fatal("unrelated literal changed", pos)
			}
		}
		vector, e := replayVectorAt(r, owner, 6)
		if e != nil {
			return e
		}
		n, e := vector.value.Len()
		if e != nil {
			return e
		}
		if n != 0 {
			t.Fatal("unused variable remains", n)
		}
		mapID, e := r.Object(7)
		if e != nil {
			return e
		}
		mapping := r.maps[mapID]
		if mapping == nil || mapping.owner != owner {
			return ErrZ3ReplayReference
		}
		n, e = mapping.value.Size()
		if e != nil {
			return e
		}
		if n != 0 {
			t.Fatal("unexpected Boolean skolem mapping", n)
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("projection results not checked")
	}
}
