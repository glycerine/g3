package realclosure

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type realClosurePrefixWriter struct {
	limit int
	buf   bytes.Buffer
}

func (w *realClosurePrefixWriter) Write(p []byte) (int, error) {
	if w.limit <= 0 {
		return 0, io.ErrClosedPipe
	}
	if len(p) > w.limit {
		p = p[:w.limit]
	}
	n, _ := w.buf.Write(p)
	w.limit -= n
	if w.limit == 0 {
		return n, io.ErrClosedPipe
	}
	return n, nil
}

func TestRetainedValueDisplayRationalAlgebraicAndIntervals(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateSquareFreeRationalRoots([]*realValue{minusTwo, nil, one}, true)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := f.store.displayValue(&out, nil, false, false); err != nil || out.String() != "0" {
		t.Fatal(out.String(), err)
	}
	out.Reset()
	if err := f.store.displayValue(&out, minusTwo, false, false); err != nil || out.String() != "-2" {
		t.Fatal(out.String(), err)
	}
	out.Reset()
	if err := f.store.displayValue(&out, roots[1], true, false); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.HasPrefix(text, "[r!1; r!1 := root(x^2 + -2, ") || !strings.HasSuffix(text, ", {})]") {
		t.Fatal("compact algebraic display", text)
	}
	out.Reset()
	if err := f.store.displayValue(&out, roots[1], false, true); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "x<sup>2</sup>") || !strings.HasPrefix(text, "root(") {
		t.Fatal("pretty algebraic display", text)
	}
	out.Reset()
	if err := f.store.displayValueInterval(&out, nil, f.bounds); err != nil || out.String() != "[0, 0]" {
		t.Fatal(out.String(), err)
	}
	out.Reset()
	if err := f.store.displayValueInterval(&out, roots[0], f.bounds); err != nil || out.Len() == 0 {
		t.Fatal(out.String(), err)
	}
	releaseIsolatedValues(t, f.store, roots)
}

func TestRetainedValueDisplayWriterPrefix(t *testing.T) {
	f := newBoundedSignFixture(t, -2, 2)
	minusTwo, one := f.rational(-2), f.rational(1)
	owner := &algebraicRootOwner{values: f.store, intervals: f.bounds}
	roots, err := owner.isolateSquareFreeRationalRoots([]*realValue{minusTwo, nil, one}, false)
	if err != nil {
		t.Fatal(err)
	}
	writer := &realClosurePrefixWriter{limit: 9}
	err = f.store.displayValue(writer, roots[0], true, false)
	if !errors.Is(err, io.ErrClosedPipe) || writer.buf.String() != "[r!0; r!0" {
		t.Fatal("writer prefix", writer.buf.String(), err)
	}
	releaseIsolatedValues(t, f.store, roots)
}
