package api

import "testing"

func TestAPIDatalogRelationDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationRelationStore,
		APIDeclarationRelationEmpty,
		APIDeclarationRelationIsEmpty,
		APIDeclarationRelationJoin,
		APIDeclarationRelationUnion,
		APIDeclarationRelationWiden,
		APIDeclarationRelationProject,
	}
	for operation, want := range wants {
		if got := apiDatalogRelationDeclarationKind(int32(operation)); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiDatalogRelationDeclarationKind(7); got != APIDeclarationInternal {
		t.Fatal("non-relation operation", got)
	}
}
