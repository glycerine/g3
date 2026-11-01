package text

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

type numericWriterProbe struct {
	bytes.Buffer
	flushes     int
	refuse      bool
	refuseFlush bool
}

func (w *numericWriterProbe) Write(p []byte) (int, error) {
	if w.refuse {
		return 0, errors.New("write refused")
	}
	return w.Buffer.Write(p)
}
func (w *numericWriterProbe) Flush() error {
	w.flushes++
	if w.refuseFlush {
		return errors.New("flush refused")
	}
	return nil
}

func TestNumericStreamBorrowedWriterFormattingAndFailure(t *testing.T) {
	budget := arena.NewBudget(1 << 20)
	var missing *numericWriterProbe
	if _, err := NewNumericTextStream(NumericTextStreamConfig{Budget: budget, BorrowedWriter: missing}); !errors.Is(err, ErrNumericStreamConfig) {
		t.Fatal("nil borrowed writer", err)
	}
	w := &numericWriterProbe{}
	s, err := NewNumericTextStream(NumericTextStreamConfig{Budget: budget, BorrowedWriter: w})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetWidth(4); err != nil {
		t.Fatal(err)
	}
	if err = s.AppendInt32(12); err != nil {
		t.Fatal(err)
	}
	if err = s.Flush(); err != nil {
		t.Fatal(err)
	}
	if w.String() != "  12" || w.flushes != 1 || s.Stats().Position != 4 {
		t.Fatal("formatted writer output", w.String(), w.flushes, s.Stats())
	}
	if _, err = s.String(); !errors.Is(err, ErrNumericStreamFile) {
		t.Fatal("external output exposed as buffer", err)
	}
	w.refuse = true
	if err = s.AppendString("refused"); err != nil {
		t.Fatal(err)
	}
	bad, err := s.Bad()
	if err != nil || !bad {
		t.Fatal("writer failure was not sticky", bad, err)
	}
	w.refuse = false
	if err = s.AppendString("lost"); err != nil {
		t.Fatal(err)
	}
	if w.String() != "  12" {
		t.Fatal("failed stream resumed writing", w.String())
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if budget.Used() != 0 {
		t.Fatal("formatter ownership", budget.Used())
	}
	if _, err = w.Write([]byte("!")); err != nil {
		t.Fatal("borrowed writer closed", err)
	}
}

func TestNumericStreamBorrowedWriterFlushFailure(t *testing.T) {
	budget := arena.NewBudget(1 << 20)
	w := &numericWriterProbe{refuseFlush: true}
	s, err := NewNumericTextStream(NumericTextStreamConfig{Budget: budget, BorrowedWriter: w})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AppendString("before"); err != nil {
		t.Fatal(err)
	}
	if err = s.Flush(); err != nil {
		t.Fatal(err)
	}
	bad, err := s.Bad()
	if err != nil || !bad {
		t.Fatal("flush failure was not recorded", bad, err)
	}
	if err = s.AppendString("after"); err != nil || w.String() != "before" {
		t.Fatal("failed flush did not stop output", w.String(), err)
	}
	if err = s.Close(); err != nil || budget.Used() != 0 {
		t.Fatal("formatter cleanup", err, budget.Used())
	}
}
