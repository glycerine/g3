package sat

import "testing"

func TestSATMUSToVectorCopiesLiteralSetInSourceOrder(t *testing.T) {
	s, _, _ := satSolverCheckFixture(t)
	var set satMUSLiteralSet
	for _, literal := range []Literal{7, 1, 5} {
		satOK(t, set.insert(literal))
	}
	var vector satUintColumn
	vector.Bind(s.runtime.memory)
	satOK(t, s.mus.toVector(&set, &vector))
	for i, want := range []uint32{7, 1, 5} {
		got, err := vector.Read(uint32(i))
		satOK(t, err)
		if got != want {
			t.Fatal(i, got, want)
		}
	}
	satOK(t, vector.Finalize())
}
