//go:build g3trace

package fp

// Source: pdr_dl_interface.cpp:68 check_reset's first fresh-rule trace.

import (
	"bytes"
	"errors"
	"io"
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
			if output.Len() != 0 {
				t.Fatal("first query emitted a reset trace", output.String())
			}
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
			if output.Len() != 0 || m.Stats().LiveReferences != before {
				t.Fatal("repeat emitted trace or changed ownership", output.String())
			}
		})
	}
}
