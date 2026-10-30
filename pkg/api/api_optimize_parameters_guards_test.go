package api

import (
	"errors"
	"testing"
)

func TestAPIOptimizeSetParametersOwnershipGuards(t *testing.T) {
	left := newAPIOptimizeFixture(t)
	right := newSMTAPIFixture(t, "ALL", nil)
	foreign, err := right.c.NewParameterSet()
	astTestMust(t, err)
	defer func() { astTestMust(t, foreign.Close()) }()
	if err = left.opt.SetParameters(foreign); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign parameters", err)
	}
	closed, err := left.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if err = left.opt.SetParameters(closed); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed parameters", err)
	}
	astTestMust(t, left.opt.Close())
	if _, err = left.opt.ParameterDescriptions(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed optimize", err)
	}
}
