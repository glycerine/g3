package api

import "testing"

func TestAPIBitVectorRelationDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationBitVectorUnsignedLE, APIDeclarationBitVectorSignedLE,
		APIDeclarationBitVectorUnsignedGE, APIDeclarationBitVectorSignedGE,
		APIDeclarationBitVectorUnsignedLT, APIDeclarationBitVectorSignedLT,
		APIDeclarationBitVectorUnsignedGT, APIDeclarationBitVectorSignedGT,
		APIDeclarationBitVectorAnd, APIDeclarationBitVectorOr, APIDeclarationBitVectorNot,
		APIDeclarationBitVectorXor, APIDeclarationBitVectorNand, APIDeclarationBitVectorNor,
		APIDeclarationBitVectorXnor,
	}
	for offset, want := range wants {
		operation := int32(offset + 22)
		if got := apiBitVectorRelationDeclarationKind(operation); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiBitVectorRelationDeclarationKind(37); got != APIDeclarationInternal {
		t.Fatal("non-relation operation", got)
	}
}
