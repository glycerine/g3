package api

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestAPIFixedpointSMT2LifecycleAndWriterGuards(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	if err := f.fp.WriteSMT2(nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("nil fixedpoint display writer", err)
	}
	var output strings.Builder
	astTestMust(t, f.fp.WriteSMT2(&output, f.pAtom))
	if !strings.Contains(output.String(), "(query api-object-fp-p)") {
		t.Fatal("fixedpoint writer omitted query", output.String())
	}
	astTestMust(t, f.fp.Close())
	if _, err := f.fp.SMT2(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed fixedpoint display", err)
	}
}
