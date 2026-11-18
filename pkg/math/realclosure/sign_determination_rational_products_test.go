package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSignDeterminationRationalProductsAndThreeWayExpansion(t *testing.T) {
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
	product, err := store.multiplyRationalPolynomials([]*realValue{one, one}, []*realValue{one, minusOne})
	must(err)
	if len(product) != 3 || product[1] != nil || product[0] == nil || product[2] == nil {
		t.Fatalf("(1+x)(1-x) coefficients: %v", product)
	}
	first, err := store.numbers.Equal(&product[0].rational, &one.rational)
	must(err)
	last, err := store.numbers.Equal(&product[2].rational, &minusOne.rational)
	must(err)
	if !first || !last || one.refs != 1 || minusOne.refs != 1 {
		t.Fatal("convolution changed its borrowed operands or failed cancellation")
	}
	must(store.releaseCoefficients(product))
	state := newTestSignDetermination(3)
	must(state.attachInitialPolynomial(store, one))
	// For roots -1, 0, 1, q=x has signs -, 0, +. The source Tarski
	// results for 1, q, q² are 3, 0, 2, respectively.
	kept, allOne, err := state.advanceFromTarski([]*realValue{nil, one}, 0, 2, []int{3, 0, 2})
	must(err)
	if !kept || !allOne || len(state.prs) != 3 || len(state.qs) != 1 || len(state.prs[0].coefficients) != 1 || len(state.prs[1].coefficients) != 2 || len(state.prs[2].coefficients) != 3 || state.prs[1].coefficients[0] != nil || state.prs[2].coefficients[0] != nil || state.prs[2].coefficients[1] != nil {
		t.Fatal("three-way sign expansion did not construct 1, q, and q² rows")
	}
	if cardinalities, ok := state.matrix.solveIntT(state.tarski); !ok || len(cardinalities) != 3 || cardinalities[0] != 1 || cardinalities[1] != 1 || cardinalities[2] != 1 {
		t.Fatalf("expanded rows did not separate the three roots: %v, %v", cardinalities, ok)
	}
	for _, row := range state.prs {
		lastCoefficient := row.coefficients[len(row.coefficients)-1]
		equal, err := store.numbers.Equal(&lastCoefficient.rational, &one.rational)
		must(err)
		if !equal {
			t.Fatal("expanded polynomial has an incorrect rational coefficient")
		}
	}
	must(state.closePolynomials())
	must(store.release(one))
	must(store.release(minusOne))
	must(service.Close())
	if used := budget.Used(); used != 0 {
		t.Fatalf("real-closure polynomial expansion retained %d bytes", used)
	}
}
