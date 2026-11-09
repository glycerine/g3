package base_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func TestRuleSetWriteSourceDisplay(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	for _, rule := range []*base.Rule{
		f.rule("P", nil, nil),
		f.rule("Q", nil, nil),
		f.rule("P", []string{"Q"}, []bool{false}),
	} {
		ruleMust(t, set.Add(rule))
		ruleMust(t, rule.Close())
	}
	ruleMust(t, set.SetOutputPredicate(f.decl["Q"]))
	ruleMust(t, set.SetOutputPredicate(f.decl["P"]))
	var out bytes.Buffer
	ruleMust(t, set.WriteDisplay(&out))
	// m_output_preds and m_head2rules are obj_hashtable/obj_map: both
	// listings follow table order, which here puts ruleset-Q first.
	want := "; rule count: 3\n; predicate count: 2\n" +
		"; output: ruleset-Q\n; output: ruleset-P\n" +
		"Q:\nruleset-Q().\n" +
		"P:\nruleset-P().\n" +
		"P:\nruleset-P() :- \n ruleset-Q().\n"
	if got := out.String(); got != want {
		t.Fatalf("rule-set display = %q, want %q", got, want)
	}
}

func TestRuleSetWriteSourceSMT2(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	for _, rule := range []*base.Rule{
		f.rule("P", nil, nil),
		f.rule("P", []string{"Q"}, []bool{false}),
	} {
		ruleMust(t, set.Add(rule))
		ruleMust(t, rule.Close())
	}
	var out bytes.Buffer
	ruleMust(t, set.WriteSMT2(&out))
	want := "(set-logic HORN)\n" +
		"(declare-fun ruleset-P () Bool)\n" +
		"(declare-fun ruleset-Q () Bool)\n" +
		"(assert (! ruleset-P :named P))\n" +
		"(assert (! (=> ruleset-Q ruleset-P) :named P!))\n"
	if got := out.String(); got != want {
		t.Fatalf("SMT2 rule-set dump = %q, want %q", got, want)
	}
}

func TestRuleSetWriteDisplayIncludesProofsOnRulesWithTails(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	ruleMust(t, f.manager.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := f.manager.MkProofAsserted(f.app["Q"])
	ruleMust(t, err)
	rule := f.rule("P", []string{"Q"}, []bool{false})
	ruleMust(t, rule.SetProof(proof))
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	ruleMust(t, set.Add(rule))
	ruleMust(t, rule.Close())
	ruleMust(t, set.SetOutputPredicate(f.decl["P"]))

	var proofText bytes.Buffer
	printer, err := ast.NewSimpleASTPrinterContext(f.manager, &proofText, &proofText)
	ruleMust(t, err)
	ruleMust(t, printer.DisplayExpr(&proofText, proof, 0))
	var out bytes.Buffer
	ruleMust(t, set.WriteDisplay(&out))
	if !strings.HasSuffix(out.String(), ".\n"+proofText.String()+"\n") {
		t.Fatalf("proof-carrying rule display omitted its proof: %q; proof %q", out.String(), proofText.String())
	}
	fact := f.rule("Q", nil, nil)
	factProof, err := f.manager.MkProofAsserted(f.app["Q"])
	ruleMust(t, err)
	ruleMust(t, fact.SetProof(factProof))
	var factOutput bytes.Buffer
	ruleMust(t, f.rules.WriteRuleDisplay(&factOutput, fact))
	ruleMust(t, fact.Close())
	if got, want := factOutput.String(), "Q:\nruleset-Q().\n"; got != want {
		t.Fatalf("fact display = %q, want source early-return output %q", got, want)
	}
}

func TestRuleSetWriteDisplayProfileAndThreshold(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	rule := f.rule("P", []string{"Q"}, []bool{false})
	ruleMust(t, set.Add(rule))
	ruleMust(t, rule.Close())
	ruleMust(t, set.SetOutputPredicate(f.decl["P"]))

	var profiled bytes.Buffer
	ruleMust(t, set.WriteDisplayWithProfile(&profiled, true, 0))
	if !strings.HasSuffix(profiled.String(), ". {instr: 0  time: 0ms}\n") {
		t.Fatalf("profiled rule output = %q", profiled.String())
	}
	var filtered bytes.Buffer
	ruleMust(t, set.WriteDisplayWithProfile(&filtered, false, 1))
	wantFiltered := "; rule count: 1\n; predicate count: 1\n; output: ruleset-P\n"
	if got := filtered.String(); got != wantFiltered {
		t.Fatalf("threshold-filtered output = %q, want %q", got, wantFiltered)
	}
	var fact bytes.Buffer
	rule = f.rule("Q", nil, nil)
	ruleMust(t, f.rules.WriteRuleDisplayWithProfile(&fact, rule, true))
	ruleMust(t, rule.Close())
	if got, want := fact.String(), "Q:\nruleset-Q().\n"; got != want {
		t.Fatalf("profiled fact output = %q, want source early-return output %q", got, want)
	}
}
