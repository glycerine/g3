package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedOptimizeReplaySnapshotsAndLifecycle(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 538\n= 0x42\nR\nP 0x41\nP 0x42\nC 539\n" +
		"R\nP 0x41\nP 0x42\nS \"(declare-const y Int)(assert (>= y 0))(assert (<= y 3))(maximize y)\"\nC 557\n" +
		"R\nP 0x41\nP 0x42\nC 545\nR\nP 0x41\nC 54\n= 0x43\nR\nP 0x41\nP 0x42\nP 0x43\nC 541\n" +
		"R\nP 0x41\nC 33\n= 0x47\nR\nP 0x41\nI 1\nP 0x47\nC 150\n= 0x48\n" +
		"R\nP 0x41\nP 0x42\nP 0x48\nC 543\nR\nP 0x41\nP 0x42\nP 0x48\nC 544\nR\nP 0x41\nP 0x42\nC 546\n" +
		"R\nP 0x41\nP 0x42\nC 556\nR\nP 0x41\nP 0x42\nC 559\nR\nP 0x41\nP 0x42\nC 547\nR\nP 0x41\nP 0x42\nC 548\n" +
		"R\nP 0x41\nP 0x42\nC 549\n= 0x44\nR\nP 0x41\nP 0x44\nC 296\n" +
		"R\nP 0x41\nP 0x42\nC 561\n= 0x45\nR\nP 0x41\nP 0x45\nC 448\nR\nP 0x41\nP 0x42\nC 562\n= 0x46\nR\nP 0x41\nP 0x46\nC 448\n" +
		"R\nP 0x41\nP 0x42\nC 540\nR\nP 0x41\nP 0x45\nP 0x46\nP 0x44\nC 900\n" +
		"R\nP 0x41\nP 0x45\nC 449\nR\nP 0x41\nP 0x46\nC 449\nR\nP 0x41\nP 0x44\nC 297\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedModelCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-optimizer-snapshots", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos, want := range []uint32{2, 1} {
			v, e := replayVectorAt(r, owner, uint32(pos+1))
			if e != nil {
				return e
			}
			n, e := v.value.Len()
			if e != nil {
				return e
			}
			if n != want {
				t.Fatalf("snapshot %d length %d, want %d", pos, n, want)
			}
		}
		model, e := replayModelAt(r, owner, 3)
		if e != nil {
			return e
		}
		n, e := model.value.NumConstants()
		if e != nil {
			return e
		}
		if n != 1 {
			t.Fatal("optimal model constants", n)
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
		t.Fatal("optimizer snapshots were not checked")
	}
}

func TestGeneratedOptimizeReplayClosesIncompleteLog(t *testing.T) {
	r, err := NewZ3Replayer(strings.NewReader("R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 538\n= 0x42\nR\nP 0x41\nP 0x42\nC 539\n"), io.Discard)
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
	var o *replayOptimize
	for _, value := range r.optimizers {
		o = value
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if o == nil || !o.value.closed || !o.object.closed {
		t.Fatal("unreleased optimizer")
	}
}
