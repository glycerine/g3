package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestBVBoundsSourceRecognizesUnsignedSignedAndEquality(t *testing.T) {
	f := newBVBoundsFixture(t)
	for _, tc := range []struct {
		term func() ast.ASTHandle
		lo   int64
	}{{func() ast.ASTHandle { return f.ule(t, f.num(t, 4), f.x) }, 4}, {func() ast.ASTHandle { return f.ule(t, f.x, f.num(t, 24)) }, 0}, {func() ast.ASTHandle { return f.sle(t, f.num(t, 128), f.x) }, 0}} {
		key, b, ok, e := f.simp.recognize(tc.term())
		sharingMust(t, e)
		if !ok || key != f.x || b.empty() {
			t.Fatal(ok, key, b)
		}
		if tc.lo == 4 && b.ranges[0].lo != uint64(tc.lo) {
			t.Fatal(b)
		}
	}
}

// AUDIT.md tactic_bv_01-F4: bv_bounds_simplifier::is_bound rejects
// (= numeral numeral) and still accepts (= numeral x) and (= x numeral).
func TestBVBoundsRecognizeRejectsNumeralEquality(t *testing.T) {
	f := newBVBoundsFixture(t)
	eq := func(a, b ast.ASTHandle) ast.ASTHandle {
		h, e := f.m.MkEq(a, b)
		sharingMust(t, e)
		return f.pin(t, h)
	}
	_, _, ok, e := f.simp.recognize(eq(f.num(t, 1), f.num(t, 2)))
	sharingMust(t, e)
	if ok {
		t.Fatal("(= #x01 #x02) recognized as a bound")
	}
	for _, h := range []ast.ASTHandle{eq(f.num(t, 1), f.x), eq(f.x, f.num(t, 1))} {
		key, b, ok, e := f.simp.recognize(h)
		sharingMust(t, e)
		if !ok || key != f.x || len(b.ranges) != 1 || b.ranges[0].lo != 1 || b.ranges[0].hi != 1 {
			t.Fatal(ok, key, b)
		}
	}
}
