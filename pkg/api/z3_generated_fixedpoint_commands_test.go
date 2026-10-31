package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedFixedpointReplaySnapshotsAndLifecycle(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |R|\nU 0\np 0\nP 0x42\nC 49\n= 0x43\n" +
		"R\nP 0x41\nC 511\n= 0x44\nR\nP 0x41\nP 0x44\nC 512\nR\nP 0x41\nP 0x44\nP 0x43\nC 526\n" +
		"R\nP 0x41\nP 0x44\nC 536\nR\nP 0x41\nP 0x44\nC 537\n" +
		"R\nP 0x41\nP 0x44\nC 528\n= 0x45\nR\nP 0x41\nP 0x45\nC 448\nR\nP 0x41\nP 0x44\nC 529\n= 0x46\nR\nP 0x41\nP 0x46\nC 448\n" +
		"R\nP 0x41\nP 0x44\nC 525\n= 0x47\nR\nP 0x41\nP 0x47\nC 438\nR\nP 0x41\nP 0x44\nC 513\n" +
		"R\nP 0x41\nP 0x45\nP 0x46\nP 0x47\nC 900\nR\nP 0x41\nP 0x45\nC 449\nR\nP 0x41\nP 0x46\nC 449\nR\nP 0x41\nP 0x47\nC 439\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedSolverCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-fixedpoint-snapshots", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos := uint32(1); pos <= 2; pos++ {
			v, e := replayVectorAt(r, owner, pos)
			if e != nil {
				return e
			}
			size, e := v.value.Len()
			if e != nil {
				return e
			}
			if size != 0 {
				t.Fatal("nonempty snapshot", pos, size)
			}
		}
		id, e := r.Object(3)
		if e != nil {
			return e
		}
		stats := r.statistics[id]
		if stats == nil || stats.owner != owner {
			return ErrZ3ReplayReference
		}
		if _, e = stats.value.Size(); e != nil {
			return e
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
		t.Fatal("fixedpoint snapshots not checked")
	}
}

func TestGeneratedFixedpointReplayClosesIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 511\n= 0x42\nR\nP 0x41\nP 0x42\nC 512\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	var f *replayFixedpoint
	for _, value := range r.fixedpoints {
		f = value
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if f == nil || !f.value.closed || !f.object.closed {
		t.Fatal("unreleased fixedpoint")
	}
}
