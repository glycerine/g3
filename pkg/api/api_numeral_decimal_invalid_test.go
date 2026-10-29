package api

import (
	"errors"
	"testing"
)

func TestAPINumeralDecimalStringRejectsNonNumeral(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	truth := f.pin(f.c.Bool(true))
	if text, err := f.c.NumeralDecimalString(truth, 4); text != "" || !errors.Is(err, errAPINumeralArgument) {
		t.Fatal("Boolean decimal", text, err)
	}
}
