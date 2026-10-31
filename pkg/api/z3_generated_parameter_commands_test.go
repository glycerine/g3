package api

import (
	"io"
	"strings"
	"testing"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestGeneratedParameterReplayValuesAndLifetime(t *testing.T) {
	log := "R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 13\n= 0x42\nR\nP 0x41\nP 0x42\nC 14\n" +
		"R\nP 0x41\nP 0x42\n$ |:ENABLED|\nI 1\nC 16\n" +
		"R\nP 0x41\nP 0x42\n$ |random-seed|\nU 17\nC 17\n" +
		"R\nP 0x41\nP 0x42\n$ |ratio|\nD 1.25\nC 18\n" +
		"R\nP 0x41\nP 0x42\n$ |mode|\n$ |fast|\nC 19\n" +
		"R\nP 0x41\nP 0x42\nC 20\nR\nP 0x41\nP 0x42\nC 900\n" +
		"R\nP 0x41\nC 13\n= 0x43\nR\nP 0x41\nP 0x42\nC 15\nR\nP 0x41\nP 0x42\nC 901\nR\nP 0x41\nC 8\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedParameterCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-params", func(r *Z3Replayer) error {
		p, err := replayParameters(r)
		if err != nil {
			return err
		}
		name := "enabled"
		b, err := p.value.params.GetBoolName(&p.value.value, &name, false)
		if err != nil || !b {
			t.Fatalf("bool: %v %v", b, err)
		}
		name = "random_seed"
		u, err := p.value.params.GetUIntName(&p.value.value, &name, 0)
		if err != nil || u != 17 {
			t.Fatalf("uint: %v %v", u, err)
		}
		name = "ratio"
		d, err := p.value.params.GetDoubleName(&p.value.value, &name, 0)
		if err != nil || d != 1.25 {
			t.Fatalf("double: %v %v", d, err)
		}
		name = "mode"
		s, err := p.value.params.GetSymbolName(&p.value.value, &name, usymbol.Symbol{})
		nameValue, textErr := p.value.params.Symbols().String(s)
		if err != nil || textErr != nil || nameValue != "fast" {
			t.Fatalf("symbol: %v %v", s, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(901, "check-expired", func(r *Z3Replayer) error {
		if _, err := replayParameters(r); err != ErrZ3ReplayReference {
			t.Fatalf("expired parameters: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if len(r.parameterSets) != 0 || len(r.contexts) != 0 {
		t.Fatal("objects were not cleaned up")
	}
}

func TestGeneratedParameterReplayCleansIncompleteLog(t *testing.T) {
	log := "R\nP 0\nC 7\n= 0x41\nR\nP 0x41\nC 13\n= 0x42\nR\nP 0x41\nP 0x42\nC 14\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedParameterCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	var p *replayParameterSet
	for _, entry := range r.parameterSets {
		p = entry
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if p == nil || !p.value.closed || !p.object.closed {
		t.Fatal("parameter set was not closed")
	}
}
