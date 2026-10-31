package api

// Source: src/test/expr_substitution.cpp registered expression rewrite.
import "testing"

func TestAPIExprSubstitutionCPPRewrite(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_BV", nil)
	sort := f.pin(f.c.BitVectorSort(8))
	x := f.pin(f.c.Constant("x", sort))
	eight := f.pin(f.c.BitVectorValue(8, sort))
	four := f.pin(f.c.BitVectorValue(4, sort))
	xorEight := f.pin(f.c.BitVectorXor(x, eight))
	xorSelf := f.pin(f.c.BitVectorXor(x, x))
	_ = f.pin(f.c.BitVectorAnd(xorEight, xorSelf))
	input := f.pin(f.c.BitVectorSub(x, four))
	substitution, err := f.m.NewExprSubstitution()
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelExprSubstitution(&substitution)) }()
	rewritten, err := f.c.Simplify(input)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&rewritten)) }()
	actual, err := f.m.RefGet(rewritten)
	astTestMust(t, err)
	minusFour := f.pin(f.c.BitVectorValue(252, sort))
	expectedInput := f.pin(f.c.BitVectorAdd(minusFour, x))
	expected, err := f.c.Simplify(expectedInput)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&expected)) }()
	want, err := f.m.RefGet(expected)
	astTestMust(t, err)
	if actual != want {
		t.Fatal("subtraction rewrite differs from addition of -4", actual, want)
	}
}
