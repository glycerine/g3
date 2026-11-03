package base_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type ruleNameSymbolCase struct {
	label        string
	symbol       usymbol.Symbol
	legacy, text string
}

func ruleNameSymbolCases(t *testing.T, m *ast.ASTManager) []ruleNameSymbolCase {
	t.Helper()
	empty, err := m.Symbols.Intern("")
	ruleMust(t, err)
	spelling, err := m.Symbols.Intern("k!7")
	ruleMust(t, err)
	cases := []ruleNameSymbolCase{
		{"null", usymbol.Symbol{}, "", "<null>"},
		{"empty", empty, "", ""},
		{"numeric", usymbol.Numerical(7), "k!7", "k!7"},
		{"numeric-spelling", spelling, "k!7", "k!7"},
	}
	for i := range cases {
		for j := 0; j < i; j++ {
			if cases[i].symbol == cases[j].symbol {
				t.Fatal("fixture collapsed distinct source symbols", cases[i].label, cases[j].label)
			}
		}
	}
	return cases
}

func holdRuleNameAST(t *testing.T, m *ast.ASTManager, h ast.ASTHandle) {
	t.Helper()
	ref, err := m.NewRef(h)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, m.DelRef(&ref)) })
}

func checkRuleNameSymbol(t *testing.T, rule *base.Rule, want ruleNameSymbolCase) {
	t.Helper()
	symbol, err := rule.NameSymbol()
	ruleMust(t, err)
	legacy, err := rule.Name()
	ruleMust(t, err)
	text, err := rule.NameText()
	ruleMust(t, err)
	if symbol != want.symbol || legacy != want.legacy || text != want.text {
		t.Fatalf("name %s: symbol=%v legacy=%q text=%q", want.label, symbol, legacy, text)
	}
}

func TestRuleNameSymbolsPreserveIdentityAndSourceDisplay(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	holdRuleNameAST(t, f.manager, f.app["P"])
	for _, tc := range ruleNameSymbolCases(t, f.manager) {
		t.Run(tc.label, func(t *testing.T) {
			rule, err := f.rules.NewRuleWithSymbol(f.app["P"], nil, nil, tc.symbol)
			ruleMust(t, err)
			defer func() { ruleMust(t, rule.Close()) }()
			checkRuleNameSymbol(t, rule, tc)
			var out bytes.Buffer
			ruleMust(t, f.rules.WriteRuleDisplay(&out, rule))
			if !strings.HasPrefix(out.String(), tc.text+":\n") {
				t.Fatalf("source diagnostic name: %q", out.String())
			}
			formula, err := f.rules.NewRuleFromFormulaWithSymbol(f.app["P"], tc.symbol)
			ruleMust(t, err)
			defer func() { ruleMust(t, formula.Close()) }()
			checkRuleNameSymbol(t, formula, tc)
		})
	}
	legacy, err := f.rules.NewRule(f.app["P"], nil, nil, "")
	ruleMust(t, err)
	defer func() { ruleMust(t, legacy.Close()) }()
	checkRuleNameSymbol(t, legacy, ruleNameSymbolCase{"legacy-empty", usymbol.Symbol{}, "", "<null>"})
}

func TestRuleNameSymbolsSurviveCopiesAndActualSubstitution(t *testing.T) {
	f := newRuleSetFixture(t, "replacement")
	m := f.manager
	holdRuleNameAST(t, m, f.app["replacement"])
	boolean, err := m.MkBoolSort()
	ruleMust(t, err)
	declarationName, err := m.Symbols.Intern("rule-symbol-substitute")
	ruleMust(t, err)
	declaration, err := m.MkFuncDecl(declarationName, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{})
	ruleMust(t, err)
	ruleMust(t, f.rules.RegisterPredicate(declaration))
	variable, err := m.MkVar(0, boolean)
	ruleMust(t, err)
	head, err := m.MkApp(declaration, []ast.ASTHandle{variable})
	ruleMust(t, err)
	holdRuleNameAST(t, m, head)
	truth, err := m.MkTrue()
	ruleMust(t, err)
	holdRuleNameAST(t, m, truth)
	cases := ruleNameSymbolCases(t, m)
	for i, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			original, err := f.rules.NewUnnormalizedRuleWithSymbol(head, nil, nil, tc.symbol)
			ruleMust(t, err)
			defer func() { ruleMust(t, original.Close()) }()
			checkRuleNameSymbol(t, original, tc)
			clone, err := f.rules.CloneRule(original)
			ruleMust(t, err)
			defer func() { ruleMust(t, clone.Close()) }()
			replaced, err := f.rules.ReplaceRuleHeadNamed(original, f.app["replacement"])
			ruleMust(t, err)
			defer func() { ruleMust(t, replaced.Close()) }()
			substituted, err := f.rules.SubstituteRule(original, []ast.ASTHandle{truth})
			ruleMust(t, err)
			defer func() { ruleMust(t, substituted.Close()) }()
			for _, rule := range []*base.Rule{clone, replaced, substituted} {
				checkRuleNameSymbol(t, rule, tc)
			}
			substitutedHead, err := substituted.Head()
			ruleMust(t, err)
			argument, err := m.GetArg(substitutedHead, 0)
			ruleMust(t, err)
			if argument != truth || substitutedHead == head {
				t.Fatal("name-preservation test did not perform substitution")
			}
			replacementName := cases[(i+1)%len(cases)]
			renamed, err := f.rules.CopyRuleWithSymbol(original, replacementName.symbol)
			ruleMust(t, err)
			defer func() { ruleMust(t, renamed.Close()) }()
			renamedHead, err := f.rules.ReplaceRuleHeadWithSymbol(original, f.app["replacement"], replacementName.symbol)
			ruleMust(t, err)
			defer func() { ruleMust(t, renamedHead.Close()) }()
			checkRuleNameSymbol(t, renamed, replacementName)
			checkRuleNameSymbol(t, renamedHead, replacementName)
			ruleMust(t, original.Close())
			checkRuleNameSymbol(t, clone, tc)
			checkRuleNameSymbol(t, substituted, tc)
		})
	}
}

