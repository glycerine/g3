package api

import "testing"

func TestAPIBitVectorArithmeticDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationBitVectorNumeral, APIDeclarationBitOne, APIDeclarationBitZero,
		APIDeclarationBitVectorNegate, APIDeclarationBitVectorAdd, APIDeclarationBitVectorSubtract,
		APIDeclarationBitVectorMultiply, APIDeclarationBitVectorSignedDivide,
		APIDeclarationBitVectorUnsignedDivide, APIDeclarationBitVectorSignedRemainder,
		APIDeclarationBitVectorUnsignedRemainder, APIDeclarationBitVectorSignedModulo,
		APIDeclarationBitVectorSignedDivideByZero, APIDeclarationBitVectorUnsignedDivideByZero,
		APIDeclarationBitVectorUnsignedRemainderByZero, APIDeclarationBitVectorUnsignedRemainderByZero,
		APIDeclarationBitVectorSignedModuloByZero, APIDeclarationBitVectorSignedDivideInternal,
		APIDeclarationBitVectorUnsignedDivideInternal, APIDeclarationBitVectorSignedRemainderInternal,
		APIDeclarationBitVectorUnsignedRemainderInternal, APIDeclarationBitVectorSignedModuloInternal,
	}
	for operation, want := range wants {
		if got := apiBitVectorArithmeticDeclarationKind(int32(operation)); got != want {
			t.Fatalf("operation %d: got %d, want %d", operation, got, want)
		}
	}
	if got := apiBitVectorArithmeticDeclarationKind(22); got != APIDeclarationInternal {
		t.Fatal("non-arithmetic operation", got)
	}
}
