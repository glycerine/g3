package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestAPIPolynomialSubresultantsBoundsAndClose(t *testing.T) {
	f := newAPIPolynomialFixture(t)
	result, err := f.c.PolynomialSubresultants(f.p, f.q, f.x)
	astTestMust(t, err)
	size, err := result.Len()
	astTestMust(t, err)
	if _, err = result.Get(size); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("result bounds", err)
	}
	astTestMust(t, result.Close())
	if _, err = result.Len(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed result", err)
	}
	astTestMust(t, result.Close())
}
