package base_test

// Source: dl_context.cpp:1182-1201 uses symbol identity for label presence and
// freshness, converting a colliding candidate to text only when appending '!'.

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func contextSymbolRuleSet(t *testing.T, f *ruleSetFixture) *base.RuleSet {
	t.Helper()
	set, err := base.NewRuleSet(f.rules)
	ruleMust(t, err)
	t.Cleanup(func() { ruleMust(t, set.Close()) })
	return set
}

func addContextSymbolRule(t *testing.T, f *ruleSetFixture, set *base.RuleSet, head ast.ASTHandle, name symbol.Symbol) {
	t.Helper()
	rule, err := f.rules.NewRuleWithSymbol(head, nil, nil, name)
	ruleMust(t, err)
	ruleMust(t, set.Add(rule))
	ruleMust(t, rule.Close())
}

func contextSymbolRuleLines(text, prefix string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			lines = append(lines, line)
		}
	}
	return lines
}

func contextSymbolCounts(m *ast.ASTManager) [4]uint64 {
	s := m.Stats()
	return [4]uint64{uint64(s.LiveNodes), uint64(s.LiveInfos), uint64(s.LiveParameters), uint64(s.LiveReferences)}
}

func TestContextSMT2SymbolNamesPreserveIdentityAndFreshness(t *testing.T) {
	for _, extensions := range []bool{false, true} {
		for _, low := range []bool{false, true} {
			for _, variables := range []bool{false, true} {
				t.Run(fmt.Sprintf("extensions=%v/low=%v/variables=%v", extensions, low, variables), func(t *testing.T) {
					f := newRuleSetFixture(t, "P")
					set := contextSymbolRuleSet(t, f)
					empty, err := f.manager.Symbols.Intern("")
					ruleMust(t, err)
					text, err := f.manager.Symbols.Intern("k!7")
					ruleMust(t, err)
					names := []symbol.Symbol{symbol.Numerical(7), symbol.Numerical(7), text, text, empty, empty, {}}
					for _, name := range names {
						addContextSymbolRule(t, f, set, f.app["P"], name)
					}
					config := base.DefaultContextSMT2Config()
					config.FixedpointExtensions, config.LowLevel, config.DeclareVariables = extensions, low, variables
					prefix := "(assert "
					if extensions {
						prefix = "(rule "
					}
					var want []string
					for _, spelling := range []string{"k!7", "k!7!", "k!7", "k!7!!", "", "!"} {
						want = append(want, prefix+"(! ruleset-P :named "+spelling+"))")
					}
					want = append(want, prefix+"ruleset-P)")
					before := contextSymbolCounts(f.manager)
					var first string
					for repeat := 0; repeat < 2; repeat++ {
						var out bytes.Buffer
						ruleMust(t, set.WriteContextSMT2(&out, config))
						if got := contextSymbolRuleLines(out.String(), prefix); !reflect.DeepEqual(got, want) {
							t.Fatal("source symbol freshness or null/empty presence changed", got, want)
						}
						if repeat == 0 {
							first = out.String()
						} else if out.String() != first {
							t.Fatal("display changed stored label identities between calls")
						}
						if after := contextSymbolCounts(f.manager); after != before {
							t.Fatal("typed label dump leaked AST owners", before, after)
						}
					}
					for i, want := range names {
						rule, err := set.Rule(i)
						ruleMust(t, err)
						got, err := rule.NameSymbol()
						ruleMust(t, err)
						ruleMust(t, rule.Close())
						if got != want {
							t.Fatal("freshening mutated a persistent rule symbol", i, got, want)
						}
					}
					if !extensions && low && !variables {
						var out bytes.Buffer
						ruleMust(t, set.WriteSMT2(&out))
						if got := contextSymbolRuleLines(out.String(), prefix); !reflect.DeepEqual(got, want) {
							t.Fatal("explicit HORN export collapsed typed labels", got, want)
						}
					}
				})
			}
		}
	}
}

