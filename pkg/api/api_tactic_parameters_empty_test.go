package api

import "testing"

func TestAPITacticParameterDescriptionsEmptyPrimitive(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	skip, err := f.c.SkipTactic()
	astTestMust(t, err)
	descriptions, err := skip.ParameterDescriptions()
	astTestMust(t, err)
	if size, sizeErr := descriptions.Size(); sizeErr != nil || size != 0 {
		t.Fatal("skip parameters", size, sizeErr)
	}
	astTestMust(t, descriptions.Close())
	astTestMust(t, skip.Close())
}
