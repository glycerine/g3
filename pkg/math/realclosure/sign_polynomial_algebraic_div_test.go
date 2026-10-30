package realclosure

import "testing"

func TestSignedRemainderWithAlgebraicDivisorCoefficient(t *testing.T) {
	f := newRealFunctionFixture(t)
	remainder, err := f.store.signedRemainderNormalized(
		[]*realValue{f.one, nil, f.one},
		[]*realValue{f.alpha, f.one},
	)
	mustRealFunction(t, err)
	t.Cleanup(func() { mustRealFunction(t, f.store.releaseTemporaryCoefficients(remainder)) })
	if len(remainder) != 1 || remainder[0] == nil || !remainder[0].isRational || f.coefficientText(remainder[0]) != "-3" {
		t.Fatal("signed remainder of x^2+1 by x+alpha")
	}
}
