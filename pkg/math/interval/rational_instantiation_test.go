package interval

// Source: src/math/interval/interval_mpq.cpp:17.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// rationalIntervalInstantiationSurface is the concrete portion of
// interval_manager<im_default_config> exercised by retained consumers. Go has
// no separate template-instantiation translation unit, so compilation of this
// assignment is the corresponding build-time check.
type rationalIntervalInstantiationSurface interface {
	SetFinite(*numeral.Rational, *numeral.Rational, bool, bool) error
	Reset() error
	Neg(*RationalInterval) error
	Add(*RationalInterval, *RationalInterval) error
	Mul(*RationalInterval, *RationalInterval) error
	Power(*RationalInterval, uint32) error
	Inv(*RationalInterval) error
	Div(*RationalInterval, *RationalInterval) error
	Close() error
}

var _ rationalIntervalInstantiationSurface = (*RationalInterval)(nil)

func TestRationalIntervalConcreteInstantiationRejectsMissingManager(t *testing.T) {
	got, err := NewRationalInterval(nil)
	if got != nil || !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("NewRationalInterval(nil) = (%v, %v), want (nil, invalid slot)", got, err)
	}
}
