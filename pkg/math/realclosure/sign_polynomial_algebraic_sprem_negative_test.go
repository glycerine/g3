package realclosure

import "testing"

func TestAlgebraicSignedPseudoRemainderKeepsOddNegativeLeadingSign(t *testing.T) {
	f := newRealFunctionFixture(t)
	minusOne := f.rational(-1)
	remainder, err := f.store.signedPseudoRemainderNormalized(
		[]*realValue{f.one, nil, f.one},
		[]*realValue{f.alpha, nil, minusOne},
	)
	mustRealFunction(t, err)
	t.Cleanup(func() { mustRealFunction(t, f.store.releaseTemporaryCoefficients(remainder)) })
	sign, err := f.store.signValue(remainder[0])
	mustRealFunction(t, err)
	if len(remainder) != 1 || sign != -1 {
		t.Fatal("odd pseudo-remainder step flipped a negative leading sign")
	}
}
