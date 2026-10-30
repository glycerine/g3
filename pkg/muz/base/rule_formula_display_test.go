package base_test

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRuleManagerWriteRuleFormulaAndString(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	rule, err := f.rules.NewRule(f.head, nil, nil, "formula-display")
	ruleMust(t, err)
	defer func() { ruleMust(t, rule.Close()) }()
	var out bytes.Buffer
	ruleMust(t, f.rules.WriteRuleFormula(&out, rule))
	text, err := f.rules.RuleFormulaString(rule)
	ruleMust(t, err)
	if out.String() != text || text != "(forall ((A Int)) (analysis-P A))" {
		t.Fatal("rule display", text)
	}
}

func TestRuleManagerWriteSourceRuleDisplay(t *testing.T) {
	f := newRuleAnalysisFixture(t)
	headPin, err := f.manager.NewRef(f.head)
	ruleMust(t, err)
	defer func() { ruleMust(t, f.manager.DelRef(&headPin)) }()
	for _, tc := range []struct {
		name    string
		tails   []ast.ASTHandle
		negated []bool
		want    string
	}{
		{name: "source-display", tails: []ast.ASTHandle{f.head}, want: "source-display:\nanalysis-P(#0) :- \n analysis-P(#0).\n"},
		{name: "source-display-fact", want: "source-display-fact:\nanalysis-P(#0).\n"},
		{name: "source-display-negated", tails: []ast.ASTHandle{f.head}, negated: []bool{true}, want: "source-display-negated:\nanalysis-P(#0) :- \n not analysis-P(#0).\n"},
	} {
		rule, err := f.rules.NewRule(f.head, tc.tails, tc.negated, tc.name)
		if err != nil {
			t.Fatalf("create display rule %s: %v", tc.name, err)
		}
		var out bytes.Buffer
		ruleMust(t, f.rules.WriteRuleDisplay(&out, rule))
		ruleMust(t, rule.Close())
		if got := out.String(); got != tc.want {
			t.Errorf("source rule display %s = %q, want %q", tc.name, got, tc.want)
		}
	}
	truth, err := f.manager.MkTrue()
	ruleMust(t, err)
	interpreted, err := f.rules.NewRule(f.head, []ast.ASTHandle{truth}, nil, "source-display-interpreted")
	ruleMust(t, err)
	var out bytes.Buffer
	ruleMust(t, f.rules.WriteRuleDisplay(&out, interpreted))
	ruleMust(t, interpreted.Close())
	if got, want := out.String(), "source-display-interpreted:\nanalysis-P(#0) :- \n true.\n"; got != want {
		t.Fatalf("interpreted source rule display = %q, want %q", got, want)
	}
}
