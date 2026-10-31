package arith

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/text"
)

func TestRecover01SourceProgressReports(t *testing.T) {
	for _, tc := range []struct {
		name              string
		complete, verbose bool
	}{{"two recovered variables", true, true}, {"incomplete", false, true}, {"verbosity gate", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			base, m := fixDLFixture(t, false)
			sort, err := m.MkBoolSort()
			equationMust(t, err)
			symbol, err := m.Symbols.Intern("recover-progress-p")
			equationMust(t, err)
			p, err := m.MkConst(symbol, sort)
			equationMust(t, err)
			fixDLPin(t, m, p)
			notP, err := m.MkNot(p)
			equationMust(t, err)
			fixDLPin(t, m, notP)
			var clauses []ast.ASTHandle
			for _, name := range []string{"recover-progress-x", "recover-progress-y"} {
				x := fixDLConst(t, m, true, name)
				for index, lit := range []ast.ASTHandle{p, notP} {
					if index == 1 && !tc.complete {
						break
					}
					value := "0"
					if index == 1 {
						value = "3"
					}
					eq := collectorEq(t, m, x, fixDLNumber(t, m, true, value))
					clause, e := m.MkOrArgs([]ast.ASTHandle{lit, eq})
					equationMust(t, e)
					fixDLPin(t, m, clause)
					clauses = append(clauses, clause)
				}
			}
			goal := fixDLGoal(t, m, false, true, false, clauses...)
			stream, err := text.NewNumericTextStream(text.NumericTextStreamConfig{Budget: arena.NewBudget(1 << 20)})
			equationMust(t, err)
			oldStream, err := diagnostic.BorrowProcessVerboseStream()
			equationMust(t, err)
			oldLevel, err := diagnostic.ProcessVerbosityLevel()
			equationMust(t, err)
			equationMust(t, diagnostic.SetProcessVerboseStream(stream))
			t.Cleanup(func() {
				equationMust(t, diagnostic.SetProcessVerboseStream(oldStream))
				equationMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
				equationMust(t, stream.Close())
			})
			level := uint32(9)
			if tc.verbose {
				level = 10
			}
			equationMust(t, diagnostic.SetProcessVerbosityLevel(level))
			var out tactic.TacticResult
			defer func() { equationMust(t, out.Close()) }()
			equationMust(t, recover01Apply(goal, base.rewriter, base.services, 10, &out))
			got, err := stream.String()
			equationMust(t, err)
			if strings.Contains(got, "(:recovered-01-vars 2)\n") != (tc.complete && tc.verbose) {
				t.Fatalf("progress report: %q", got)
			}
			if strings.Contains(got, "(recover-01 :num-exprs ") != tc.verbose {
				t.Fatalf("tactic report: %q", got)
			}
		})
	}
}
