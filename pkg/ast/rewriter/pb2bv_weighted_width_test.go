package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp mk_le_ge.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

// AUDIT.md ast_rewriter_04-F2: 4.7.1 card2bv_rewriter::mk_le_ge has no gcd
// reduction: 2a + 4b <= 5 compiles with nb = 3 bits for k = 5, so the adder
// is 4 bits wide (with a gcd reduction to a + 2b <= 2 it would be 3).
func TestPB2BVWeightedKeepsCommonFactor(t *testing.T) {
	m, r := pb2BVFixture(t)
	a := goalTestAtom(t, m, "gcd-a")
	b := goalTestAtom(t, m, "gcd-b")
	values := pbTestRationals(t, m, "2", "4", "5")
	h, e := m.MkPseudoBooleanLE(values[:2], []ast.ASTHandle{a, b}, &values[2])
	astTestMust(t, e)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
	got, e := m.RefGet(out)
	astTestMust(t, e)
	widest := uint32(0)
	seen := map[ast.ASTHandle]bool{}
	var walk func(ast.ASTHandle)
	walk = func(x ast.ASTHandle) {
		if seen[x] {
			return
		}
		seen[x] = true
		if kind, e := m.GetKind(x); e != nil || kind != ast.ASTApp {
			return
		}
		if sort, e := m.GetSort(x); e == nil {
			if width, e := m.BitVectorSize(sort); e == nil && width > widest {
				widest = width
			}
		}
		n, e := m.GetNumArgs(x)
		astTestMust(t, e)
		for i := uint32(0); i < n; i++ {
			arg, e := m.GetArg(x, i)
			astTestMust(t, e)
			walk(arg)
		}
	}
	walk(got)
	if widest != 4 {
		t.Fatal("weighted PB adder width", widest)
	}
}
