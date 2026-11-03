package qe

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestLiteConstantsQuantifierTraceFailureReleasesRegisteredResult(t *testing.T) {
	f := newFMFixture(t)
	m := f.r.manager
	l := f.lite(false)
	constant := f.constant("lite-quantifier-trace", f.boolean)
	// Reuse the abstracted variable so the new quantifier is the first
	// constructor to write to the ordinary AST trace stream.
	f.variable(0, f.boolean)
	variables := f.conjunctions(constant)
	formula := f.conjunctions(constant)
	variableRef, formulaRef := (*variables)[0], (*formula)[0]
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	fmMust(t, err)
	fmMust(t, stream.Close())
	counts := func() [4]uint32 {
		s := m.Stats()
		return [4]uint32{s.LiveNodes, s.LiveInfos, s.LiveParameters, s.LiveReferences}
	}
	before := counts()
	for run := 0; run < 2; run++ {
		m.TraceStream = stream
		err = l.EliminateConstants(variables, formulaRef)
		m.TraceStream = nil
		if !errors.Is(err, arena.ErrClosed) {
			t.Fatalf("run %d: quantifier trace error = %v", run, err)
		}
		if len(*variables) != 1 || (*variables)[0] != variableRef || (*formula)[0] != formulaRef {
			t.Fatalf("run %d: failed quantifier construction changed caller references", run)
		}
		for _, ref := range []struct {
			name string
			root ast.ASTRef
		}{{"formula", formulaRef}, {"variable", variableRef}} {
			got, err := m.RefGet(ref.root)
			fmMust(t, err)
			if got != constant {
				t.Fatalf("run %d: %s changed to %v", run, ref.name, got)
			}
		}
		// Source qe_lite.cpp owns q through quantifier_ref before mk_exists.
		// Check before manager teardown can collect an unowned registered q.
		if after := counts(); after != before {
			t.Fatalf("run %d: quantifier failure AST counts = %v, want %v", run, after, before)
		}
		if len(l.pending) != 0 {
			t.Fatalf("run %d: completed cleanup left %d pending calls", run, len(l.pending))
		}
	}
	fmMust(t, l.EliminateConstants(variables, formulaRef))
	result, err := m.RefGet(formulaRef)
	fmMust(t, err)
	if !m.IsTrue(result) || len(*variables) != 0 {
		t.Fatalf("same-owner retry: result %v, remaining variables %d", result, len(*variables))
	}
}
