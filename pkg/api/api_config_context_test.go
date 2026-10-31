package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/engine"
)

func TestSMTContextCopiesInitialConfig(t *testing.T) {
	config, err := MkConfig()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if config.params != nil {
			if err := config.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := engine.FinalizeDefaultProcess(); err != nil {
			t.Error(err)
		}
	}()
	if err = SetParamValue(config, "proof", "true"); err != nil {
		t.Fatal(err)
	}
	context, err := NewSMTContextWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err = SetParamValue(config, "proof", "false"); err != nil {
		t.Fatal(err)
	}
	if err = config.Close(); err != nil {
		t.Fatal(err)
	}
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	proofs, err := manager.ProofsEnabled()
	if err != nil || !proofs {
		t.Fatal("config snapshot", proofs, err)
	}
	if _, err = NewSMTContextWithConfig(config); err != ErrConfigOwner {
		t.Fatal("closed config accepted", err)
	}
	defaults, err := NewSMTContextWithConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := defaults.Close(); err != nil {
			t.Error(err)
		}
	}()
	manager, err = defaults.Manager()
	if err != nil {
		t.Fatal(err)
	}
	proofs, err = manager.ProofsEnabled()
	if err != nil || proofs {
		t.Fatal("default context proofs", proofs, err)
	}
}
