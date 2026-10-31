package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedTacticApplyReplayAndSubgoalOwnership(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\nR\nP 0x41\n$ |p|\nP 0x42\nC 51\n= 0x43\n" +
		"R\nP 0x41\nI 1\nI 0\nI 0\nC 348\n= 0x44\nR\nP 0x41\nP 0x44\nC 349\nR\nP 0x41\nP 0x44\nP 0x43\nC 352\n" +
		"R\nP 0x41\nC 377\n= 0x45\nR\nP 0x41\nP 0x45\nC 364\nR\nP 0x41\nC 13\n= 0x46\nR\nP 0x41\nP 0x46\nC 14\n" +
		"R\nP 0x41\nP 0x45\nP 0x44\nC 400\n= 0x47\nR\nP 0x41\nP 0x47\nC 402\n" +
		"R\nP 0x41\nP 0x45\nP 0x44\nP 0x46\nC 401\n= 0x48\nR\nP 0x41\nP 0x48\nC 402\n" +
		"R\nP 0x41\nP 0x47\nC 404\nR\nP 0x41\nP 0x47\nC 405\nR\nP 0x41\nP 0x47\nU 0\nC 406\n= 0x49\nR\nP 0x41\nP 0x49\nC 349\n" +
		"R\nP 0x41\nC 295\n= 0x50\nR\nP 0x41\nP 0x50\nC 296\nR\nP 0x41\nP 0x47\nU 0\nP 0x50\nC 407\n= 0x51\nR\nP 0x41\nP 0x51\nC 296\n" +
		"R\nP 0x41\nP 0x47\nC 403\nR\nP 0x41\nP 0x48\nC 403\nR\nP 0x41\nP 0x45\nC 365\nR\nP 0x41\nP 0x44\nC 350\nR\nP 0x41\nP 0x50\nC 297\nR\nP 0x41\nP 0x46\nC 15\n" +
		"R\nP 0x41\nP 0x49\nP 0x51\nC 900\nR\nP 0x41\nP 0x49\nC 350\nR\nP 0x41\nP 0x51\nC 297\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	for _, reg := range []func(*Z3Replayer) error{RegisterZ3GeneratedContextCommands, RegisterZ3GeneratedParameterCommands, RegisterZ3GeneratedASTCommands, RegisterZ3GeneratedModelCommands} {
		if err = reg(r); err != nil {
			t.Fatal(err)
		}
	}
	checked := false
	if err = r.RegisterCommand(900, "check-application-products", func(r *Z3Replayer) error {
		g, e := replayGetGoal(r)
		if e != nil {
			return e
		}
		model, e := replayModelAt(r, g.owner, 2)
		if e != nil {
			return e
		}
		size, e := g.value.Size()
		if e != nil {
			return e
		}
		if size != 1 {
			t.Fatal("subgoal size", size)
		}
		if _, e = model.value.NumConstants(); e != nil {
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
		t.Fatal("application products not checked")
	}
}
