package subpaving

// Source: src/math/subpaving/subpaving_mpf.cpp:21 and
// src/math/subpaving/subpaving_mpff.cpp:21.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// The C++ files in this source batch only force concrete template
// instantiation. These interfaces make the equivalent concrete Go surface a
// compile-time obligation without adding a generic or any-valued adapter.
type arbitraryContextInstantiationSurface interface {
	NewVariable(bool) (uint32, error)
	Root() (*ArbitraryNode, error)
	InitializeRoot() error
	Search() error
	Close() error
}

type fastContextInstantiationSurface interface {
	NewVariable(bool) (uint32, error)
	Root() (*FastNode, error)
	InitializeRoot() error
	Search() error
	Close() error
}

var _ arbitraryContextInstantiationSurface = (*ArbitraryContext)(nil)
var _ fastContextInstantiationSurface = (*FastContext)(nil)

func TestConcreteSubpavingInstantiationsRejectMissingManagers(t *testing.T) {
	if got, err := NewArbitraryContext(nil, nil, 0, 0, 0); got != nil || !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("NewArbitraryContext(nil) = (%v, %v), want (nil, invalid slot)", got, err)
	}
	if got, err := NewFastContext(nil, nil, nil, 0, 0, 0); got != nil || !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("NewFastContext(nil) = (%v, %v), want (nil, invalid slot)", got, err)
	}
}
