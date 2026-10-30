package api

import (
	"errors"
	"testing"
)

func TestAPITacticParameterMetadataOwnershipGuards(t *testing.T) {
	left := newSMTAPIFixture(t, "ALL", nil)
	right := newSMTAPIFixture(t, "ALL", nil)
	simplify, err := left.c.Tactic("simplify")
	astTestMust(t, err)
	parameters, err := right.c.NewParameterSet()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	if _, err = simplify.UsingParameters(parameters); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign parameters", err)
	}
	astTestMust(t, simplify.Close())
	if _, err = simplify.ParameterDescriptions(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed tactic descriptions", err)
	}
	if _, err = simplify.Help(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed tactic help", err)
	}
}
