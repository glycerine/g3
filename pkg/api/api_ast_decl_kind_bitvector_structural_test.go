package api

import "testing"

func TestAPIBitVectorStructuralDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationBitVectorConcat, APIDeclarationBitVectorSignExtend,
		APIDeclarationBitVectorZeroExtend, APIDeclarationBitVectorExtract,
		APIDeclarationBitVectorRepeat, APIDeclarationBitVectorReduceOr,
		APIDeclarationBitVectorReduceAnd, APIDeclarationBitVectorCompare,
		APIDeclarationBitVectorShiftLeft, APIDeclarationBitVectorLogicalShiftRight,
		APIDeclarationBitVectorArithmeticShiftRight, APIDeclarationBitVectorRotateLeft,
		APIDeclarationBitVectorRotateRight, APIDeclarationBitVectorExtendedRotateLeft,
		APIDeclarationBitVectorExtendedRotateRight,
	}
	for offset, want := range wants {
		operation := int32(offset + 37)
		if got := apiBitVectorStructuralDeclarationKind(operation); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiBitVectorStructuralDeclarationKind(52); got != APIDeclarationInternal {
		t.Fatal("non-structural operation", got)
	}
}
