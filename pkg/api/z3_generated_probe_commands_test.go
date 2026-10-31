package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedProbeReplayLifecycleAndApplication(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nI 0\nI 0\nI 0\nC 348\n= 0x42\nR\nP 0x41\nP 0x42\nC 349\n" +
		"R\nP 0x41\nS \"size\"\nC 366\n= 0x43\nR\nP 0x41\nP 0x43\nC 367\nR\nP 0x41\nD 2.5\nC 382\n= 0x44\nR\nP 0x41\nP 0x44\nC 367\n" +
		"R\nP 0x41\nP 0x43\nP 0x42\nC 399\nR\nP 0x41\nP 0x44\nP 0x42\nC 399\nR\nP 0x41\nP 0x44\nP 0x42\nC 900\n" +
		"R\nP 0x41\nP 0x43\nC 368\nR\nP 0x41\nP 0x44\nC 368\nR\nP 0x41\nP 0x42\nC 350\nR\nP 0x41\nC 8\n"
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
	var checked *replayProbe
	if err = r.RegisterCommand(900, "check-probe", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		p, e := replayProbeAt(r, owner, 1)
		if e != nil {
			return e
		}
		g, e := replayGoalAt(r, owner, 2)
		if e != nil {
			return e
		}
		value, e := p.value.Apply(g.value)
		if e != nil {
			return e
		}
		if value != 2.5 {
			t.Fatal("probe value", value)
		}
		checked = p
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if checked == nil || !checked.value.closed || len(r.probes) != 0 {
		t.Fatal("probe cleanup")
	}
}

func TestGeneratedProbeReplayClosesIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nD 1.0\nC 382\n= 0x42\nR\nP 0x41\nP 0x42\nC 367\n"
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
	var p *replayProbe
	for _, value := range r.probes {
		p = value
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if p == nil || !p.value.closed || !p.object.closed {
		t.Fatal("unreleased probe")
	}
}
