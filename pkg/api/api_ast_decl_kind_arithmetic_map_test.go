package api

import "testing"

func TestAPIArithmeticDeclarationKindSourceTable(t *testing.T) {
	wants := []APIDeclarationKind{
		APIDeclarationArithmeticNumeral, APIDeclarationAlgebraicNumeral,
		APIDeclarationLE, APIDeclarationGE, APIDeclarationLT, APIDeclarationGT,
		APIDeclarationAdd, APIDeclarationSubtract, APIDeclarationUnaryMinus,
		APIDeclarationMultiply, APIDeclarationDivide, APIDeclarationIntegerDivide,
		APIDeclarationRemainder, APIDeclarationModulo,
		APIDeclarationToReal, APIDeclarationToInteger, APIDeclarationIsInteger,
	}
	for operation, want := range wants {
		if got := apiArithmeticDeclarationKind(int32(operation)); got != want {
			t.Fatal(operation, got, want)
		}
	}
	if got := apiArithmeticDeclarationKind(18); got != APIDeclarationPower {
		t.Fatal("power", got)
	}
	for _, operation := range []int32{17, 19, -1, 1000} {
		if got := apiArithmeticDeclarationKind(operation); got != APIDeclarationInternal {
			t.Fatal("source-unmapped arithmetic operation", operation, got)
		}
	}
}
