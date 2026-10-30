package interval

import (
	"bytes"
	"errors"
	"testing"
)

func TestModularIntervalValidationDispatchAndDisplay(t *testing.T) {
	if _, err := NewModularInterval(modularTestValue(t, "5"), modularTestValue(t, "4"), 8, true); !errors.Is(err, ErrExtendedDomain) {
		t.Fatal(err)
	}
	if _, err := NewModularInterval(modularTestValue(t, "0"), modularTestValue(t, "256"), 8, true); !errors.Is(err, ErrExtendedDomain) {
		t.Fatal(err)
	}
	small := modularTestInterval(t, "1", "3", 8, true)
	large := modularTestInterval(t, "1", "3", 80, true)
	if _, err := small.Equal(large); !errors.Is(err, ErrExtendedDomain) {
		t.Fatal(err)
	}
	if _, err := small.Intersect(large, &ModularInterval{}); !errors.Is(err, ErrExtendedDomain) {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := small.Display(&out); err != nil || out.String() != "[1, 3]" {
		t.Fatal(out.String(), err)
	}
	writeErr := errors.New("stop")
	w := &failAfterWrites{remaining: 2, err: writeErr}
	if err := small.Display(w); !errors.Is(err, writeErr) || w.String() != "[1" {
		t.Fatal(w.String(), err)
	}
}
