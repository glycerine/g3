package api

import "testing"

func TestAPIFPARelationDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationFPAEqual, APIDeclarationFPALessThan,
		APIDeclarationFPAGreaterThan, APIDeclarationFPALessEqual,
		APIDeclarationFPAGreaterEqual, APIDeclarationFPAIsNaN,
		APIDeclarationFPAIsInfinite, APIDeclarationFPAIsZero,
		APIDeclarationFPAIsNormal, APIDeclarationFPAIsSubnormal,
		APIDeclarationFPAIsNegative, APIDeclarationFPAIsPositive,
	}
	for offset, want := range wants {
		operation := int32(offset + 23)
		if got := apiFPARelationDeclarationKind(operation); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiFPARelationDeclarationKind(35); got != APIDeclarationInternal {
		t.Fatal("non-relation operation", got)
	}
}
