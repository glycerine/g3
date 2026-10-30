package api

import "testing"

func TestAPIFPAArithmeticDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationFPAAdd, APIDeclarationFPASubtract, APIDeclarationFPANegate,
		APIDeclarationFPAMultiply, APIDeclarationFPADivide, APIDeclarationFPARemainder,
		APIDeclarationFPAAbsolute, APIDeclarationFPAMinimum, APIDeclarationFPAMaximum,
		APIDeclarationFPAFusedMultiplyAdd, APIDeclarationFPASquareRoot,
		APIDeclarationFPARoundToIntegral,
	}
	for offset, want := range wants {
		operation := int32(offset + 11)
		if got := apiFPAArithmeticDeclarationKind(operation); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiFPAArithmeticDeclarationKind(23); got != APIDeclarationInternal {
		t.Fatal("non-arithmetic operation", got)
	}
}
