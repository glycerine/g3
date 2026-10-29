package api

import (
	"bytes"
	"strings"
	"testing"
)

func TestZ3ReplayerLifecycleAndRegistrationGuards(t *testing.T) {
	if r, err := NewZ3Replayer(nil, new(bytes.Buffer)); err != ErrZ3ReplayState || r != nil {
		t.Fatal(r, err)
	}
	r, err := NewZ3Replayer(strings.NewReader(""), new(bytes.Buffer))
	if err != nil {
		t.Fatal(err)
	}
	if err = r.RegisterCommand(0, "nil", nil); err != ErrZ3ReplayState {
		t.Fatal(err)
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != ErrZ3ReplayState {
		t.Fatal(err)
	}
	if err = r.StoreResult(1); err != ErrZ3ReplayState {
		t.Fatal(err)
	}
}
