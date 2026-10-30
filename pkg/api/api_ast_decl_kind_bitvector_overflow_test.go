package api

import "testing"

func TestAPIBitVectorOverflowDeclarationKindSourceTable(t *testing.T) {
	tests := []struct {
		operation int32
		want      APIDeclarationKind
	}{
		{52, APIDeclarationBitVectorUnsignedMultiplyNoOverflow},
		{53, APIDeclarationBitVectorSignedMultiplyNoOverflow},
		{54, APIDeclarationBitVectorSignedMultiplyNoUnderflow},
		{55, APIDeclarationBitToBoolean},
		{57, APIDeclarationIntegerToBitVector},
		{58, APIDeclarationBitVectorToInteger},
		{59, APIDeclarationBitVectorCarry},
		{60, APIDeclarationBitVectorXor3},
	}
	for _, test := range tests {
		if got := apiBitVectorOverflowDeclarationKind(test.operation); got != test.want {
			t.Fatalf("operation %d: got %d, want %d", test.operation, got, test.want)
		}
	}
	// OP_MKBV is intentionally absent from the retained source API switch.
	if got := apiBitVectorDeclarationKind(56); got != APIDeclarationInternal {
		t.Fatal("mkbv must remain internal", got)
	}
	if got := apiBitVectorDeclarationKind(61); got != APIDeclarationInternal {
		t.Fatal("unknown operation", got)
	}
}
