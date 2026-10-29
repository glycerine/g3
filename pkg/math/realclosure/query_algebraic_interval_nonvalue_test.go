package realclosure

import "testing"

func TestAlgebraicMetadataIntervalRejectsNonAlgebraicWithoutMutation(t *testing.T) {
	f := newRankedValueFixture(t, true)
	for _, value := range []*realValue{nil, f.one} {
		query, algebraic, err := f.base.store.queryAlgebraicInterval(value)
		if err != nil || algebraic || query != (algebraicIntervalQuery{}) {
			t.Fatal(query, algebraic, err)
		}
	}
}
