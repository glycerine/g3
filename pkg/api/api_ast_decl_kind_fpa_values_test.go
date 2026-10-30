package api

import "testing"

func TestAPIFPAValueDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationFPANumeral,
		APIDeclarationFPAPlusInfinity,
		APIDeclarationFPAMinusInfinity,
		APIDeclarationFPANaN,
		APIDeclarationFPAPlusZero,
		APIDeclarationFPAMinusZero,
	}
	for offset, want := range wants {
		operation := int32(offset + 5)
		if got := apiFPAValueDeclarationKind(operation); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiFPAValueDeclarationKind(11); got != APIDeclarationInternal {
		t.Fatal("non-value operation", got)
	}
}
