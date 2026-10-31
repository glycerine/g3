package api

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestGeneratedConfigReplayLifecycle(t *testing.T) {
	log := "R\nC 3\n= 0x42\nR\nP 0x42\nS \"proof\"\nS \"true\"\nC 5\nC 900\nR\nP 0x42\nC 4\n"
	r, err := NewZ3Replayer(strings.NewReader(log), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err = RegisterZ3GeneratedConfigCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(900, "check-config", func(r *Z3Replayer) error {
		config, e := replayConfig(r, 0)
		if e != nil {
			return e
		}
		state, e := config.params.State()
		if e != nil {
			return e
		}
		if !state.Proof {
			t.Fatal("replayed option not applied")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if len(r.configs) != 0 {
		t.Fatal("configuration not deleted", len(r.configs))
	}
	if _, err := replayConfig(r, 0); !errors.Is(err, ErrZ3ReplayReference) {
		t.Fatal("deleted configuration accepted", err)
	}
}

func TestGeneratedConfigReplayClosesUnreleasedConfigurations(t *testing.T) {
	r, err := NewZ3Replayer(strings.NewReader("R\nC 3\n= 0x42\n"), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedConfigCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	config := r.configs[1]
	if config == nil || config.params == nil {
		t.Fatal("configuration was not created")
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if config.params != nil {
		t.Fatal("unreleased configuration remains open")
	}
}
