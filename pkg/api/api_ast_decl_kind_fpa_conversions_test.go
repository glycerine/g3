package api

import "testing"

func TestAPIFPAConversionDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationFPAConstruct,
		APIDeclarationFPAToFloatingPoint,
		APIDeclarationFPAToFloatingPointUnsigned,
		APIDeclarationFPAToUnsignedBitVector,
		APIDeclarationFPAToSignedBitVector,
		APIDeclarationFPAToReal,
		APIDeclarationFPAToIEEEBitVector,
	}
	for offset, want := range wants {
		operation := int32(offset + 35)
		if got := apiFPAConversionDeclarationKind(operation); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiFPAConversionDeclarationKind(42); got != APIDeclarationInternal {
		t.Fatal("non-conversion operation", got)
	}
}
