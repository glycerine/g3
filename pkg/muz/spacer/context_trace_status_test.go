package spacer

// Source: src/muz/spacer/spacer_context.cpp:2330-2337.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func TestSpacerTraceStatusPolicyAndOwnership(t *testing.T) {
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
						t.Fatal("expected bounded unknown", status)
					}
				case "unsat":
					status, err := c.Solve(0, 3)
					depthMust(t, err)
					if status != lbool.False {
						t.Fatal("expected unreachable", status)
					}
				case "sat":
					status, err := c.Solve(0, 1)
					depthMust(t, err)
					if status != lbool.True {
						t.Fatal("expected reachable", status)
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
				if phase != "sat" && level == 0 {
					m.Budget().FailAfter(0)
				}
				trace, cause := c.RulesAlongTrace()
				m.Budget().FailAfter(-1)
				depthMust(t, cause)
				expected := ""
				if phase != "sat" {
					if len(trace) != 0 {
						t.Fatal("non-SAT trace was published", len(trace))
					}
					if level != 0 {
						expected = "Trace unavailable when result is false\n"
					}
				} else {
					if len(trace) != 1 {
						t.Fatal("fact trace size", len(trace))
					}
					if level != 0 {
						expected = "Warning: counterexample is trivial or non-existent\n"
					}
				}
				message, err := stream.String()
				depthMust(t, err)
				if message != expected {
					t.Fatal("source trace diagnostic", message, expected)
				}
				if got := m.Stats().LiveReferences; got != before {
					t.Fatal("trace temporary roots", got, before)
				}
				depthMust(t, c.Close())
				// Returned SAT rules remain owned independently of the context.
				for _, rule := range trace {
					name, err := rule.Name()
					depthMust(t, err)
					if name != "answer-rule" {
						t.Fatal("trace rule changed", name)
					}
					depthMust(t, rule.Close())
				}
				if _, err := c.RulesAlongTrace(); !errors.Is(err, ErrContextOwner) {
					t.Fatal("closed context accepted trace access", err)
				}
			})
		}
	}
}

func TestSpacerUnconfiguredContextHasEmptyTrace(t *testing.T) {
	m, globals := newSpacerTestServices(t)
	c, err := NewContext(m, nil, 1, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	depthMust(t, err)
	defer func() { depthMust(t, c.Close()) }()
	before := m.Stats().LiveReferences
	trace, err := c.RulesAlongTrace()
	depthMust(t, err)
	if len(trace) != 0 || m.Stats().LiveReferences != before {
		t.Fatal("unconfigured context published or allocated a trace")
	}
}
