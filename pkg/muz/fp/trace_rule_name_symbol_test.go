package fp

// Source: dl_context.cpp get_rules_along_trace and api_datalog_spacer.inc
// Z3_fixedpoint_get_rule_names_along_trace. The matching 4.7.1 reference emits
// <null>;k!7;k!7;;<null> for this chain with preprocessing disabled.

import (
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestFixedpointTraceRuleNameSymbolsPreserveSourceRenderingAndOwners(t *testing.T) {
	m, globals := newPDRLifecycleServices(t)
	for _, setting := range []string{"fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.slice", "fixedpoint.xform.subsumption_checker"} {
		fragmentMust(t, globals.SourceSetName(setting, "false"))
	}
	e, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	f := &fpRuleNameFixture{t: t, m: m, e: e}
	t.Cleanup(func() {
		fragmentMust(t, e.Close())
		fragmentMust(t, closeASTRefs(m, f.roots))
	})
	fragmentMust(t, e.SelectEngine("spacer"))
	names := fpRuleNameSymbols(t, m)
	_, p := f.predicate("typed-trace-p")
	_, q := f.predicate("typed-trace-q")
	_, r := f.predicate("typed-trace-r")
	_, s := f.predicate("typed-trace-s")
	clauses := []ast.ASTHandle{p, f.own(m.MkImplies(p, q)), f.own(m.MkImplies(q, r)), f.own(m.MkImplies(r, s))}
	for i, clause := range clauses {
		fragmentMust(t, e.AddRuleWithSymbol(clause, names[i]))
	}
	status, err := e.QueryExpression(s)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("chain query", status)
	}
	forms, labels, err := e.RulesAlongTraceWithSymbols()
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, closeASTRefs(m, forms)) })
	textForms, textNames, err := e.RulesAlongTrace()
	fragmentMust(t, err)
	t.Cleanup(func() { fragmentMust(t, closeASTRefs(m, textForms)) })
	wantLabels := []symbol.Symbol{names[0], names[3], names[2], names[1], names[0]}
	wantText := []string{"<null>", "k!7", "k!7", "", "<null>"}
	if !reflect.DeepEqual(labels, wantLabels) || !reflect.DeepEqual(textNames, wantText) || len(forms) != len(wantLabels) || len(textForms) != len(forms) {
		t.Fatalf("trace labels = %v / %q (%d, %d formulas), want %v / %q", labels, textNames, len(forms), len(textForms), wantLabels, wantText)
	}
	if labels[1] == labels[2] || labels[3] == labels[0] {
		t.Fatal("trace conflated numeric/text or empty/null identities")
	}
	clone, kind, err := e.expressionQueryState()
	fragmentMust(t, err)
	if kind != EngineSpacer || clone.spacer == nil {
		t.Fatal("query did not use actual Spacer trace owners", kind)
	}
	rules, err := clone.spacer.RulesAlongTrace()
	fragmentMust(t, err)
	t.Cleanup(func() {
		for _, rule := range rules {
			fragmentMust(t, rule.Close())
		}
	})
	if len(rules) != len(forms) {
		t.Fatal("actual rule trace length", len(rules), len(forms))
	}
	queryHead, err := rules[0].Head()
	fragmentMust(t, err)
	queryFormula := f.own(m.MkImplies(s, queryHead))
	wantForms := []ast.ASTHandle{queryFormula, clauses[3], clauses[2], clauses[1], clauses[0]}
	wantHeads := []ast.ASTHandle{queryHead, s, r, q, p}
	wantTails := []ast.ASTHandle{s, r, q, p}
	checkSnapshot := func(phase string) {
		t.Helper()
		if !reflect.DeepEqual(labels, wantLabels) || !reflect.DeepEqual(textNames, wantText) {
			t.Fatal(phase, "prior trace label snapshots changed")
		}
		for i, rule := range rules {
			got, err := m.RefGet(forms[i])
			fragmentMust(t, err)
			textGot, err := m.RefGet(textForms[i])
			fragmentMust(t, err)
			if got != wantForms[i] || textGot != wantForms[i] {
				t.Fatal(phase, "trace formula/order changed", i, got, textGot, wantForms[i])
			}
			label, err := rule.NameSymbol()
			fragmentMust(t, err)
			if label != wantLabels[i] {
				t.Fatal(phase, "held rule label changed", i, label, wantLabels[i])
			}
			head, err := rule.Head()
			fragmentMust(t, err)
			if head != wantHeads[i] {
				t.Fatal(phase, "held rule head changed", i)
			}
			size, err := rule.TailSize()
			fragmentMust(t, err)
			if i == len(wantTails) {
				if size != 0 {
					t.Fatal(phase, "fact acquired a body", size)
				}
				continue
			}
			if size != 1 {
				t.Fatal(phase, "chain rule body size", i, size)
			}
			tail, negative, err := rule.Tail(0)
			fragmentMust(t, err)
			if negative || tail != wantTails[i] {
				t.Fatal(phase, "held chain rule body changed", i)
			}
		}
	}
	checkSnapshot("initial query")
	status, err = e.QueryExpression(p)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("replacement query", status)
	}
	checkSnapshot("after query replacement")
	fragmentMust(t, e.Close())
	checkSnapshot("after source close")
}
