//go:build g3trace

package fp

// Source: pdr_dl_interface.cpp:144-148 transformed rule listing/context dump.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func newPDRTransformedTraceFixture(t *testing.T, tags *diagnostic.TraceTags) (*PropositionalEngine, ast.ASTHandle) {
	m, g := newPDRLifecycleServicesWithTrace(t, tags)
	for _, key := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.slice", "fixedpoint.xform.subsumption_checker"} {
		fragmentMust(t, g.SourceSetName(key, "false"))
	}
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, e.Close()) })
	fragmentMust(t, e.SelectEngine("pdr"))
	boolean, err := m.MkBoolSort()
	fragmentMust(t, err)
	declare := func(label string, domain []ast.ASTHandle) ast.ASTHandle {
		name, err := m.Symbols.Intern(label)
		fragmentMust(t, err)
		d, err := m.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
		fragmentMust(t, err)
		fragmentMust(t, e.RegisterPredicate(d))
		return d
	}
	p, q := declare("dump-P", []ast.ASTHandle{boolean}), declare("dump-goal", nil)
	x, err := m.MkVar(0, boolean)
	fragmentMust(t, err)
	px, err := m.MkApp(p, []ast.ASTHandle{x})
	fragmentMust(t, err)
	fact, err := e.rules.NewRule(px, nil, nil, "dump-fact")
	fragmentMust(t, err)
	fragmentMust(t, e.set.Add(fact))
	fragmentMust(t, fact.Close())
	goal, err := m.MkApp(q, nil)
	fragmentMust(t, err)
	step, err := e.rules.NewRule(goal, []ast.ASTHandle{px}, nil, "dump-step")
	fragmentMust(t, err)
	fragmentMust(t, e.set.Add(step))
	fragmentMust(t, step.Close())
	return e, q
}

func TestPDRTransformedTraceUsesContextPrintProfiles(t *testing.T) {
	for _, profile := range []string{"extensions", "plain", "low", "quantified"} {
		t.Run(profile, func(t *testing.T) {
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			tags.Enable("pdr")
			e, q := newPDRTransformedTraceFixture(t, tags)
			switch profile {
			case "plain":
				fragmentMust(t, e.globals.SourceSetName("fixedpoint.print_fixedpoint_extensions", "false"))
			case "low":
				fragmentMust(t, e.globals.SourceSetName("fixedpoint.print_low_level_smt2", "true"))
			case "quantified":
				fragmentMust(t, e.globals.SourceSetName("fixedpoint.print_with_variable_declarations", "false"))
			}
			status, err := e.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("trace changed Horn status", status)
			}
			text := output.String()
			if !strings.Contains(text, "[pdr] query src/muz/pdr/pdr_dl_interface.cpp:144") || !strings.Contains(text, "rules:\n; rule count:") {
				t.Fatal("missing source trace checkpoint", text)
			}
			if profile == "plain" {
				if !strings.Contains(text, "(set-logic HORN)") || !strings.Contains(text, "(check-sat)") || strings.Contains(text, "declare-rel") {
					t.Fatal("plain profile", text)
				}
			} else {
				if !strings.Contains(text, "(declare-rel dump-P (Bool))") || !strings.Contains(text, "(query dump-goal)") || strings.Contains(text, "set-logic HORN") {
					t.Fatal("extension profile", text)
				}
				declared := strings.Contains(text, "(declare-var A Bool)")
				if declared != (profile != "quantified") {
					t.Fatal("variable declaration switch", text)
				}
			}
			// mk_query registers a fresh query!N per query, so each query
			// keeps the same number of new owners; tracing adds none.
			growth := func() int64 {
				before := e.manager.Stats().LiveReferences
				output.Reset()
				status, err := e.QueryPredicate(q)
				fragmentMust(t, err)
				if status != lbool.True || output.Len() == 0 {
					t.Fatal("reused query omitted trace", status)
				}
				return int64(e.manager.Stats().LiveReferences) - int64(before)
			}
			if first, second := growth(), growth(); first != second {
				t.Fatal("reused trace leaked owners", first, second)
			}
		})
	}
}

func TestPDRTransformedTraceStreamFailureKeepsLaterOperandErrors(t *testing.T) {
	for _, mode := range []string{"disabled", "enabled", "refused"} {
		t.Run(mode, func(t *testing.T) {
			var output bytes.Buffer
			tags := diagnostic.NewTraceTagsWithWriter(&output)
			if mode == "refused" {
				tags = diagnostic.NewTraceTagsWithWriter(pdrTraceRefusingWriter{})
			}
			e, q := newPDRTransformedTraceFixture(t, tags)
			fragmentMust(t, e.globals.SourceSetName("fixedpoint.print_fixedpoint_extensions", "false"))
			truth, err := e.manager.MkTrue()
			fragmentMust(t, err)
			fragmentMust(t, e.AssertBackground(truth))
			status, err := e.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("quiet query", status)
			}
			// Owners one more quiet query adds (its registered query!N).
			quietBefore := e.manager.Stats().LiveReferences
			status, err = e.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("second quiet query", status)
			}
			perQuery := int64(e.manager.Stats().LiveReferences) - int64(quietBefore)
			if mode != "disabled" {
				tags.Enable("pdr")
			} else {
				fragmentMust(t, e.tracePDRTransformedRules(nil))
			}
			before := e.manager.Stats().LiveReferences
			status, err = e.QueryPredicate(q)
			if mode == "disabled" {
				fragmentMust(t, err)
				if status != lbool.True {
					t.Fatal("disabled trace changed query", status)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "Background axioms cannot be used with SMT-LIB2 HORN format") {
					t.Fatal("trace stream hid the later source exception", status, err)
				}
				answer, cause := e.Answer(q)
				if answer != (ast.ASTRef{}) {
					fragmentMust(t, e.manager.DelRef(&answer))
				}
				if cause == nil {
					t.Fatal("failed trace published a query result")
				}
			}
			if delta := int64(e.manager.Stats().LiveReferences) - int64(before); delta != perQuery {
				t.Fatal("trace failure lost owners", delta, perQuery)
			}
			tags.Disable("pdr")
			status, err = e.QueryPredicate(q)
			fragmentMust(t, err)
			if status != lbool.True {
				t.Fatal("query retry", status)
			}
		})
	}
}
