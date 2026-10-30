package arith

import (
	"reflect"
	"testing"
)

func TestFMResolveLiteralUnionAndTautology(t *testing.T) {
	got, tautology := fmMergeLiterals([]int32{2, -4}, []int32{-4, 7})
	if tautology || !reflect.DeepEqual(got, []int32{2, -4, 7}) {
		t.Fatalf("literal union = %v, tautology %v", got, tautology)
	}
	if _, tautology = fmMergeLiterals([]int32{2}, []int32{-2, 8}); !tautology {
		t.Fatal("complementary literals were not a tautology")
	}
}

func TestFMLiteralVariableHandlesNegativeLimit(t *testing.T) {
	if got := fmLiteralVariable(-2147483648); got != 2147483648 {
		t.Fatalf("literal variable = %d", got)
	}
}
