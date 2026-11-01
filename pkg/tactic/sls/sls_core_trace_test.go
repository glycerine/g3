package sls

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type slsRefusingTrace struct{}

func (slsRefusingTrace) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestSLSCoreSourceTraceAndTacticReport(t *testing.T) {
	for _, mode := range []string{"disabled", "enabled", "refusing"} {
		t.Run(mode, func(t *testing.T) {
			_, owner, _, _, context, globals, local, budget := slsTrackerFixture(t)
			var trace bytes.Buffer
			var traceWriter io.Writer = &trace
			if mode == "refusing" {
				traceWriter = slsRefusingTrace{}
			}
			tags := diagnostic.NewTraceTagsWithWriter(traceWriter)
			if mode != "disabled" {
				tags.Enable("sls")
			}
			manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: owner.Symbols, Numerals: owner.NumeralService(), Budget: budget, TraceTags: tags})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := manager.Close(); err != nil {
					t.Error(err)
				}
			})
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{})
			if err != nil {
				t.Fatal(err)
			}
			previous, err := diagnostic.BorrowProcessVerboseStream()
			if err != nil {
				t.Fatal(err)
			}
			level, err := diagnostic.ProcessVerbosityLevel()
			if err != nil {
				t.Fatal(err)
			}
			if err = diagnostic.SetProcessVerboseStream(stream); err != nil {
				t.Fatal(err)
			}
			if err = diagnostic.SetProcessVerbosityLevel(10); err != nil {
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
			core, err := NewCoreTactic(manager, context, globals, local)
			if err != nil {
				t.Fatal(err)
			}
			defer core.Close()
			p := slsBoolConst(t, manager, "sls-trace-p")
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
			if err = core.Apply(goal, &result); err != nil {
				t.Fatal(err)
			}
			text, err := stream.String()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(text, "(sls :num-exprs 0 :num-asts ") {
				t.Fatal("missing source tactic report", text)
			}
			if !tags.ShouldTrace("sls") || mode == "refusing" {
				if trace.Len() != 0 {
					t.Fatal("unexpected trace", trace.String())
				}
				return
			}
			record := func(line int, body string) string {
				return fmt.Sprintf("-------- [sls] operator() src/tactic/sls/sls_tactic.cpp:%d ---------\n%s------------------------------------------------\n", line, body)
			}
			want := record(70, "(goal\n  sls-trace-p)\n") + record(77, "(goal)\n")
			if trace.String() != want {
				t.Fatalf("source trace %q, want %q", trace.String(), want)
			}
		})
	}
}
