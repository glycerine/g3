package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedQuantifierInspectionReplayBorrowedASTs(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\nU 0\nP 0x42\nC 191\n= 0x43\n" +
		"R\nP 0x41\n$ |f|\nU 1\nP 0x42\np 1\nP 0x42\nC 49\n= 0x44\n" +
		"R\nP 0x41\nP 0x44\nU 1\nP 0x43\np 1\nC 50\n= 0x45\n" +
		"R\nP 0x41\nU 1\nP 0x45\np 1\nC 190\n= 0x46\n" +
		"R\nP 0x41\nU 5\nU 1\nP 0x46\np 1\nU 1\nP 0x42\np 1\n# 7\ns 1\nP 0x45\nC 192\n= 0x47\n" +
		"R\nP 0x41\nC 901\n" +
		"R\nP 0x41\nP 0x47\nU 0\nC 280\n= 0x48\n" +
		"R\nP 0x41\nP 0x47\nC 283\nR\nP 0x41\nP 0x47\nU 0\nC 284\n" +
		"R\nP 0x41\nP 0x47\nU 0\nC 285\n= 0x53\nR\nP 0x41\nP 0x47\nC 286\n= 0x54\n" +
		"R\nP 0x41\nP 0x48\nC 273\n= 0x49\n" +
		"R\nP 0x41\nP 0x48\nC 274\n" +
		"R\nP 0x41\nP 0x48\nU 0\nC 275\n= 0x50\n" +
		"R\nP 0x41\nC 902\n" +
		"R\nP 0x41\nP 0x43\nC 276\n" +
		"R\nP 0x41\nP 0x47\nC 277\nR\nP 0x41\nP 0x47\nC 278\nR\nP 0x41\nP 0x47\nC 279\nR\nP 0x41\nP 0x47\nC 281\n" +
		"R\nP 0x41\nI 1\nU 2\nN\nN\nU 0\np 0\nU 1\nP 0x45\np 1\nU 1\nP 0x42\np 1\n$ |x|\ns 1\nP 0x45\nC 195\n= 0x51\n" +
		"R\nP 0x41\nP 0x51\nU 0\nC 282\n= 0x52\n" +
		"R\nP 0x41\nP 0x46\nP 0x48\nP 0x49\nP 0x45\nP 0x50\nP 0x52\nP 0x42\nP 0x53\nP 0x54\nC 900\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	trailSize := -1
	if err = r.RegisterCommand(901, "capture-trail", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		trailSize = len(owner.results.astTrail)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(902, "check-trail", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		if trailSize < 0 || len(owner.results.astTrail) != trailSize {
			t.Fatal("borrowed inspection added an owned AST trail")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-borrowed-results", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		values := make([]*replayASTOwner, 9)
		for i := range values {
			values[i], e = r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
		}
		if values[0].handle != values[1].handle || values[0].handle != values[2].handle {
			t.Fatal("pattern identity changed")
		}
		if values[3].handle != values[4].handle || values[3].handle != values[5].handle {
			t.Fatal("pattern or no-pattern term identity changed")
		}
		if values[6].handle != values[7].handle || values[3].handle != values[8].handle {
			t.Fatal("bound sort or body identity changed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}