func TestRuleNameSymbolsPreserveFormulaProofAndUnboundReduction(t *testing.T) {
	f := newUnboundFixture(t)
	m := f.manager
	arithmetic, err := m.ArithmeticFamilyID()
	ruleMust(t, err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	ruleMust(t, err)
	x, err := m.MkVar(0, integer)
	ruleMust(t, err)
	y, err := m.MkVar(1, integer)
	ruleMust(t, err)
	head := f.predicate(t, "rule-symbol-reduce-head", []ast.ASTHandle{integer}, []ast.ASTHandle{x})
	tail := f.predicate(t, "rule-symbol-reduce-tail", []ast.ASTHandle{integer}, []ast.ASTHandle{x})
	three, err := m.ArithmeticIntegerNumeralText("3")
	ruleMust(t, err)
	equality, err := m.MkEq(y, three)
	ruleMust(t, err)
	for _, h := range []ast.ASTHandle{head, tail, equality} {
		holdRuleNameAST(t, m, h)
	}
	ruleMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	proof, err := m.MkProofAsserted(head)
	ruleMust(t, err)
	holdRuleNameAST(t, m, proof)
	ruleMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	for _, tc := range ruleNameSymbolCases(t, m) {
		t.Run(tc.label, func(t *testing.T) {
			proved, err := f.rules.NewRuleFromFormulaWithProofAndSymbol(head, proof, tc.symbol)
			ruleMust(t, err)
			defer func() { ruleMust(t, proved.Close()) }()
			checkRuleNameSymbol(t, proved, tc)
			carried, found, err := proved.Proof()
			ruleMust(t, err)
			if !found || carried.IsNull() {
				t.Fatal("typed formula construction lost proof")
			}
			original, err := f.rules.NewRuleWithSymbol(head, []ast.ASTHandle{tail, equality}, nil, tc.symbol)
			ruleMust(t, err)
			defer func() { ruleMust(t, original.Close()) }()
			reduced, changed, err := f.rules.ReduceUnboundVariables(original, f.lite)
			ruleMust(t, err)
			defer func() { ruleMust(t, reduced.Close()) }()
			if !changed {
				t.Fatal("name-preservation test did not reduce the unbound equality")
			}
			if size, err := reduced.TailSize(); err != nil || size != 1 {
				t.Fatal("reduced tail", size, err)
			}
			checkRuleNameSymbol(t, reduced, tc)
		})
	}
}

func TestRuleNameSymbolsRejectForeignAndMarkedOwnersWithoutMutation(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	other := newRuleSetFixture(t, "foreign")
	holdRuleNameAST(t, f.manager, f.app["P"])
	foreign, err := other.manager.Symbols.Intern("foreign-rule-name")
	ruleMust(t, err)
	local, err := f.manager.Symbols.Intern("local-rule-name")
	ruleMust(t, err)
	marked, err := local.Mark()
	ruleMust(t, err)
	original, err := f.rules.NewRuleWithSymbol(f.app["P"], nil, nil, local)
	ruleMust(t, err)
	defer func() { ruleMust(t, original.Close()) }()
	for _, name := range []usymbol.Symbol{foreign, marked} {
		before := f.manager.Stats()
		operations := []func() (*base.Rule, error){
			func() (*base.Rule, error) { return f.rules.NewRuleWithSymbol(f.app["P"], nil, nil, name) },
			func() (*base.Rule, error) { return f.rules.NewUnnormalizedRuleWithSymbol(f.app["P"], nil, nil, name) },
			func() (*base.Rule, error) { return f.rules.CopyRuleWithSymbol(original, name) },
			func() (*base.Rule, error) { return f.rules.ReplaceRuleHeadWithSymbol(original, f.app["P"], name) },
			func() (*base.Rule, error) { return f.rules.NewRuleFromFormulaWithSymbol(f.app["P"], name) },
			func() (*base.Rule, error) {
				return f.rules.NewRuleFromFormulaWithProofAndSymbol(f.app["P"], ast.ASTHandle{}, name)
			},
		}
		for _, operation := range operations {
			rule, err := operation()
			if rule != nil {
				ruleMust(t, rule.Close())
			}
			if err == nil {
				t.Fatal("invalid symbol owner accepted")
			}
		}
		after := f.manager.Stats()
		if before.LiveNodes != after.LiveNodes || before.LiveReferences != after.LiveReferences {
			t.Fatal("rejected names changed AST ownership", before, after)
		}
		checkRuleNameSymbol(t, original, ruleNameSymbolCase{"unchanged", local, "local-rule-name", "local-rule-name"})
	}
}
