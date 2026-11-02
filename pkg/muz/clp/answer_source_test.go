package clp_test

// Source: src/muz/clp/clp_context.cpp imp::get_answer/display_certificate.
// The CLP answer is constructed as true on demand and is not engine-owned state.

import (
	"testing"

	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestCLPAnswerIsStatelessSourceTruth(t *testing.T) {
	f := newFixture(t)
	if err := f.m.EnableDebugRefCount(); err != nil {
		t.Fatal(err)
	}
	truth, err := f.m.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	before, err := f.m.GetRefCount(truth)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := clp.NewEngine(f.m, f.set, f.solver, 20)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		status, err := engine.Query(f.app["Q"])
		if err != nil || status != lbool.True {
			t.Fatal("query", i, status, err)
		}
		answer, err := engine.Answer()
		if err != nil || answer != truth {
			t.Fatal("source answer", i, answer, err)
		}
		after, err := f.m.GetRefCount(truth)
		if err != nil || after != before {
			t.Fatal("query stored an answer owner", i, before, after, err)
		}
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := f.m.GetRefCount(truth)
	if err != nil || after != before {
		t.Fatal("close changed stateless answer ownership", before, after, err)
	}
}
