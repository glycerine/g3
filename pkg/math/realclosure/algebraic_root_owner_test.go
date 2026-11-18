package realclosure

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestAlgebraicRootsShareSignDetAndReleaseSourceOrder(t *testing.T) {
	budget := arena.NewBudget(1 << 25)
	service, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	intervals, err := numeral.NewBinaryRationalManager(service.Manager.Z, numeral.BinaryRationalConfig{Diagnostics: service})
	if err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	store := &realValueStore{numbers: service.Manager}
	one, err := store.newRational(1, 1)
	must(err)
	q, err := store.newRational(2, 1)
	must(err)
	state := newTestSignDetermination(2)
	must(state.attachInitialPolynomial(store, one))
	kept, allOne, err := state.advanceWithPolynomials(1, 1, 0, []*realValue{q}, []int{2, 1}, [][]*realValue{{one}, {q}})
	must(err)
	if !kept || !allOne {
		t.Fatal("fixture did not produce two sign conditions")
	}
	det, err := state.finish()
	must(err)
	lo, hi := numeral.NewBinaryRational(), numeral.NewBinaryRational()
	must(intervals.SetInt32(&lo, 0))
	must(intervals.SetInt32(&hi, 3))
	enclosure, err := NewBinaryInterval(intervals)
	must(err)
	isolating, err := NewBinaryInterval(intervals)
	must(err)
	must(enclosure.SetFinite(&lo, &hi, false, false))
	must(isolating.SetFinite(&lo, &hi, false, false))
	owner := &algebraicRootOwner{values: store, intervals: intervals}
	first, err := owner.mkAlgebraic([]*realValue{one, q}, enclosure, isolating, det, 0)
	must(err)
	second, err := owner.mkAlgebraic([]*realValue{one, q}, enclosure, isolating, det, 1)
	must(err)
	if det.refs != 2 || first.index != 0 || second.index != 1 || first.refs != 0 || second.refs != 0 {
		t.Fatal("algebraic roots did not register and retain their sign owner")
	}
	firstValue, err := store.newAlgebraicValue(first, one)
	must(err)
	secondValue, err := store.newAlgebraicValue(second, one)
	must(err)
	if first.refs != 1 || second.refs != 1 || firstValue.isRational || len(firstValue.functionNumerator.coefficients) != 2 || firstValue.functionNumerator.coefficients[0] != nil || firstValue.functionNumerator.coefficients[1] != one {
		t.Fatal("rational-function values did not retain algebraic roots and numerator x")
	}
	if count, err := first.numRootsInsideInterval(); err != nil || count != 2 {
		t.Fatalf("first root sees %d roots: %v", count, err)
	}
	must(intervals.SetInt32(&hi, 4))
	must(enclosure.SetFinite(&lo, &hi, false, false))
	var output bytes.Buffer
	must(first.interval.Display(&output))
	if output.String() != "[0, 3]" {
		t.Fatalf("root interval aliased the source: %q", output.String())
	}
	output.Reset()
	must(firstValue.functionInterval.Display(&output))
	if output.String() != "[0, 3]" {
		t.Fatalf("rational-function value interval aliased the source: %q", output.String())
	}
	must(store.release(one))
	must(store.release(q))
	must(store.release(firstValue))
	if det.refs != 1 || det.deleted || one.deleted || q.deleted || owner.roots[0] != nil || owner.roots[1] != second {
		t.Fatal("first root deletion changed the shared result or second registry slot")
	}
	must(store.release(secondValue))
	if !det.deleted || !one.deleted || !q.deleted || owner.roots[1] != nil || !first.deleted || !second.deleted {
		t.Fatal("last root deletion retained sign state or coefficients")
	}
	must(enclosure.Close())
	must(isolating.Close())
	must(intervals.Del(&lo))
	must(intervals.Del(&hi))
	must(intervals.CloseChecked())
	must(service.Close())
	if used := budget.Used(); used != 0 {
		t.Fatalf("algebraic roots retained %d bytes", used)
	}
}
