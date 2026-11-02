package pdr

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type pdrDisplayFailWriter struct{ remaining int }

func (w *pdrDisplayFailWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestPredicateTransformerWriteDisplay(t *testing.T) {
	for _, mode := range []int{inductionTestEmpty, inductionTestFact, inductionTestRecursive} {
		_, owner, transformer, _, closeBase := inductionTestTransformer(t, mode)
		var out bytes.Buffer
		closureMust(t, transformer.WriteDisplay(&out))
		want := []string{
			"transition\nfalse\n",
			"rules\n(forall ((A Real)) (closure_predicate A))\ntransition\nclosure_predicate!0\n",
			"rules\n(forall ((A Real)) (=> (closure_predicate A) (closure_predicate A)))\ntransition\n(and (= closure_predicate_0_0 closure_predicate_0_n) closure_predicate!0)\n",
		}[mode]
		if out.String() != want {
			t.Fatalf("mode %d: got %q, want %q", mode, out.String(), want)
		}
		for _, limit := range []int{0, 6, len(want) - 1} {
			err := transformer.WriteDisplay(&pdrDisplayFailWriter{remaining: limit})
			if !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("writer failure lost: %v", err)
			}
			out.Reset()
			closureMust(t, transformer.WriteDisplay(&out))
			if out.String() != want {
				t.Fatal("writer failure changed transformer")
			}
		}
		if err := transformer.WriteDisplay(nil); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
		inductionCloseFixture(t, owner, transformer, closeBase)
		if err := transformer.WriteDisplay(&out); !errors.Is(err, ErrPredicateTransformerOwner) {
			t.Fatal("closed transformer display", err)
		}
	}
}
