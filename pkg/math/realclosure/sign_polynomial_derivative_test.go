package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestRealClosureRationalDerivativeKeepsCoefficientOwnership(t *testing.T) {
	budget := arena.NewBudget(1 << 24)
	service, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	store := &realValueStore{numbers: service.Manager}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	one, err := store.newRational(1, 1)
	must(err)
	minusOne, err := store.newRational(-1, 1)
	must(err)
	three, err := store.newRational(3, 1)
	must(err)
	// d(x³-x)/dx = 3x²-1, with the source nil zero coefficient.
	derived, err := store.derivativeRationalPolynomial([]*realValue{nil, minusOne, nil, one})
	must(err)
	if len(derived) != 3 || derived[1] != nil || derived[0] == nil || derived[2] == nil {
		t.Fatalf("unexpected derivative coefficients: %v", derived)
	}
	constant, err := store.numbers.Equal(&derived[0].rational, &minusOne.rational)
	must(err)
	quadratic, err := store.numbers.Equal(&derived[2].rational, &three.rational)
	must(err)
	if !constant || !quadratic || one.refs != 1 || minusOne.refs != 1 {
		t.Fatal("derivative changed its input or produced incorrect coefficients")
	}
	remainder, err := store.signedRemainderRational([]*realValue{nil, minusOne, nil, one}, derived)
	must(err)
	twoThirds, err := store.newRational(2, 3)
	must(err)
	if len(remainder) != 2 || remainder[0] != nil || remainder[1] == nil {
		t.Fatalf("first Sturm signed remainder: %v", remainder)
	}
	equal, err := store.numbers.Equal(&remainder[1].rational, &twoThirds.rational)
	must(err)
	if !equal {
		t.Fatal("first Sturm signed remainder is not 2x/3")
	}
	lastNonzero, err := store.signedRemainderRational(derived, remainder)
	must(err)
	if len(lastNonzero) != 1 || lastNonzero[0] == nil {
		t.Fatalf("second Sturm signed remainder: %v", lastNonzero)
	}
	equal, err = store.numbers.Equal(&lastNonzero[0].rational, &one.rational)
	must(err)
	if !equal {
		t.Fatal("second Sturm signed remainder is not one")
	}
	end, err := store.signedRemainderRational(remainder, lastNonzero)
	must(err)
	if len(end) != 0 {
		t.Fatal("Sturm sequence did not end at the constant divisor")
	}
	pseudo, err := store.signedPseudoRemainderRational([]*realValue{nil, minusOne, nil, one}, derived)
	must(err)
	if len(pseudo) != 2 || pseudo[0] != nil || pseudo[1] == nil {
		t.Fatalf("normalized pseudo-remainder: %v", pseudo)
	}
	equal, err = store.numbers.Equal(&pseudo[1].rational, &one.rational)
	must(err)
	if !equal {
		t.Fatal("default pseudo-remainder did not normalize 2x to x")
	}
	pseudoLast, err := store.signedPseudoRemainderRational(derived, pseudo)
	must(err)
	if len(pseudoLast) != 1 || pseudoLast[0] == nil {
		t.Fatalf("last pseudo-remainder: %v", pseudoLast)
	}
	equal, err = store.numbers.Equal(&pseudoLast[0].rational, &one.rational)
	must(err)
	if !equal {
		t.Fatal("default pseudo-remainder did not reach one")
	}
	negativeLeading, err := store.signedPseudoRemainderRational([]*realValue{one, nil, one}, []*realValue{nil, minusOne})
	must(err)
	if len(negativeLeading) != 1 || negativeLeading[0] == nil {
		t.Fatalf("negative-leading pseudo-remainder: %v", negativeLeading)
	}
	equal, err = store.numbers.Equal(&negativeLeading[0].rational, &minusOne.rational)
	must(err)
	if !equal {
		t.Fatal("odd pseudo-remainder step flipped a negative leading divisor")
	}
	polynomial := []*realValue{nil, minusOne, nil, one}
	query := []*realValue{nil, one}
	querySquared, err := store.multiplyRationalPolynomials(query, query)
	must(err)
	for _, usePseudo := range []bool{true, false} {
		rootCount, err := store.rootCountUnboundedRational(polynomial, usePseudo)
		must(err)
		firstQuery, err := store.tarskiUnboundedRational(polynomial, query, usePseudo)
		must(err)
		secondQuery, err := store.tarskiUnboundedRational(polynomial, querySquared, usePseudo)
		must(err)
		if rootCount != 3 || firstQuery != 0 || secondQuery != 2 {
			t.Fatalf("Sturm-Tarski values with use_prem=%v: roots=%d, q=%d, q²=%d", usePseudo, rootCount, firstQuery, secondQuery)
		}
		state := newTestSignDetermination(rootCount)
		must(state.attachInitialPolynomial(store, one))
		kept, allOne, err := state.advanceFromTarski(query, firstQuery, secondQuery, []int{rootCount, firstQuery, secondQuery})
		must(err)
		if !kept || !allOne || len(state.conditions) != 3 {
			t.Fatalf("Sturm-Tarski values did not separate three roots with use_prem=%v", usePseudo)
		}
		must(state.closePolynomials())
	}
	must(store.releaseCoefficients(querySquared))
	must(store.releaseCoefficients(pseudo))
	must(store.releaseCoefficients(pseudoLast))
	must(store.releaseCoefficients(negativeLeading))
	must(store.releaseCoefficients(remainder))
	must(store.releaseCoefficients(lastNonzero))
	must(store.release(twoThirds))
	must(store.releaseCoefficients(derived))
	zero, err := store.derivativeRationalPolynomial([]*realValue{one})
	must(err)
	if len(zero) != 0 {
		t.Fatal("constant polynomial has nonzero derivative")
	}
	must(store.release(one))
	must(store.release(minusOne))
	must(store.release(three))
	must(service.Close())
	if used := budget.Used(); used != 0 {
		t.Fatalf("real-closure derivative retained %d bytes", used)
	}
}
