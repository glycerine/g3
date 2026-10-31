package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
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

func TestAPIFixedpointObjectLoadQueriesIsolatedAndTransfersRules(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	_, err := f.fp.LoadSMT2WithQueries(strings.NewReader(`(declare-rel Bad ()) (rule Bad bad) (assert`))
	if err == nil {
		t.Fatal("expected parse error")
	}
	before, err := f.fp.Rules()
	astTestMust(t, err)
	releaseAPIRefs(t, f.m, before)
	if len(before) != 0 {
		t.Fatal("failed parse added rules", len(before))
	}
	v, err := f.fp.LoadSMT2WithQueries(strings.NewReader(`(declare-rel R ()) (rule R loaded) (query R)`))
	astTestMust(t, err)
	defer func() { astTestMust(t, v.Close()) }()
	values, err := v.Values()
	astTestMust(t, err)
	if len(values) != 1 {
		t.Fatal("queries", len(values))
	}
	decl, err := f.m.GetDecl(values[0])
	astTestMust(t, err)
	rules, names, err := f.fp.command.FixedpointNamedRules()
	astTestMust(t, err)
	releaseAPIRefs(t, f.m, rules)
	if len(names) != 1 || names[0] != "loaded" {
		t.Fatal("loaded rule names", names)
	}
	status, err := f.fp.Query(decl)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("loaded fact query", status)
	}
}
