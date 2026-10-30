package api

import "testing"

func TestAPIDatalogFilterDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationRelationFilter,
		APIDeclarationRelationNegationFilter,
		APIDeclarationRelationRename,
		APIDeclarationRelationComplement,
		APIDeclarationRelationSelect,
		APIDeclarationRelationClone,
	}
	for offset, want := range wants {
		operation := int32(offset + 7)
		if got := apiDatalogFilterDeclarationKind(operation); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiDatalogFilterDeclarationKind(13); got != APIDeclarationInternal {
		t.Fatal("non-filter operation", got)
	}
}
