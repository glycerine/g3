package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp profile selectors and mk_pb.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestPB2BVSourceGCDReductionRoundingModes(t *testing.T) {
	coefficients := []*big.Rat{big.NewRat(4, 1), big.NewRat(6, 1)}
	for _, test := range []struct {
		mode  pb2BVComparison
		bound *big.Rat
		want  string
	}{{pb2BVLE, big.NewRat(7, 1), "3"}, {pb2BVGE, big.NewRat(7, 1), "4"}, {pb2BVEQ, big.NewRat(8, 1), "4"}} {
		got, bound := pb2BVNormalizeGCD(test.mode, coefficients, test.bound)
		if got[0].RatString() != "2" || got[1].RatString() != "3" || bound.RatString() != test.want {
			t.Fatal(test.mode, got, bound, test.want)
		}
	}
	if coefficients[0].RatString() != "4" || coefficients[1].RatString() != "6" {
		t.Fatal("source lexical coefficient copy mutated caller")
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

// The C++ rewriter does not reject unrecognized pb.solver values.
func TestPB2BVSegmentedProfileFallsBackToBitVectors(t *testing.T) {
	m, r := pb2BVFixture(t)
	pb2BVSetProfile(t, r, "pb.solver", "segmented")
	h := pb2BVWeightedFixture(t, m)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, r.Rewrite(h, out, ast.ASTRef{}))
	got, err := m.RefGet(out)
	astTestMust(t, err)
	if got == h {
		t.Fatal("weighted constraint was not translated")
	}
}
