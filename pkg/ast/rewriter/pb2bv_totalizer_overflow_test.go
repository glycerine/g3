package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPB2BVWeightedCoefficientSumOverflow(t *testing.T) {
	for _, profile := range []string{"totalizer", "binary_merge"} {
		t.Run(profile, func(t *testing.T) {
			m, r := pb2BVFixture(t)
			pb2BVSetProfile(t, r, "pb.solver", profile)
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

		})
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

func TestPB2BVSortingMaximumUnsignedBound(t *testing.T) {
	m, context, globals, params := boolRewriterConfigFixture(t)
	astTestMust(t, m.RegisterDeclarationPlugins())
	pbContext, pbParams := enum2BVParameters(t, m)
	r, err := NewPB2BVRewriter(m, pbContext, pbParams)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	theory, err := NewTheoryRewriter(m, context, globals, params)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, theory.Close()) })
	pb2BVSetProfile(t, r, "pb.solver", "sorting")
	sort, err := m.MkBoolSort()
	astTestMust(t, err)
	a, err := m.MkVar(0, sort)
	astTestMust(t, err)
	astTestPin(t, m, a)
	b, err := m.MkVar(1, sort)
	astTestMust(t, err)
	astTestPin(t, m, b)
	values := pbTestRationals(t, m, "4294967294", "1", "4294967295")
	h, err := m.MkPseudoBooleanEQ(values[:2], []ast.ASTHandle{a, b}, &values[2])
	astTestMust(t, err)
	astTestPin(t, m, h)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	subst, err := NewVarSubstitution(m, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, subst.Close()) })
	assigned := astTestPin(t, m, ast.ASTHandle{})
	evaluated := astTestPin(t, m, ast.ASTHandle{})
	for bits := 0; bits < 4; bits++ {
		left, err := m.MkBoolVal(bits&1 != 0)
		astTestMust(t, err)
		right, err := m.MkBoolVal(bits&2 != 0)
		astTestMust(t, err)
		astTestMust(t, subst.Substitute(got, []ast.ASTHandle{left, right}, assigned))
		ground, err := m.RefGet(assigned)
		astTestMust(t, err)
		astTestMust(t, theory.Rewrite(ground, evaluated))
		result, err := m.RefGet(evaluated)
		astTestMust(t, err)
		if !(m.IsTrue(result) || m.IsFalse(result)) || m.IsTrue(result) != (bits == 3) {
			t.Fatal("maximum-bound equality truth table", bits, result)
		}
	}
}
