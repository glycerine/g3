package sat

// Source: src/sat/sat_scc.cpp:225-250; src/sat/sat_scc_params.pyg.

import "testing"

func TestSATSCCTransitiveReductionDispatchParameterAndStatistic(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			s := satBIGRedundantFixture(t)
			s.scc.transitive = enabled
			n, err := s.scc.run()
			satOK(t, err)
			if n != 0 {
				t.Fatal("no SCC variables should be eliminated", n)
			}
			pairs, err := s.appendBinaryClauses(nil, false)
			satOK(t, err)
			want := 3
			if enabled {
				want = 2
			}
			if len(pairs) != want || s.scc.numElimBinary != uint32(3-want) {
				t.Fatal(enabled, pairs, s.scc.numElimBinary)
			}
		})
	}
}

func TestSATSCCTransitiveParameterUpdate(t *testing.T) {
	s, ctx, params := satSolverCheckFixture(t)
	if !s.scc.transitive {
		t.Fatal("source scc.tr default")
	}
	key := "scc.tr"
	satOK(t, ctx.SetBoolName(&params, &key, false))
	satOK(t, s.UpdateParams(&params))
	if s.scc.transitive {
		t.Fatal("local scc.tr parameter was ignored")
	}
}
