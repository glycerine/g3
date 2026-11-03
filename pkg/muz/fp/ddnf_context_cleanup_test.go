package fp

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

type ddnfDumpRefusingWriter struct {
	remaining int
	cause     error
}

func (w *ddnfDumpRefusingWriter) Write(text []byte) (int, error) {
	if len(text) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, w.cause
	}
	w.remaining -= len(text)
	return len(text), nil
}

func TestDDNFContextDumpWriterFailurePreservesRulesAndRetry(t *testing.T) {
	for _, expression := range []bool{false, true} {
		t.Run(fmt.Sprintf("expression=%v", expression), func(t *testing.T) {
			e, m := newFragmentTestEngine(t, "ddnf")
			f := &fpRuleNameFixture{t: t, e: e, m: m}
			t.Cleanup(func() { fragmentMust(t, closeASTRefs(m, f.roots)) })
			declaration, fact := f.predicate("ddnf-writer-P")
			fragmentMust(t, e.AddRule(fact, "ddnf-writer-fact"))
			fragmentMust(t, e.flushPendingRules())
			persistent := e.set
			formulas, err := e.RuleFormulas()
			fragmentMust(t, err)
			t.Cleanup(func() { fragmentMust(t, closeASTRefs(m, formulas)) })
			query := func() (lbool.LBool, error) {
				if expression {
					return e.QueryExpression(fact)
				}
				return e.QueryPredicate(declaration)
			}
			var baseline bytes.Buffer
			fragmentMust(t, e.SetDDNFOutput(&baseline))
			status, err := query()
			fragmentMust(t, err)
			if status != lbool.Undef || !bytes.Contains(baseline.Bytes(), []byte("(query ddnf-writer-P)")) {
				t.Fatal("source DDNF query dump", status, baseline.String())
			}
			fragmentMust(t, e.SelectEngine("datalog"))
			before := contextDisplayLiveCounts(m.Stats())
			original := errors.New("DDNF dump writer refused output")
			for _, limit := range []int{0, len("(declare-rel ddnf-writer-P ())\n"), baseline.Len() - 2} {
				fragmentMust(t, e.SelectEngine("ddnf"))
				fragmentMust(t, e.SetDDNFOutput(&ddnfDumpRefusingWriter{remaining: limit, cause: original}))
				status, failure := query()
				if status != lbool.Undef || !errors.Is(failure, original) {
					t.Fatal("failed dump lost unknown status or writer cause", limit, status, failure)
				}
				// Source DDNF registry caches can freeze after a completed
				// preprocessing pass. Replacing the backend releases that
				// source-lifetime state before a new query attempt.
				fragmentMust(t, e.SelectEngine("datalog"))
				if after := contextDisplayLiveCounts(m.Stats()); after != before {
					t.Fatal("failed dump leaked AST ownership", limit, before, after)
				}
				fragmentMust(t, e.SelectEngine("ddnf"))
				var retry bytes.Buffer
				fragmentMust(t, e.SetDDNFOutput(&retry))
				status, err = query()
				fragmentMust(t, err)
				if status != lbool.Undef || retry.String() != baseline.String() || e.set != persistent {
					t.Fatal("dump retry changed source output or persistent owner", status, retry.String())
				}
				fragmentMust(t, e.SelectEngine("datalog"))
				if after := contextDisplayLiveCounts(m.Stats()); after != before {
					t.Fatal("dump retry leaked AST ownership", limit, before, after)
				}
				if len(formulas) != 1 {
					t.Fatal("held original rule snapshot changed", len(formulas))
				}
				if got, err := m.RefGet(formulas[0]); err != nil || got != fact {
					t.Fatal("failed dump invalidated the original rule snapshot", got, err)
				}
			}
		})
	}
}
