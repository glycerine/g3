package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp profile selectors and mk_pb.

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

func pb2BVSetProfile(t *testing.T, r *PB2BVRewriter, name, spelling string) {
	t.Helper()
	p, e := r.paramsContext.NewValue()
	astTestMust(t, e)
	s := astTestSymbol(t, r.manager, spelling)
	astTestMust(t, r.paramsContext.SetSymbolName(&p, &name, s))
	astTestMust(t, r.UpdateParams(&p))
	astTestMust(t, r.paramsContext.Del(&p))
}

func pb2BVWeightedFixture(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
	t.Helper()
	a := goalTestAtom(t, m, "profile-weight-a")
	b := goalTestAtom(t, m, "profile-weight-b")
	values := pbTestRationals(t, m, "2", "3", "4")
	h, e := m.MkPseudoBooleanLE(values[:2], []ast.ASTHandle{a, b}, &values[2])
	astTestMust(t, e)
	return h
}

func TestPB2BVProfileSolverRetainsSmallWeightedConstraint(t *testing.T) {
	m, r := pb2BVFixture(t)
	pb2BVSetProfile(t, r, "pb.solver", "solver")
	h := pb2BVWeightedFixture(t, m)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != h {
		t.Fatal("default solver failed to retain small weighted PB")
	}
	if n, e := r.NumTranslated(); e != nil || n != 0 {
		t.Fatal("retained constraint counted", n, e)
	}
}

func TestPB2BVSourceBinaryMergeAndTotalizerProfiles(t *testing.T) {
	for _, profile := range []string{"binary_merge", "totalizer"} {
		m, r := pb2BVFixture(t)
		pb2BVSetProfile(t, r, "pb.solver", profile)
		h := pb2BVWeightedFixture(t, m)
		out := astTestPin(t, m, ast.ASTHandle{})
		astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if got == h {
			t.Fatal(profile, "did not compile weighted PB")
		}
		stats, e := r.CollectStatistics()
		astTestMust(t, e)
		if stats.CompiledBV != 1 || stats.CompiledCardinality != 0 {
			t.Fatal(profile, "wrong source compile counters", stats)
		}
	}
}

func TestPB2BVSourceKeepCardinalityMinimumArity(t *testing.T) {
	m, r := pb2BVFixture(t)
	p, e := r.paramsContext.NewValue()
	astTestMust(t, e)
	keep, minimum := "keep_cardinality_constraints", "pb.min_arity"
	astTestMust(t, r.paramsContext.SetBoolName(&p, &keep, true))
	astTestMust(t, r.paramsContext.SetUIntName(&p, &minimum, 3))
	astTestMust(t, r.UpdateParams(&p))
	astTestMust(t, r.paramsContext.Del(&p))
	args := []ast.ASTHandle{goalTestAtom(t, m, "keep-a"), goalTestAtom(t, m, "keep-b"), goalTestAtom(t, m, "keep-c")}
	h, e := m.MkPseudoBooleanAtMost(args, 1)
	astTestMust(t, e)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
	got, e := m.RefGet(out)
	astTestMust(t, e)
	if got != h {
		t.Fatal("minimum arity cardinality retention omitted")
	}
}
