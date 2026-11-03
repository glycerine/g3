package rel

import (
	"bytes"
	"errors"
	"testing"
)

func TestLazyTableDisplayForcesDeferredIteratorOwner(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	base := newLazyTableWithFacts(t, engine, lazySignature(t, 3), Fact{1}, Fact{2})
	projected, err := base.Project(nil)
	if err != nil {
		t.Fatal(err)
	}
	if projected.Forced() {
		t.Fatal("project forced eagerly")
	}
	var output bytes.Buffer
	if err = projected.Display(&output); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "table with signature (3):\n(1)\n(2)\n\n"; got != want {
		t.Fatalf("display got %q, want %q", got, want)
	}
	if !projected.Forced() {
		t.Fatal("display did not force lazy iterator owner")
	}
	if facts, err := base.Facts(); err != nil || len(facts) != 2 {
		t.Fatal("display changed source owner", facts, err)
	}
}

func TestLazyTableDisplayPropagatesConcreteWriterFailure(t *testing.T) {
	engine := NewLazyTableEngine()
	t.Cleanup(func() { _ = engine.Close() })
	table := newLazyTableWithFacts(t, engine, lazySignature(t, 2), Fact{1})
	refusal := errors.New("lazy display writer refusal")
	if err := table.Display(tableDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatal("display ignored concrete writer refusal", err)
	}
}
