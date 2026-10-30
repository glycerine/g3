package realclosure

import "testing"

func TestAlgebraicSignedPseudoRemainderNormalizesNestedIntegerContent(t *testing.T) {
	f := newRealFunctionFixture(t)
	remainder, err := f.store.signedPseudoRemainderNormalized(
		[]*realValue{f.one, nil, f.one},
		[]*realValue{f.alpha, f.one},
	)
	mustRealFunction(t, err)
	t.Cleanup(func() { mustRealFunction(t, f.store.releaseTemporaryCoefficients(remainder)) })
	if len(remainder) != 1 || f.coefficientText(remainder[0]) != "-1" {
		t.Fatal("signed pseudo remainder did not normalize integer content")
	}
}
