package sat

import (
	"bytes"
	"fmt"
	"os"
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

func TestDIMACSDenseRepairFindsVerifiedModel(t *testing.T) {
	// The short probe and a longer greedy walk both miss this planted model.
	source, err := os.ReadFile("test_vectors/dimacs_dense_planted.cnf")
	satOK(t, err)
	s, _, _ := satSolverCheckFixture(t)
	model, loaded, err := s.PrepareDIMACSFile(bytes.NewReader(source), len(source))
	satOK(t, err)
	if model == nil || loaded || s.Variables.NumVars() != 0 {
		t.Fatal("dense repair did not find a model before solver ingestion")
	}
	cnf, complete := readDIMACSModelFormula(bytes.NewReader(source), len(source), s.Limit)
	if !complete || len(model) != int(cnf.variables)+1 {
		t.Fatal("model does not cover the parsed variables")
	}
	var start uint32
	for i, end := range cnf.ends {
		satisfied := false
		for _, lit := range cnf.literals[start:end] {
			satisfied = satisfied || model[lit.variable()] == lbool.FromBool(!lit.sign())
		}
		if !satisfied {
			t.Fatalf("model falsifies clause %d", i)
		}
		start = end
	}
}

func TestDIMACSDenseRepairFallsBackOnContradictoryComponent(t *testing.T) {
	source, err := os.ReadFile("test_vectors/dimacs_dense_planted.cnf")
	satOK(t, err)
	// These eight ternary clauses exclude every assignment to three fresh
	// variables while keeping the formula within the dense repair bounds.
	for signs := 0; signs < 8; signs++ {
		var term [3]int
		for i := range term {
			term[i] = 257 + i
			if signs&(1<<i) != 0 {
				term[i] = -term[i]
			}
		}
		source = fmt.Appendf(source, "%d %d %d 0\n", term[0], term[1], term[2])
	}
	s, _, _ := satSolverCheckFixture(t)
	model, loaded, err := s.PrepareDIMACSFile(bytes.NewReader(source), len(source))
	satOK(t, err)
	if model != nil || !loaded {
		t.Fatalf("dense contradiction yielded model=%v loaded=%t", model, loaded)
	}
	answer, err := s.Check(nil)
	satOK(t, err)
	if answer != lbool.False {
		t.Fatalf("ordinary fallback returned %v", answer)
	}
}
