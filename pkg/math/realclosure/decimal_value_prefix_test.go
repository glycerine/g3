package realclosure

import (
	"errors"
	"strings"
	"testing"
)

func TestRetainedDecimalValueWriterPrefix(t *testing.T) {
	f := newRankedValueFixture(t, true)
	full := decimalText(t, f, f.alpha, 12)
	for stop := 1; stop < len(full); stop++ {
		writer := &decimalPrefixWriter{remaining: stop}
		err := f.base.store.displayValueDecimal(writer, f.alpha, f.alpha.root.owner.intervals, 12)
		if !errors.Is(err, errDecimalWriterStop) || writer.String() != full[:stop] {
			t.Fatal(stop, writer.String(), full[:stop], err)
		}
	}
	if strings.Contains(full, " ") {
		t.Fatal(full)
	}
}
