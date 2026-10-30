package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIBitVectorDeclarationKindLiveConversions(t *testing.T) {
	f := newAPITypedASTFixture(t)
	integer, err := f.manager.ArithmeticIntegerNumeralText("7")
	if err != nil {
		t.Fatal(err)
	}
	toBitVector, err := f.manager.MkIntToBitVector(8, integer)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.BitVectorDeclarationKind(toBitVector); err != nil || got != APIDeclarationIntegerToBitVector {
		t.Fatal("int2bv app", got, err)
	}
	declaration, err := f.manager.GetDecl(toBitVector)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := f.context.BitVectorDeclarationKind(declaration); err != nil || got != APIDeclarationIntegerToBitVector {
		t.Fatal("int2bv declaration", got, err)
	}
	if _, err := f.context.BitVectorDeclarationKind(integer); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("arithmetic family accepted", err)
	}
}
