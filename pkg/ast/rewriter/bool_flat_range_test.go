package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// Compare the flat reducer with its general non-flattening pass on every
// triple from this alphabet. None of the operands is itself an AND or OR.
func TestBoolFlatSignedOperandRanges(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "flat-range-p")
	q := goalTestAtom(t, m, "flat-range-q")
	r := goalTestAtom(t, m, "flat-range-r")
	notP := goalTestApp(t, m, ast.ASTOpNot, p)
	notQ := goalTestApp(t, m, ast.ASTOpNot, q)
	truth, err := m.MkTrue()
	astTestMust(t, err)
	falsity, err := m.MkFalse()
	astTestMust(t, err)
	alphabet := []ast.ASTHandle{p, q, r, notP, notQ, truth, falsity}
	reference := boolRewriter{manager: m}
	flat := boolRewriter{manager: m, flat: true}
	want := astTestPin(t, m, truth)
	got := astTestPin(t, m, truth)
	for _, or := range []bool{true, false} {
		for _, a := range alphabet {
			for _, b := range alphabet {
				for _, c := range alphabet {
					astTestMust(t, m.SetRef(want, truth))
					astTestMust(t, m.SetRef(got, truth))
					args, err := ast.SliceArguments([]ast.ASTHandle{falsity, a, b, c, truth}).Range(1, 3)
					astTestMust(t, err)
					ws, err := reference.mkNFlatCore(args, want, or)
					astTestMust(t, err)
					gs, err := flat.mkFlatCore(args, got, or)
					astTestMust(t, err)
					wh, err := m.RefGet(want)
					astTestMust(t, err)
					gh, err := m.RefGet(got)
					astTestMust(t, err)
					if gs != ws || gh != wh {
						t.Fatalf("or=%t operands=%v: status %v, result %v, want %v, %v", or, []ast.ASTHandle{a, b, c}, gs, gh, ws, wh)
					}
				}
			}
		}
	}
}
