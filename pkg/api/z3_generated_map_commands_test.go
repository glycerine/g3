package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedASTMapReplayKeySnapshot(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 32\n= 0x42\n" +
		"R\nP 0x41\n$ |x|\nP 0x42\nC 51\n= 0x43\nR\nP 0x41\n$ |y|\nP 0x42\nC 51\n= 0x44\n" +
		"R\nP 0x41\nC 457\n= 0x45\nR\nP 0x41\nP 0x45\nC 458\n" +
		"R\nP 0x41\nP 0x45\nP 0x43\nP 0x44\nC 462\nR\nP 0x41\nP 0x45\nP 0x44\nP 0x43\nC 462\n" +
		"R\nP 0x41\nP 0x45\nP 0x43\nC 460\nR\nP 0x41\nP 0x45\nP 0x43\nC 461\n= 0x46\n" +
		"R\nP 0x41\nP 0x45\nP 0x44\nC 463\nR\nP 0x41\nP 0x45\nC 465\nR\nP 0x41\nP 0x45\nC 467\n" +
		"R\nP 0x41\nP 0x45\nC 466\n= 0x47\nR\nP 0x41\nP 0x47\nC 448\n" +
		"R\nP 0x41\nP 0x45\nC 464\nR\nP 0x41\nP 0x45\nC 459\nR\nP 0x41\nP 0x47\nU 0\nC 451\n= 0x48\n" +
		"R\nP 0x41\nP 0x47\nP 0x43\nP 0x48\nP 0x44\nP 0x46\nC 900\nR\nP 0x41\nP 0x47\nC 449\n"
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
	checked := false
	if err = r.RegisterCommand(900, "check-map-snapshot", func(r *Z3Replayer) error {
		v, e := replayGetVector(r)
		if e != nil {
			return e
		}
		n, e := v.value.Len()
		if e != nil {
			return e
		}
		if n != 1 {
			t.Fatal("key count", n)
		}
		for pos := uint32(2); pos <= 4; pos += 2 {
			a, e := r.astObject(v.owner, pos)
			if e != nil {
				return e
			}
			b, e := r.astObject(v.owner, pos+1)
			if e != nil {
				return e
			}
			if a.handle != b.handle {
				t.Fatal("map result", pos)
			}
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
		t.Fatal("map snapshot not checked")
	}
}

func TestGeneratedASTMapReplayClosesIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 457\n= 0x42\nR\nP 0x41\nP 0x42\nC 458\n"
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
	var m *replayASTMap
	for _, value := range r.maps {
		m = value
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if m == nil || m.value.owner != nil || !m.object.closed {
		t.Fatal("unreleased map")
	}
}
