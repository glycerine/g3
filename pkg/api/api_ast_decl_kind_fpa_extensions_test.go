package api

import "testing"

func TestAPIFPAExtensionDeclarationKindSourceTable(t *testing.T) {
	if got := apiFPAExtensionDeclarationKind(42); got != APIDeclarationFPABitVectorWrap {
		t.Fatal("bvwrap", got)
	}
	if got := apiFPAExtensionDeclarationKind(43); got != APIDeclarationFPABitVectorToRoundingMode {
		t.Fatal("bv2rm", got)
	}
	if got := apiFloatingPointDeclarationKind(44); got != APIDeclarationInternal {
		t.Fatal("unknown FPA operation", got)
	}
}
