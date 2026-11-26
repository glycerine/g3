package sat

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDIMACSRepairValidatesMixedModel(t *testing.T) {
	limit, err := control.NewReslimit(nil)
	satOK(t, err)
	defer limit.Close()

	// Repeated literals count independently. Both tautologies stay satisfied
	// while their variables change sign during the search.
	const input = "p cnf 2 4\n1 1 0\n-2 -2 0\n1 -1 0\n2 -2 0\n"
	cnf, complete := readDIMACSModelFormula(strings.NewReader(input), len(input), limit)
	if !complete {
		t.Fatal("complete mixed formula was refused")
	}
	got := repairDIMACSModel(cnf, limit)
	if len(got) != 3 || got[1] != 2 || got[2] != 5 {
		t.Fatalf("unexpected assignment %v", got)
	}

	for _, incorrectSize := range []int{len(input) - 1, len(input) + 1} {
		if _, complete = readDIMACSModelFormula(strings.NewReader(input), incorrectSize, limit); complete {
			t.Fatalf("accepted incorrect size %d for %d input bytes", incorrectSize, len(input))
		}
	}
}

func TestDIMACSRepairBoundedMiss(t *testing.T) {
	limit, err := control.NewReslimit(nil)
	satOK(t, err)
	defer limit.Close()
	// The binary clauses forbid all four assignments, without a unit or empty
	// clause. The heuristic must exhaust its bound and let the solver decide.
	const input = "1 2 0\n1 -2 0\n-1 2 0\n-1 -2 0\n"
	cnf, complete := readDIMACSModelFormula(strings.NewReader(input), len(input), limit)
	if !complete {
		t.Fatal("fixture did not parse")
	}
	if model := repairDIMACSModel(cnf, limit); model != nil {
		t.Fatalf("repair claimed an unsatisfiable formula has model %v", model)
	}
	limit.Cancel()
	if _, complete = readDIMACSModelFormula(strings.NewReader(input), len(input), limit); complete {
		t.Fatal("canceled scan completed")
	}
}

func TestDIMACSFileRepairMissLoadsOrdinarySolver(t *testing.T) {
	// A high variable index appears before the contradictory binary component.
	// Replay must allocate the same variables and install the same clauses in
	// input order, including the duplicated literal and the ignored header.
	const input = "c bounded repair miss\np cnf 2 2\n5 -1 5 0\n1 2 0\n1 -2 0\n-1 2 0\n-1 -2 0\n"
	replay, _, _ := satSolverCheckFixture(t)
	stream, _, _ := satSolverCheckFixture(t)
	model, loaded, err := replay.PrepareDIMACSFile(strings.NewReader(input), len(input))
	satOK(t, err)
	if model != nil || !loaded {
		t.Fatalf("expected a complete repair miss and replay, got model=%v loaded=%t", model, loaded)
	}
	satOK(t, ParseDIMACS(strings.NewReader(input), stream))
	if replay.Variables.NumVars() != 6 || stream.Variables.NumVars() != 6 || replay.Lifecycle.mkBinary != stream.Lifecycle.mkBinary {
		t.Fatal("replay changed input variable or clause registration")
	}
	want, err := stream.Check(nil)
	satOK(t, err)
	got, err := replay.Check(nil)
	satOK(t, err)
	if want != lbool.False || got != want || replay.decision.decisions != stream.decision.decisions || replay.conflict.conflictStat != stream.conflict.conflictStat {
		t.Fatalf("replay got %v with %d decisions and %d conflicts, stream got %v with %d decisions and %d conflicts",
			got, replay.decision.decisions, replay.conflict.conflictStat, want, stream.decision.decisions, stream.conflict.conflictStat)
	}
}
