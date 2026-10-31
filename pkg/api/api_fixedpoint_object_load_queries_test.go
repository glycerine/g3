package api

import (
	"strings"
	"testing"
)

func TestAPIFixedpointObjectLoadSMT2Queries(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	v, err := f.fp.LoadSMT2WithQueries(strings.NewReader(`(declare-rel R (Int Bool)) (declare-rel S ()) (query R) (fixedpoint-push) (query S) (fixedpoint-pop) (query S)`))
	astTestMust(t, err)
	defer func() { astTestMust(t, v.Close()) }()
	values, err := v.Values()
	astTestMust(t, err)
	if len(values) != 2 {
		t.Fatalf("got %d queries", len(values))
	}
	m, err := f.c.Manager()
	astTestMust(t, err)
	for i, want := range []uint32{2, 0} {
		n, e := m.GetNumArgs(values[i])
		astTestMust(t, e)
		if n != want {
			t.Fatalf("query %d has %d arguments", i, n)
		}
		for j := uint32(0); j < n; j++ {
			a, e := m.GetArg(values[i], j)
			astTestMust(t, e)
			idx, e := m.GetVarIndex(a)
			astTestMust(t, e)
			if idx != j {
				t.Fatalf("variable %d has index %d", j, idx)
			}
		}
	}
}
