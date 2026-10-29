package realclosure

import (
	"bytes"
	"errors"
	"testing"
)

var errDecimalWriterStop = errors.New("decimal writer stop")

type decimalPrefixWriter struct {
	buffer    bytes.Buffer
	remaining int
}

func (w *decimalPrefixWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errDecimalWriterStop
	}
	if len(p) > w.remaining {
		p = p[:w.remaining]
	}
	n, _ := w.buffer.Write(p)
	w.remaining -= n
	if w.remaining == 0 {
		return n, errDecimalWriterStop
	}
	return n, nil
}

func (w *decimalPrefixWriter) String() string { return w.buffer.String() }

func decimalText(t *testing.T, f *rankedValueFixture, value *realValue, precision uint32) string {
	t.Helper()
	var out bytes.Buffer
	if err := f.base.store.displayValueDecimal(&out, value, f.alpha.root.owner.intervals, precision); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
