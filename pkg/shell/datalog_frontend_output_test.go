package shell

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDatalogFrontendLifecycleAndAnswerAdapter(t *testing.T) {
	context := datalogFrontendContext(t)
	var stats bytes.Buffer
	frontend, err := NewDatalogFrontend(context, &stats)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = frontend.Results(); !errors.Is(err, ErrDatalogFrontend) {
		t.Fatal("results before run", err)
	}
	if err = frontend.Saturate(); !errors.Is(err, ErrDatalogFrontend) {
		t.Fatal("saturate before load", err)
	}
	path := filepath.Join(t.TempDir(), "empty.dl")
	if err = os.WriteFile(path, []byte("\nP\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = frontend.Load(path); err != nil {
		t.Fatal(err)
	}
	if err = frontend.Load(path); !errors.Is(err, ErrDatalogFrontend) {
		t.Fatal("duplicate load", err)
	}
	if err = frontend.Saturate(); err != nil {
		t.Fatal(err)
	}
	var answers bytes.Buffer
	if err = frontend.WriteAnswers(&answers); err != nil {
		t.Fatal(err)
	}
	if answers.String() != "P: unsat\n" {
		t.Fatal("answers", answers.String())
	}
	if err = frontend.Saturate(); !errors.Is(err, ErrDatalogFrontend) {
		t.Fatal("duplicate saturation", err)
	}
}
