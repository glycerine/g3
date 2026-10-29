package qe

import (
	"testing"

	"github.com/glycerine/g3/pkg/nlsat"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestNLQSearchTerminalAlternatingChecks(t *testing.T) {
	for _, tc := range []struct {
		name       string
		truthUnit  bool
		want       lbool.LBool
		wantRounds uint32
	}{
		{name: "true", truthUnit: true, want: lbool.True, wantRounds: 2},
		{name: "false", truthUnit: false, want: lbool.False, wantRounds: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			solver, _ := nlqStateFixture(t)
			variable, err := solver.MkBoolVar()
			fmMust(t, err)
			truth := sat.MakeLiteral(variable, false)
			unit := truth.Negated()
			if tc.truthUnit {
				unit = truth
			}
			fmMust(t, solver.AddClause([]sat.Literal{unit}, nlsat.Assumption{}))
			search, err := newNLQSearch(solver, truth, nil, nil, nil, map[sat.BoolVar]bool{variable: true})
			fmMust(t, err)
			t.Cleanup(func() { fmMust(t, search.close()) })
			status, err := search.check()
			fmMust(t, err)
			if status != tc.want || search.rounds != tc.wantRounds {
				t.Fatal("alternating terminal result", status, search.rounds, tc.want, tc.wantRounds)
			}
		})
	}
}

func TestNLQSearchProjectsConflictAndBacktracksEvenScopes(t *testing.T) {
	solver, literals := nlqStateFixture(t)
	variable, err := solver.MkBoolVar()
	fmMust(t, err)
	truth := sat.MakeLiteral(variable, false)
	fmMust(t, solver.AddClause([]sat.Literal{truth}, nlsat.Assumption{}))
	status, err := solver.Check()
	fmMust(t, err)
	if status != lbool.True {
		t.Fatal("projection fixture is not satisfiable", status)
	}
	levels := []MaxLevel{
		{Existential: 0, Universal: NoQuantifierLevel},
		{Existential: NoQuantifierLevel, Universal: 1},
		{Existential: 2, Universal: NoQuantifierLevel},
	}
	search, err := newNLQSearch(solver, truth, levels, [][]uint32{{0}, {1}, {2}}, nil, map[sat.BoolVar]bool{variable: true})
	fmMust(t, err)
	t.Cleanup(func() { fmMust(t, search.close()) })
	fmMust(t, search.state.saveModel())
	fmMust(t, search.state.push())
	fmMust(t, search.state.push())
	fmMust(t, search.projectConflict([]sat.Literal{literals[2]}))
	if search.state.level() != 0 || search.state.validModel {
		t.Fatal("projected conflict did not backtrack two scopes", search.state.level(), search.state.validModel)
	}
	// Eliminating x2 > 0 and the unused x1 leaves only the parity blocker.
	status, err = solver.CheckAssumptions(&[]sat.Literal{truth})
	fmMust(t, err)
	if status != lbool.False {
		t.Fatal("projected parity clause was not learned", status)
	}
}
