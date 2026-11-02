package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPB2BVTotalizerCoefficientSumOverflow(t *testing.T) {
	m, r := pb2BVFixture(t)
	pb2BVSetProfile(t, r, "pb.solver", "totalizer")
	a := goalTestAtom(t, m, "totalizer-overflow-a")
	b := goalTestAtom(t, m, "totalizer-overflow-b")
	values := pbTestRationals(t, m, "4294967295", "1", "1")
	h, err := m.MkPseudoBooleanLE(values[:2], []ast.ASTHandle{a, b}, &values[2])
	astTestMust(t, err)
	astTestPin(t, m, h)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	want, err := m.MkNot(a)
	astTestMust(t, err)
	if got != want {
		t.Fatal("overflow changed the constraint requiring the first literal false", got, want)
	}
}

func TestPB2BVSortingCoefficientSumOverflow(t *testing.T) {
	m, r := pb2BVFixture(t)
	pb2BVSetProfile(t, r, "pb.solver", "sorting")
	a := goalTestAtom(t, m, "sorting-overflow-a")
	b := goalTestAtom(t, m, "sorting-overflow-b")
	values := pbTestRationals(t, m, "4294967295", "2", "1")
	h, err := m.MkPseudoBooleanLE(values[:2], []ast.ASTHandle{a, b}, &values[2])
	astTestMust(t, err)
	astTestPin(t, m, h)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	for bits := 0; bits < 4; bits++ {
		env := map[ast.ASTHandle]bool{a: bits&1 != 0, b: bits&2 != 0}
		if rewriterOracleEval(t, m, got, env) != (bits == 0) {
			t.Fatal("weighted sorting overflow changed constraint", bits)
		}
	}
}
