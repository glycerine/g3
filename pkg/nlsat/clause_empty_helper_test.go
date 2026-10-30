package nlsat

import (
	"testing"

	"github.com/glycerine/g3/pkg/sat"
)

func TestNormalizeClauseLiteralsPreservesNonemptyBorrow(t *testing.T) {
	literals := []sat.Literal{TrueLiteral}
	got := normalizeClauseLiterals(literals)
	if len(got) != 1 || got[0] != TrueLiteral || &got[0] != &literals[0] {
		t.Fatal("nonempty clause borrow changed", got)
	}
	got = normalizeClauseLiterals(nil)
	if len(got) != 1 || got[0] != FalseLiteral {
		t.Fatal("empty normalization", got)
	}
}
