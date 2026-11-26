package shell

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type dimacsModelWriter struct {
	bytes.Buffer
	writes int
}

func (w *dimacsModelWriter) Write(p []byte) (int, error) {
	w.writes++
	return w.Buffer.Write(p)
}

func TestDIMACSModelBatchedOutput(t *testing.T) {
	model := make(sat.Model, 4096)
	var expected bytes.Buffer
	model[0] = lbool.True // The unused DIMACS variable is never emitted.
	for i := 1; i < len(model); i++ {
		switch i % 3 {
		case 0:
			model[i] = lbool.Undef
		case 1:
			model[i] = lbool.False
			fmt.Fprintf(&expected, "-%d ", i)
		case 2:
			model[i] = lbool.True
			fmt.Fprintf(&expected, "%d ", i)
		}
	}
	expected.WriteByte('\n')

	var out dimacsModelWriter
	if err := writeDIMACSModel(&out, model); err != nil {
		t.Fatal(err)
	}
	if out.writes < 2 || out.writes > 4 || !bytes.Equal(out.Bytes(), expected.Bytes()) {
		t.Fatalf("model output differed or did not use bounded batches: %d writes, %d bytes, %d expected bytes", out.writes, out.Len(), expected.Len())
	}
	short := &shellShortWriter{remaining: 5000}
	if err := writeDIMACSModel(short, model); err != io.ErrShortWrite || short.remaining != 0 {
		t.Fatalf("short write: %v, remaining %d bytes", err, short.remaining)
	}
}
