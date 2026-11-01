package rel

import (
	"bytes"
	"errors"
	"testing"
)

type boundDisplayFailWriter struct{ err error }

func (w boundDisplayFailWriter) Write([]byte) (int, error) { return 0, w.err }

func TestBoundRelationDisplayMatchesSourceLayout(t *testing.T) {
	relation := NewBoundRelation(3, false)
	if err := relation.AddLess(0, 2); err != nil {
		t.Fatal(err)
	}
	if err := relation.AddLessEqual(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := relation.Equate(1, 2); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := relation.Display(&output); err != nil {
		t.Fatal(err)
	}
	if want := "#0 < 2  <= 1 \n1 = 2 #2 < oo\n\n"; output.String() != want {
		t.Fatalf("source display\n got: %q\nwant: %q", output.String(), want)
	}
	output.Reset()
	if err := NewBoundRelation(2, true).Display(&output); err != nil {
		t.Fatal(err)
	}
	if output.String() != "empty\n" {
		t.Fatal("empty display", output.String())
	}
}

func TestBoundRelationDisplayPropagatesWriterFailure(t *testing.T) {
	failure := errors.New("display stopped")
	if err := NewBoundRelation(1, false).Display(boundDisplayFailWriter{err: failure}); !errors.Is(err, failure) {
		t.Fatal("display writer failure", err)
	}
}
