package api

import (
	"io"
	"strings"
	"testing"
)

func TestGeneratedSymbolReplayInternsNames(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nS \"generated-symbol-name\"\nC 30\nC 900\nR\nP 0x41\nI 17\nC 29\nR\nP 0x41\nC 8\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedSymbolCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-symbol", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		m, e := owner.context.Manager()
		if e != nil {
			return e
		}
		before := m.Symbols.Stats().Symbols
		_, e = m.Symbols.Intern("generated-symbol-name")
		if e != nil {
			return e
		}
		if m.Symbols.Stats().Symbols != before {
			t.Fatal("symbol was not interned")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
}
