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
