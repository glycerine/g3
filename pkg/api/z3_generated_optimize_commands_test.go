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

func TestGeneratedOptimizeReplayBoundsAndMetadata(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 538\n= 0x42\nR\nP 0x41\nP 0x42\nC 539\n" +
		"R\nP 0x41\nC 13\n= 0x43\nR\nP 0x41\nP 0x43\nC 14\nR\nP 0x41\nP 0x42\nP 0x43\nC 550\nR\nP 0x41\nP 0x43\nC 15\n" +
		"R\nP 0x41\nP 0x42\nC 551\n= 0x44\nR\nP 0x41\nP 0x44\nC 22\n" +
		"R\nP 0x41\nP 0x42\nS \"(declare-const x Int)(assert (>= x 0))(assert (<= x 3))(maximize x)\"\nC 557\nR\nP 0x41\nP 0x42\nC 547\n" +
		"R\nP 0x41\nP 0x42\nU 0\nC 552\n= 0x45\nR\nP 0x41\nP 0x42\nU 0\nC 553\n= 0x46\n" +
		"R\nP 0x41\nP 0x42\nU 0\nC 554\n= 0x47\nR\nP 0x41\nP 0x47\nC 448\n" +
		"R\nP 0x41\nP 0x42\nU 0\nC 555\n= 0x48\nR\nP 0x41\nP 0x48\nC 448\n" +
		"R\nP 0x41\nP 0x42\nC 560\n= 0x49\nR\nP 0x41\nP 0x49\nC 438\nR\nP 0x41\nP 0x42\nC 540\n" +
		"R\nP 0x41\nP 0x45\nP 0x46\nP 0x47\nP 0x48\nP 0x44\nP 0x49\nC 900\n" +
		"R\nP 0x41\nP 0x47\nC 449\nR\nP 0x41\nP 0x48\nC 449\nR\nP 0x41\nP 0x44\nC 23\nR\nP 0x41\nP 0x49\nC 439\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedParameterCommands, RegisterZ3GeneratedSolverCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-optimizer-bounds", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos := uint32(1); pos <= 2; pos++ {
			value, e := r.astObject(owner, pos)
			if e != nil {
				return e
			}
			text, _, ok, e := owner.results.manager.ArithmeticNumeralText(value.handle)
			if e != nil {
				return e
			}
			if !ok || text != "3" {
				t.Fatalf("bound %d: %q", pos, text)
			}
		}
		for pos := uint32(3); pos <= 4; pos++ {
			vector, e := replayVectorAt(r, owner, pos)
			if e != nil {
				return e
			}
			n, e := vector.value.Len()
			if e != nil {
				return e
			}
			if n != 3 {
				t.Fatal("bound vector length", n)
			}
		}
		descriptions, e := replayDescriptions(r, 5, owner)
		if e != nil {
			return e
		}
		if n, e := descriptions.value.Size(); e != nil || n == 0 {
			t.Fatal("optimizer descriptions", n, e)
		}
		id, e := r.Object(6)
		if e != nil {
			return e
		}
		if r.statistics[id] == nil || r.statistics[id].owner != owner {
			return ErrZ3ReplayReference
		}
		if _, e = r.statistics[id].value.Size(); e != nil {
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
		t.Fatal("optimizer bounds were not checked")
	}
}
