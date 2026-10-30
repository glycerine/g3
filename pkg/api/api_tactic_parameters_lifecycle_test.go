package api

import (
	"errors"
	"testing"
)

func TestAPITacticParameterDescriptionsOwnIndependentCopy(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	simplify, err := f.c.Tactic("simplify")
	astTestMust(t, err)
	descriptions, err := simplify.ParameterDescriptions()
	astTestMust(t, err)
	astTestMust(t, simplify.Close())
	if size, sizeErr := descriptions.Size(); sizeErr != nil || size == 0 {
		t.Fatal("description after tactic close", size, sizeErr)
	}
	astTestMust(t, descriptions.Close())
	astTestMust(t, descriptions.Close())
	if _, err = descriptions.Size(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed descriptions", err)
	}
}
