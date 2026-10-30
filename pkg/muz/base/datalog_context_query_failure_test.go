package base_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/muz/base"
)

func TestDatalogContextAddQueryRejectsNonBooleanAndClosedContext(t *testing.T) {
	f := newDatalogQueryFixture(t)
	zero, err := f.manager.ArithmeticIntegerNumeralText("0")
	ruleMust(t, err)
	if _, err = f.context.AddQuery(zero); err == nil {
		t.Fatal("non-Boolean query accepted")
	}
	ruleMust(t, f.context.Close())
	if _, err = f.context.AddQuery(zero); !errors.Is(err, base.ErrDatalogContextOwner) {
		t.Fatal("closed context query", err)
	}
}
