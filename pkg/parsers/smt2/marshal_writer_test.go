package smt2

import (
	"errors"
	"testing"
)

type marshalFailWriter struct {
	limit int
	wrote int
	err   error
}

func (w *marshalFailWriter) Write(data []byte) (int, error) {
	remaining := w.limit - w.wrote
	if remaining <= 0 {
		return 0, w.err
	}
	if remaining < len(data) {
		w.wrote += remaining
		return remaining, w.err
	}
	w.wrote += len(data)
	return len(data), nil
}

func TestSMT2MarshalPropagatesWriterFailureAfterExactPrefix(t *testing.T) {
	_, manager, formula := marshalTestFormula(t, `(declare-const p Bool)(assert p)`)
	want := errors.New("transport stopped")
	writer := &marshalFailWriter{limit: 17, err: want}
	if err := Marshal(writer, formula, manager); !errors.Is(err, want) {
		t.Fatalf("writer error %v", err)
	}
	if writer.wrote != writer.limit {
		t.Fatalf("wrote %d, want %d", writer.wrote, writer.limit)
	}
}
