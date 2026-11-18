package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSignDeterminationPolynomialRowsRetainSharedValues(t *testing.T) {
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
	q, err := store.newRational(2, 1)
	must(err)
	state := newTestSignDetermination(2)
	must(state.attachInitialPolynomial(store, one))
	kept, allOne, err := state.advanceWithPolynomials(1, 1, 0, []*realValue{q}, []int{2, 1}, [][]*realValue{{one}, {q}})
	must(err)
	if !kept || !allOne || len(state.prs) != 2 || len(state.qs) != 1 || state.prs[0].coefficients[0] != one || state.prs[1].coefficients[0] != q || state.qs[0].coefficients[0] != q {
		t.Fatal("sign determination lost its concrete polynomial rows")
	}
	must(store.release(one))
	must(store.release(q))
	if one.refs != 1 || q.refs != 2 {
		t.Fatalf("wrong polynomial ownership: one=%d, q=%d", one.refs, q.refs)
	}
	if kept, allOne, err = state.advanceWithPolynomials(2, 0, 0, nil, nil, nil); err != nil || kept || allOne || one.refs != 1 || q.refs != 2 {
		t.Fatalf("discarded derivative changed polynomial ownership: kept=%v, allOne=%v, err=%v", kept, allOne, err)
	}
	must(state.closePolynomials())
	if !one.deleted || !q.deleted {
		t.Fatal("sign determination did not release its polynomial coefficients")
	}
	must(service.Close())
	if used := budget.Used(); used != 0 {
		t.Fatalf("real-closure polynomial rows retained %d bytes", used)
	}
}
