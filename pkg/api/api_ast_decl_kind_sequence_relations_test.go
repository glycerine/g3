package api

import "testing"

func TestAPISequenceRelationDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationSequencePrefix,
		APIDeclarationSequenceSuffix,
		APIDeclarationSequenceContains,
	}
	for offset, want := range wants {
		operation := int32(offset + 3)
		if got := apiSequenceRelationDeclarationKind(operation); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiSequenceRelationDeclarationKind(6); got != APIDeclarationInternal {
		t.Fatal("non-relation operation", got)
	}
}
