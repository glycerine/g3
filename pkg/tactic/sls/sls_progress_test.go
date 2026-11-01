package sls

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSLSProgressAndSearchSummary(t *testing.T) {
	for _, tc := range []struct {
		name        string
		level       uint32
		maxRestarts uint32
	}{
		{"quiet-progress", 9, 3},
		{"verbose-progress", 10, 3},
		{"zero-restarts", 10, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine, _, _, _, manager, _ := slsEngineFixture(t)
			engine.maxRestarts = tc.maxRestarts
			previous, err := diagnostic.BorrowProcessVerboseStream()
			if err != nil {
				t.Fatal(err)
			}
			level, err := diagnostic.ProcessVerbosityLevel()
			if err != nil {
				t.Fatal(err)
			}
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := diagnostic.SetProcessVerboseStream(previous); err != nil {
					t.Error(err)
				}
				if err := diagnostic.SetProcessVerbosityLevel(level); err != nil {
					t.Error(err)
				}
				if err := stream.Close(); err != nil {
					t.Error(err)
				}
			})
			if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
				t.Fatal(err)
			}
			if err = diagnostic.SetProcessVerbosityLevel(tc.level); err != nil {
				t.Fatal(err)
			}
			p := slsBoolConst(t, manager, "sls-progress-p")
			slsPin(t, manager, p)
			goal, err := tactic.NewGoal(manager, false, true, false)
			if err != nil {
				t.Fatal(err)
			}
			defer goal.Close()
			if err = goal.Assert(p); err != nil {
				t.Fatal(err)
			}
			var result tactic.TacticResult
			defer result.Close()
			if _, err = ApplyGoal(engine, goal, &result); err != nil {
				t.Fatal(err)
			}
			got, err := stream.String()
			if err != nil {
				t.Fatal(err)
			}
			search := fmt.Sprintf("(Searching... restarts left: %d)\n", tc.maxRestarts)
			summary := fmt.Sprintf("(restarts: 0 flips: %d fps: ", engine.stats.Moves)
			flips := fmt.Sprintf("(Number of flips: %d)\n", engine.stats.Moves)
			if strings.Count(got, summary) != 1 || !strings.Contains(got, ")\n") {
				t.Fatalf("missing source search summary: %q", got)
			}
			if (strings.Contains(got, search)) != (tc.level >= 10 && tc.maxRestarts != 0) {
				t.Fatalf("restart progress gate: %q", got)
			}
			if (strings.Contains(got, flips)) != (tc.level >= 10 && engine.stats.Moves != 0) {
				t.Fatalf("flip progress gate: %q", got)
			}
			if tc.level >= 10 && tc.maxRestarts != 0 && strings.Index(got, search) > strings.Index(got, summary) {
				t.Fatalf("progress appeared after the search summary: %q", got)
			}
			if tc.level >= 10 && engine.stats.Moves != 0 && strings.Index(got, flips) < strings.Index(got, summary) {
				t.Fatalf("flip progress preceded the search summary: %q", got)
			}
		})
	}
}

func TestSLSSearchSummaryAfterExhaustedRestarts(t *testing.T) {
	engine, _, _, _, manager, _ := slsEngineFixture(t)
	engine.maxRestarts, engine.restartNext = 0, 0
	previous, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		t.Fatal(err)
	}
	level, err := diagnostic.ProcessVerbosityLevel()
	if err != nil {
		t.Fatal(err)
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := diagnostic.SetProcessVerboseStream(previous); err != nil {
			t.Error(err)
		}
		if err := diagnostic.SetProcessVerbosityLevel(level); err != nil {
			t.Error(err)
		}
		if err := stream.Close(); err != nil {
			t.Error(err)
		}
	})
	if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
		t.Fatal(err)
	}
	if err = diagnostic.SetProcessVerbosityLevel(10); err != nil {
		t.Fatal(err)
	}
	p := slsBoolConst(t, manager, "sls-progress-conflict")
	notP, err := manager.MkNot(p)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, p, notP)
	for _, formula := range []ast.ASTHandle{p, notP} {
		if err = engine.AssertExpression(formula); err != nil {
			t.Fatal(err)
		}
	}
	status, err := engine.Solve()
	if err != nil || status != lbool.Undef || engine.stats.Restarts != 1 {
		t.Fatal("exhausted search", status, engine.stats.Restarts, err)
	}
	got, err := stream.String()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "(restarts: 1 flips: ") || strings.Contains(got, "Searching...") || strings.Contains(got, "Number of flips:") {
		t.Fatal("exhausted search diagnostics", got)
	}
}
