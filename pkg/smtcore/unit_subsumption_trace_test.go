package smtcore

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestUnitSubsumptionSourceTrace(t *testing.T) {
	for _, mode := range []string{"disabled", "enabled", "solver-owned-parameters", "no-deletion", "refusing"} {
		t.Run(mode, func(t *testing.T) {
			owner, config := smtModelGeneratorFixture(t)
			var output bytes.Buffer
			var writer io.Writer = &output
			if mode == "refusing" {
				writer = smtTacticRefusingTrace{}
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			if mode != "disabled" {
				tags.Enable("unit_subsumption_tactic")
			}
			m, err := ast.NewASTManager(ast.ASTManagerConfig{
				Symbols: owner.Symbols, Numerals: owner.NumeralService(), Budget: owner.Budget(), TraceTags: tags,
			})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, m.Close()) })
			goal, err := tactic.NewGoal(m, false, false, false)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, goal.Close()) })
			a := goalTestAtom(t, m, "trace-unit-a")
			b := goalTestAtom(t, m, "trace-unit-b")
			weak, err := m.MkOr(a, b)
			astTestMust(t, err)
			forms := []ast.ASTHandle{weak, a, a}
			if mode == "no-deletion" {
				forms = []ast.ASTHandle{a, b}
			}
			for _, form := range forms {
				astTestMust(t, goal.Assert(form))
			}
			solverConfig := SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: config}
			if mode == "solver-owned-parameters" {
				solverConfig.Globals = nil
			}
			unit, err := NewUnitSubsumptionTactic(m, nil, solverConfig)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, unit.Close()) })
			var result tactic.TacticResult
			t.Cleanup(func() { astTestMust(t, result.Close()) })
			oldGlobals := uparams.BorrowProcessGlobalParamsService()
			astTestMust(t, unit.Apply(goal, &result))
			if uparams.BorrowProcessGlobalParamsService() != oldGlobals {
				t.Fatal("trace did not restore process printer parameters")
			}
			wantCount := uint32(1)
			if mode == "no-deletion" {
				wantCount = 2
			}
			if count, err := goal.Size(); err != nil || count != wantCount {
				t.Fatal("trace changed result", count, err)
			}
			if !tags.ShouldTrace("unit_subsumption_tactic") || mode == "refusing" {
				if output.Len() != 0 {
					t.Fatal("unexpected trace", output.String())
				}
				return
			}
			record := func(function string, line int, body string) string {
				return fmt.Sprintf("-------- [unit_subsumption_tactic] %s src/smt/tactic/unit_subsumption_tactic.cpp:%d ---------\n%s------------------------------------------------\n", function, line, body)
			}
			want := record("prune_clause", 105, "Removing clause 0\n") +
				record("prune_clause", 105, "Removing clause 1\n") +
				record("reduce_core", 76, "(goal\n  trace-unit-a)\n(goal\n  trace-unit-a)\n")
			if mode == "no-deletion" {
				want = record("reduce_core", 76, "(goal\n  trace-unit-a\n  trace-unit-b)\n(goal\n  trace-unit-a\n  trace-unit-b)\n")
			}
			if got := output.String(); got != want {
				t.Fatalf("trace = %q, want %q", got, want)
			}
		})
	}
}
