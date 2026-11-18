package realclosure

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSignDetSourceArrayPublicationAndSharedRootLifetime(t *testing.T) {
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
	if !kept || !allOne {
		t.Fatal("derivative did not distinguish the two roots")
	}
	must(store.release(one))
	must(store.release(q))
	first, second := state.conditions[0], state.conditions[1]
	det, err := state.finish()
	must(err)
	if det.refs != 0 || det.deleted || det.matrix.rows != 2 || det.matrix.cols != 2 || len(det.prs) != 2 || len(det.qs) != 1 || len(det.tarski) != 2 || len(det.conditions) != 2 || state.store != nil || len(state.prs) != 0 || len(state.conditions) != 0 {
		t.Fatal("sign determination arrays were not published and transferred")
	}
	if det.prs[0].coefficients[0] != one || det.prs[1].coefficients[0] != q || det.qs[0].coefficients[0] != q || det.conditions[0] != first || det.conditions[1] != second || one.refs != 1 || q.refs != 2 {
		t.Fatal("publication changed source coefficient or condition identity")
	}
	must(det.retain())
	must(det.retain())
	must(det.release())
	if det.deleted || one.deleted || q.deleted || first.deleted || second.deleted {
		t.Fatal("first root release deleted shared sign determination")
	}
	must(det.release())
	if !det.deleted || det.refs != 0 || !one.deleted || !q.deleted || !first.deleted || !second.deleted || det.matrix.rows != 0 || det.prs != nil || det.qs != nil || det.tarski != nil || det.conditions != nil {
		t.Fatal("last root release did not delete all sign-determination arrays")
	}
	if err := det.retain(); err == nil {
		t.Fatal("deleted sign determination accepted a new root")
	}
	shared := &signCondition{polynomial: 0, sign: 1}
	left := &signCondition{polynomial: 1, sign: 0, previous: shared}
	right := &signCondition{polynomial: 1, sign: -1, previous: shared}
	sharedDet := &signDet{store: store, refs: 1, conditions: []*signCondition{left, right}}
	must(sharedDet.release())
	if !left.deleted || !right.deleted || !shared.deleted || left.previous != nil || right.previous != nil {
		t.Fatal("shared sign-condition antecedent was not deleted once")
	}
	must(service.Close())
	if used := budget.Used(); used != 0 {
		t.Fatalf("real-closure sign determination retained %d bytes", used)
	}
}
