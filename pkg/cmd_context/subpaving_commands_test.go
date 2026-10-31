package cmd_context_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
)

type subpavingRefusingWriter struct{ remaining int }

func (w *subpavingRefusingWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(p)
	return len(p), nil
}

func TestWriteSubpavingWriterFailureAndRetry(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Error(err)
		}
	}()
	m, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	one, err := m.ArithmeticIntegerNumeralText("1")
	if err != nil {
		t.Fatal(err)
	}
	sort, err := m.GetSort(one)
	if err != nil {
		t.Fatal(err)
	}
	name, err := m.Symbols.Intern("writer-x")
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	xRef, err := m.NewRef(x)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.DelRef(&xRef); err != nil {
			t.Error(err)
		}
	}()
	for _, allowance := range []int{0, 19, 25, 34} {
		if err = context.WriteSubpaving(&subpavingRefusingWriter{remaining: allowance}, x); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal("writer error", allowance, err)
		}
		var output bytes.Buffer
		if err = context.WriteSubpaving(&output, x); err != nil {
			t.Fatal(err)
		}
		if got, want := output.String(), "writer-x\n=======>\nx0 := writer-x\n1/1 x0\n"; got != want {
			t.Fatalf("retry = %q, want %q", got, want)
		}
	}
}
