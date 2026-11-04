//go:build g3trace

package fp

// Source: pdr_dl_interface.cpp:68,92,144 reset/query traces followed by
// pdr_context.cpp:1888 context::solve's entry trace.

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type pdrTraceRefusingWriter struct{}

func (pdrTraceRefusingWriter) Write([]byte) (int, error) {
	return 0, errors.New("PDR trace stream refused")
}

func TestPDRFreshRuleTraceOrderGateAndStreamFailure(t *testing.T) {
	for _, mode := range []string{"enabled", "disabled", "refused"} {
		t.Run(mode, func(t *testing.T) {
			var output bytes.Buffer
			var writer io.Writer = &output
			if mode == "refused" {
				writer = pdrTraceRefusingWriter{}
			}
			tags := diagnostic.NewTraceTagsWithWriter(writer)
			if mode != "disabled" {
				tags.Enable("pdr")
			}
			m, g := newPDRLifecycleServicesWithTrace(t, tags)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			defer func() { fragmentMust(t, e.Close()) }()
			fragmentMust(t, e.SelectEngine("pdr"))
			boolean, err := m.MkBoolSort()
			fragmentMust(t, err)
			declare := func(name string) (ast.ASTHandle, ast.ASTHandle) {
				s, err := m.Symbols.Intern(name)
				fragmentMust(t, err)
				d, err := m.MkFuncDecl(s, nil, boolean, ast.ASTFuncDeclInfo{})
				fragmentMust(t, err)
				fragmentMust(t, e.RegisterPredicate(d))
				a, err := m.MkApp(d, nil)
				fragmentMust(t, err)
				return d, a
			}
			p, pa := declare("fresh-trace-P")
			q, _ := declare("fresh-trace-Q")
			r, _ := declare("fresh-trace-R")
			fragmentMust(t, e.AddRule(pa, "old-fact"))
			status, err := e.QueryPredicate(p)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("initial status", status)
			}
			queryTrace := ""
			if mode == "enabled" {
				queryTrace = "-------- [pdr] query src/muz/pdr/pdr_dl_interface.cpp:144 ---------\nrules:\n; rule count: 1\n; predicate count: 1\n; output: fresh-trace-P\nold-fact:\nfresh-trace-P().\n(declare-rel fresh-trace-P ())\n(query fresh-trace-P)\n------------------------------------------------\n"
			}
			postQueryTrace := queryTrace
			if mode == "enabled" {
				postQueryTrace += "-------- [pdr] init_rule src/muz/pdr/pdr_context.cpp:616 ---------\ntrue\n------------------------------------------------\n-------- [pdr] initialize src/muz/pdr/pdr_context.cpp:519 ---------\nInitial state: true\nTransition:    fresh-trace-P!0\n------------------------------------------------\n"
				postQueryTrace += "-------- [pdr] solve src/muz/pdr/pdr_context.cpp:1888 ---------\nsolve\n------------------------------------------------\n"
				queryTrace = "-------- [pdr] query src/muz/pdr/pdr_dl_interface.cpp:92 ---------\nquery: fresh-trace-P\nrules:\n; rule count: 1\n; predicate count: 1\n; output: fresh-trace-P\nold-fact:\nfresh-trace-P().\n------------------------------------------------\n" + postQueryTrace
			}
			if output.String() != queryTrace {
				t.Fatalf("initial query trace got %q; want %q", output.String(), queryTrace)
			}
			output.Reset()
			qa, err := m.MkApp(q, nil)
			fragmentMust(t, err)
			fragmentMust(t, e.AddRule(qa, "first-fresh"))
			ra, err := m.MkApp(r, nil)
			fragmentMust(t, err)
			fragmentMust(t, e.AddRule(ra, "later-fresh"))
			if mode == "disabled" {
				fragmentMust(t, e.tracePDRFreshRule(100000))
			} // Disabled operands must not run.
			status, err = e.QueryPredicate(p)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("reset status", status)
			}
			expected := ""
			if mode == "enabled" {
				expected = "-------- [pdr] check_reset src/muz/pdr/pdr_dl_interface.cpp:68 ---------\nFresh rule first-fresh:\nfresh-trace-Q().\n------------------------------------------------\n"
			}
			if mode == "enabled" {
				// Each initialization allocates the transition tag, the level-zero
				// guard, and the reachable-cache disjunction connector in that order.
				// The latter is allocated even when the cache mode does not query it.
				queryTrace = "-------- [pdr] query src/muz/pdr/pdr_dl_interface.cpp:92 ---------\nquery: fresh-trace-P\nrules:\n; rule count: 3\n; predicate count: 3\n; output: fresh-trace-P\nold-fact:\nfresh-trace-P().\nfirst-fresh:\nfresh-trace-Q().\nlater-fresh:\nfresh-trace-R().\n------------------------------------------------\n" + strings.Replace(postQueryTrace, "fresh-trace-P!0", "fresh-trace-P!3", 1)
			}
			expected += queryTrace
			if got := output.String(); got != expected {
				t.Fatalf("fresh-rule trace got %q; want %q", got, expected)
			}
			before := m.Stats().LiveReferences
			output.Reset()
			status, err = e.QueryPredicate(p)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("repeat status", status)
			}
			if output.String() != strings.Replace(queryTrace, "fresh-trace-P!3", "fresh-trace-P!6", 1) || m.Stats().LiveReferences != before {
				t.Fatal("repeat emitted trace or changed ownership", output.String())
			}
		})
	}
}
