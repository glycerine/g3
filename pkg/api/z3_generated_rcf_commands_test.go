package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedRCFRationalReplayZeroAndLifecycle(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nS \"0\"\nC 489\n= 0x0\nR\nP 0x41\nS \"3/2\"\nC 489\n= 0x42\n" +
		"R\nP 0x41\nI -2\nC 490\n= 0x43\nR\nP 0x41\nP 0x0\nP 0x42\nC 502\nR\nP 0x41\nP 0x42\nP 0x43\nC 503\n" +
		"R\nP 0x41\nP 0x42\nP 0x42\nC 504\nR\nP 0x41\nP 0x42\nP 0x0\nC 505\nR\nP 0x41\nP 0x0\nP 0x0\nC 506\nR\nP 0x41\nP 0x42\nP 0x43\nC 507\n" +
		"R\nP 0x41\nP 0x42\nI 1\nI 0\nC 508\nR\nP 0x41\nP 0x42\nU 5\nC 509\n" +
		"R\nP 0x41\nP 0x42\nP 0x43\nP 0x0\nC 900\nR\nP 0x41\nP 0x43\nC 488\nR\nP 0x41\nP 0x0\nC 488\n"
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
	checked := false
	if err = r.RegisterCommand(900, "check-rcf-values", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos, want := range []string{"3/2", "-2", "0"} {
			value, temporary, e := replayRCFAt(r, owner, uint32(pos+1))
			if e != nil {
				return e
			}
			got, e := owner.context.RCFString(value)
			if temporary {
				if x := value.Close(); x != nil {
					return x
				}
			}
			if e != nil {
				return e
			}
			if got != want {
				t.Fatal("rational value", got, want)
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
	if !checked || len(r.rcfNumbers) != 1 {
		t.Fatal("rational lifecycle", checked, len(r.rcfNumbers))
	}
	var leftover *replayRCFNumber
	for _, value := range r.rcfNumbers {
		leftover = value
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if leftover == nil || !leftover.value.closed {
		t.Fatal("unreleased rational")
	}
}

func TestGeneratedRCFRationalReplayArithmeticAndAliases(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nS \"3/2\"\nC 489\n= 0x42\nR\nP 0x41\nI 1\nC 490\n= 0x43\n" +
		"R\nP 0x41\nP 0x42\nP 0\nC 495\n= 0x42\nR\nP 0x41\nP 0x43\nP 0x42\nC 497\n= 0x42\nR\nP 0x41\nP 0x42\nP 0x43\nC 498\n= 0x42\n" +
		"R\nP 0x41\nP 0x42\nC 488\nR\nP 0x41\nP 0x42\nC 488\nR\nP 0x41\nP 0x42\nC 488\n" +
		"R\nP 0x41\nP 0x42\nP 0x43\nC 495\n= 0x44\nR\nP 0x41\nP 0x42\nP 0x43\nC 496\n= 0x45\n" +
		"R\nP 0x41\nP 0x42\nC 499\n= 0x46\nR\nP 0x41\nP 0x42\nC 500\n= 0x47\nR\nP 0x41\nP 0\nP 0\nC 498\n= 0x0\n" +
		"R\nP 0x41\nP 0x42\nP 0x44\nP 0x45\nP 0x46\nP 0x47\nP 0\nC 900\n" +
		"R\nP 0x41\nP 0x42\nC 488\nR\nP 0x41\nP 0x43\nC 488\nR\nP 0x41\nP 0x44\nC 488\nR\nP 0x41\nP 0x45\nC 488\nR\nP 0x41\nP 0x46\nC 488\nR\nP 0x41\nP 0x47\nC 488\n"
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
	if err = r.RegisterCommand(900, "check-rcf-arithmetic", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos, want := range []string{"3/2", "5/2", "1/2", "-3/2", "2/3", "0"} {
			value, temporary, e := replayRCFAt(r, owner, uint32(pos+1))
			if e != nil {
				return e
			}
			got, e := owner.context.RCFString(value)
			if temporary {
				if x := value.Close(); x != nil {
					return x
				}
			}
			if e != nil {
				return e
			}
			if got != want {
				t.Fatal("rational arithmetic", got, want)
			}
		}
		id, e := r.Object(1)
		if e != nil {
			return e
		}
		if r.rcfNumbers[id].refs != 1 {
			t.Fatal("alias count", r.rcfNumbers[id].refs)
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked || len(r.rcfNumbers) != 0 {
		t.Fatal("arithmetic lifecycle", checked, len(r.rcfNumbers))
	}
}

func TestGeneratedRCFRationalReplayPowersAndComponents(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nS \"-22/7\"\nC 489\n= 0x42\nR\nP 0x41\nI -3\nC 490\n= 0x43\n" +
		"R\nP 0x41\nP 0x42\nP 0\nP 0\nC 510\n* 0x44 2\n* 0x45 3\n" +
		"R\nP 0x41\nP 0x43\nP 0\nP 0\nC 510\n* 0x43 2\n* 0x47 3\n" +
		"R\nP 0x41\nP 0\nP 0\nP 0\nC 510\n* 0x0 2\n* 0x47 3\n" +
		"R\nP 0x41\nP 0x42\nU 0\nC 501\n= 0x47\nR\nP 0x41\nP 0x43\nU 1\nC 501\n= 0x43\n" +
		"R\nP 0x41\nP 0x43\nU 2\nC 501\n= 0x48\nR\nP 0x41\nP 0\nU 2\nC 501\n= 0x0\n" +
		"R\nP 0x41\nP 0x44\nP 0x45\nP 0x43\nP 0x47\nP 0x48\nC 900\n" +
		"R\nP 0x41\nP 0x42\nC 488\nR\nP 0x41\nP 0x43\nC 488\nR\nP 0x41\nP 0x43\nC 488\nR\nP 0x41\nP 0x43\nC 488\n" +
		"R\nP 0x41\nP 0x44\nC 488\nR\nP 0x41\nP 0x45\nC 488\nR\nP 0x41\nP 0x47\nC 488\nR\nP 0x41\nP 0x47\nC 488\nR\nP 0x41\nP 0x47\nC 488\nR\nP 0x41\nP 0x48\nC 488\n"
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
	if err = r.RegisterCommand(900, "check-rcf-components", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		for pos, want := range []string{"-22", "7", "-3", "1", "9"} {
			value, temporary, e := replayRCFAt(r, owner, uint32(pos+1))
			if e != nil {
				return e
			}
			if temporary {
				t.Fatal("unexpected null result")
			}
			got, e := owner.context.RCFString(value)
			if e != nil {
				return e
			}
			if got != want {
				t.Fatal("component or power", got, want)
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
	if !checked || len(r.rcfNumbers) != 1 {
		t.Fatal("expected only manager-owned one", checked, len(r.rcfNumbers))
	}
}