func TestContextSMT2SymbolNamesCompareDeclarationIdentity(t *testing.T) {
	for _, numericDeclaration := range []bool{false, true} {
		for _, explicit := range []bool{false, true} {
			t.Run(fmt.Sprintf("numeric-declaration=%v/explicit=%v", numericDeclaration, explicit), func(t *testing.T) {
				f := newRuleSetFixture(t)
				set := contextSymbolRuleSet(t, f)
				text, err := f.manager.Symbols.Intern("k!7")
				ruleMust(t, err)
				declName := text
				if numericDeclaration {
					declName = symbol.Numerical(7)
				}
				boolean, err := f.manager.MkBoolSort()
				ruleMust(t, err)
				declaration, err := f.manager.MkFuncDecl(declName, nil, boolean, ast.ASTFuncDeclInfo{})
				ruleMust(t, err)
				ruleMust(t, f.rules.RegisterPredicate(declaration))
				head, err := f.manager.MkApp(declaration, nil)
				ruleMust(t, err)
				addContextSymbolRule(t, f, set, head, symbol.Numerical(7))
				addContextSymbolRule(t, f, set, head, text)
				var out bytes.Buffer
				if explicit {
					ruleMust(t, set.WriteSMT2(&out))
				} else {
					config := base.DefaultContextSMT2Config()
					config.FixedpointExtensions = false
					ruleMust(t, set.WriteContextSMT2(&out, config))
				}
				spellings := []string{"k!7", "k!7!"}
				if numericDeclaration {
					spellings = []string{"k!7!", "k!7"}
				}
				body := "k!7"
				if numericDeclaration && explicit {
					// The explicit exporter uses source smt_renaming, whose
					// numeric declaration candidate appends the initial 0.
					body = "k!70"
				}
				var want []string
				for _, spelling := range spellings {
					want = append(want, "(assert (! "+body+" :named "+spelling+"))")
				}
				if got := contextSymbolRuleLines(out.String(), "(assert "); !reflect.DeepEqual(got, want) {
					t.Fatal("declaration collision compared printed text instead of symbol identity", got, want)
				}
			})
		}
	}
}

func TestContextSMT2PendingSymbolNamesAndInputGuards(t *testing.T) {
	f := newRuleSetFixture(t, "P")
	set := contextSymbolRuleSet(t, f)
	empty, err := f.manager.Symbols.Intern("")
	ruleMust(t, err)
	text, err := f.manager.Symbols.Intern("k!7")
	ruleMust(t, err)
	addContextSymbolRule(t, f, set, f.app["P"], symbol.Numerical(7))
	config := base.DefaultContextSMT2Config()
	config.PendingRules = []ast.ASTHandle{f.app["P"], f.app["P"], f.app["P"]}
	config.PendingNameSymbols = []symbol.Symbol{empty, {}, text}
	original := append([]symbol.Symbol(nil), config.PendingNameSymbols...)
	before := contextSymbolCounts(f.manager)
	var out bytes.Buffer
	ruleMust(t, set.WriteContextSMT2(&out, config))
	want := []string{"(rule (! ruleset-P :named k!7))", "(rule (! ruleset-P :named ))", "(rule ruleset-P)", "(rule (! ruleset-P :named k!7))"}
	if got := contextSymbolRuleLines(out.String(), "(rule "); !reflect.DeepEqual(got, want) || !reflect.DeepEqual(config.PendingNameSymbols, original) {
		t.Fatal("typed pending inputs lost symbol identity or source order", got, want)
	}
	other := newRuleSetFixture(t)
	foreign, err := other.manager.Symbols.Intern("foreign")
	ruleMust(t, err)
	for _, tc := range []struct {
		name   string
		legacy []string
		typed  []symbol.Symbol
		rules  []ast.ASTHandle
	}{
		{"both-present-empty", []string{}, []symbol.Symbol{}, nil},
		{"both-present", []string{""}, []symbol.Symbol{empty}, []ast.ASTHandle{f.app["P"]}},
		{"missing-name", nil, nil, []ast.ASTHandle{f.app["P"]}},
		{"typed-count", nil, []symbol.Symbol{empty}, nil},
		{"foreign-name", nil, []symbol.Symbol{foreign}, []ast.ASTHandle{f.app["P"]}},
		{"marked-name", nil, []symbol.Symbol{symbol.Dummy()}, []ast.ASTHandle{f.app["P"]}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := base.DefaultContextSMT2Config()
			invalid.PendingRules, invalid.PendingNames, invalid.PendingNameSymbols = tc.rules, tc.legacy, tc.typed
			var output bytes.Buffer
			if err := set.WriteContextSMT2(&output, invalid); err == nil || output.Len() != 0 {
				t.Fatal("invalid pending names reached source output", err, output.String())
			}
			if after := contextSymbolCounts(f.manager); after != before {
				t.Fatal("invalid typed pending input leaked AST owners", before, after)
			}
		})
	}
	legacy := base.DefaultContextSMT2Config()
	legacy.PendingRules, legacy.PendingNames = []ast.ASTHandle{f.app["P"]}, []string{""}
	out.Reset()
	ruleMust(t, set.WriteContextSMT2(&out, legacy))
	if lines := contextSymbolRuleLines(out.String(), "(rule "); len(lines) != 2 || lines[1] != "(rule ruleset-P)" {
		t.Fatal("string compatibility no longer maps empty to absent", lines)
	}
}

