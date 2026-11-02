package spacer

// Source: src/muz/spacer/spacer_context.cpp:2436-2468.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSpacerGroundAnswerStatusDiagnosticsAndOwnership(t *testing.T) {
	for _, phase := range []string{"unsolved", "undef", "unsat", "sat"} {
		for _, level := range []uint32{0, 1} {
			t.Run(fmt.Sprintf("%s/%d", phase, level), func(t *testing.T) {
				f := newContextAnswerFixture(t, phase != "sat")
				c, m := f.context, f.context.manager
				switch phase {
				case "undef":
					status, err := c.Solve(0, 0)
					depthMust(t, err)
					if status != lbool.Undef {
						t.Fatal(status)
					}
				case "unsat":
					status, err := c.Solve(0, 3)
					depthMust(t, err)
					if status != lbool.False {
						t.Fatal(status)
					}
				case "sat":
					status, err := c.Solve(0, 1)
					depthMust(t, err)
					if status != lbool.True {
						t.Fatal(status)
					}
				}
				oldLevel, err := diagnostic.ProcessVerbosityLevel()
				depthMust(t, err)
				oldStream, err := diagnostic.BorrowProcessVerboseStream()
				depthMust(t, err)
				stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
				depthMust(t, err)
				defer func() {
					depthMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
					depthMust(t, diagnostic.SetProcessVerboseStream(oldStream))
					depthMust(t, stream.Close())
				}()
				depthMust(t, diagnostic.SetProcessVerbosityLevel(level))
				depthMust(t, diagnostic.SetProcessVerboseStream(stream))
				before := m.Stats().LiveReferences
				answer, err := c.GroundSatAnswer(f.query)
				depthMust(t, err)
				expected := "Sat answer unavailable when result is false\n"
				if phase == "sat" {
					root, err := m.RefGet(answer)
					depthMust(t, err)
					if !m.IsTrue(root) {
						t.Fatal("trivial SAT answer is not true")
					}
					depthMust(t, m.DelRef(&answer))
					expected = ""
					if level != 0 {
						expected = "Warning: counterexample is trivial or non-existent\n"
					}
				} else if answer != (ast.ASTRef{}) {
					t.Fatal("non-SAT answer published")
				}
				message, err := stream.String()
				depthMust(t, err)
				if message != expected {
					t.Fatal("source answer diagnostic", message, expected)
				}
				if got := m.Stats().LiveReferences; got != before {
					t.Fatal("ground-answer temporary references", got, before)
				}
				// Wrong declarations and disposed contexts remain operational errors.
				if _, err := c.GroundSatAnswer(f.other); !errors.Is(err, ast.ErrASTDomain) {
					t.Fatal("wrong query accepted", err)
				}
				depthMust(t, c.Close())
				if _, err := c.GroundSatAnswer(f.query); !errors.Is(err, ErrContextOwner) {
					t.Fatal("closed context accepted answer", err)
				}
			})
		}
	}
}

func TestSpacerMultiplePremiseAnswerAndTraceDiagnostics(t *testing.T) {
	for _, level := range []uint32{0, 1} {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			f := newContextAnswerFixture(t, false)
			c, m := f.context, f.context.manager
			set, err := base.NewRuleSet(f.manager)
			depthMust(t, err)
			defer func() { depthMust(t, set.Close()) }()
			boolean, err := m.MkBoolSort()
			depthMust(t, err)
			var children []ast.ASTHandle
			for _, name := range []string{"multi-left", "multi-right"} {
				app, err := m.MkFreshConst(name, boolean)
				depthMust(t, err)
				decl, err := m.GetDecl(app)
				depthMust(t, err)
				depthMust(t, f.manager.RegisterPredicate(decl))
				rule, err := f.manager.NewRule(app, nil, nil, name)
				depthMust(t, err)
				depthMust(t, set.Add(rule))
				depthMust(t, rule.Close())
				children = append(children, app)
			}
			head, err := f.rule.Head()
			depthMust(t, err)
			rule, err := f.manager.NewRule(head, children, []bool{false, false}, "multiple-query")
			depthMust(t, err)
			depthMust(t, set.Add(rule))
			depthMust(t, rule.Close())
			depthMust(t, c.UpdateRules(set))
			status, err := c.Solve(0, 3)
			depthMust(t, err)
			if status != lbool.True {
				t.Fatal("multiple-premise query not reachable", status)
			}
			oldLevel, err := diagnostic.ProcessVerbosityLevel()
			depthMust(t, err)
			oldStream, err := diagnostic.BorrowProcessVerboseStream()
			depthMust(t, err)
			stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
			depthMust(t, err)
			defer func() {
				depthMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
				depthMust(t, diagnostic.SetProcessVerboseStream(oldStream))
				depthMust(t, stream.Close())
			}()
			depthMust(t, diagnostic.SetProcessVerbosityLevel(level))
			depthMust(t, diagnostic.SetProcessVerboseStream(stream))
			before := m.Stats().LiveReferences
			answer, err := c.GroundSatAnswer(f.query)
			depthMust(t, err)
			actual, err := m.RefGet(answer)
			depthMust(t, err)
			if !m.IsTrue(actual) {
				t.Fatal("multiple-premise source fallback is not true")
			}
			depthMust(t, m.DelRef(&answer))
			trace, err := c.RulesAlongTrace()
			depthMust(t, err)
			if len(trace) != 1 {
				t.Fatal("source query-wrapper trace did not stop", len(trace))
			}
			name, err := trace[0].Name()
			depthMust(t, err)
			if name != "multiple-query" {
				t.Fatal("wrong query trace rule", name)
			}
			depthMust(t, trace[0].Close())
			if got := m.Stats().LiveReferences; got != before {
				t.Fatal("multiple-premise answer/trace roots", got, before)
			}
			message, err := stream.String()
			depthMust(t, err)
			expected := ""
			if level != 0 {
				expected = "Warning: counterexample is trivial or non-existent\nWarning: counterexample is trivial or non-existent\n"
			}
			if message != expected {
				t.Fatal("source multiple-premise diagnostics", message, expected)
			}
		})
	}
}
