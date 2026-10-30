package api

import "testing"

func TestAPISequenceIndexedDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationSequenceExtract,
		APIDeclarationSequenceReplace,
		APIDeclarationSequenceAt,
		APIDeclarationSequenceLength,
		APIDeclarationSequenceIndex,
	}
	for offset, want := range wants {
		operation := int32(offset + 6)
		if got := apiSequenceIndexedDeclarationKind(operation); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiSequenceIndexedDeclarationKind(11); got != APIDeclarationInternal {
		t.Fatal("non-indexed operation", got)
	}
}