func TestAppendDisplayFormulasWithSymbolsPreservesPrefixAndQuerySeparation(t *testing.T) {
	f := newRuleSetFixture(t, "P", "Q")
	set := contextSymbolRuleSet(t, f)
	empty, err := f.manager.Symbols.Intern("")
	ruleMust(t, err)
	seed, err := f.manager.Symbols.Intern("prefix")
	ruleMust(t, err)
	addContextSymbolRule(t, f, set, f.app["P"], empty)
	addContextSymbolRule(t, f, set, f.app["P"], symbol.Numerical(7))
	addContextSymbolRule(t, f, set, f.app["Q"], seed)
	ruleMust(t, set.SetOutputPredicate(f.decl["Q"]))
	before := contextSymbolCounts(f.manager)
	first, err := f.manager.NewRef(f.app["P"])
	ruleMust(t, err)
	rules := []ast.ASTRef{first}
	var queries []ast.ASTRef
	names := []symbol.Symbol{seed}
	ruleMust(t, set.AppendDisplayFormulasWithSymbols(&rules, &queries, &names))
	if len(rules) != 3 || rules[0] != first || len(queries) != 1 || !reflect.DeepEqual(names, []symbol.Symbol{seed, empty, symbol.Numerical(7)}) {
		t.Fatal("typed formula append changed prefix or named an output query", len(rules), len(queries), names)
	}
	for _, refs := range [][]ast.ASTRef{rules, queries} {
		for i := range refs {
			ruleMust(t, f.manager.DelRef(&refs[i]))
		}
	}
	if after := contextSymbolCounts(f.manager); after != before {
		t.Fatal("typed appended formulas did not release their owning references", before, after)
	}
	rules, queries = nil, nil
	var legacy []string
	ruleMust(t, set.AppendDisplayFormulas(&rules, &queries, &legacy))
	if !reflect.DeepEqual(legacy, []string{"", "k!7"}) || len(rules) != 2 || len(queries) != 1 {
		t.Fatal("legacy formula append lost its compatibility view", legacy, len(rules), len(queries))
	}
	for _, refs := range [][]ast.ASTRef{rules, queries} {
		for i := range refs {
			ruleMust(t, f.manager.DelRef(&refs[i]))
		}
	}
}
