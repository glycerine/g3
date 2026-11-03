package transforms_test

// Source: dl_mk_rule_inliner.cpp:118-120 deliberately renders combined names;
// dl_mk_unbound_compressor.cpp:176,228 distinguishes inherited and null names.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type resolutionNameCase struct {
	kind, text string
	name       symbol.Symbol
}

func resolutionNameCases(t *testing.T, m *ast.ASTManager) []resolutionNameCase {
	t.Helper()
	empty, err := m.Symbols.Intern("")
	coneMust(t, err)
	spelling, err := m.Symbols.Intern("k!7")
	coneMust(t, err)
	return []resolutionNameCase{
		{kind: "null", text: "<null>"},
		{kind: "empty", name: empty},
		{kind: "numerical", text: "k!7", name: symbol.Numerical(7)},
		{kind: "spelling", text: "k!7", name: spelling},
	}
}

func requireResolutionName(t *testing.T, rule *base.Rule, want symbol.Symbol) {
	t.Helper()
	got, err := rule.NameSymbol()
	coneMust(t, err)
	if got != want {
		t.Fatalf("rule name symbol: got %v, want %v", got, want)
	}
}

func TestRuleUnifierCombinesSourceSymbolText(t *testing.T) {
	kinds := []string{"null", "empty", "numerical", "spelling"}
	for left, leftKind := range kinds {
		for right, rightKind := range kinds {
			t.Run(leftKind+"/"+rightKind, func(t *testing.T) {
				f := newConeFixture(t, "P", "Q", "R")
				cases := resolutionNameCases(t, f.manager)
				targetName, sourceName := cases[left], cases[right]
				target, err := f.rules.NewRuleWithSymbol(f.applications["Q"], []ast.ASTHandle{f.applications["P"], f.applications["R"]}, nil, targetName.name)
				coneMust(t, err)
				defer func() { coneMust(t, target.Close()) }()
				source, err := f.rules.NewRuleWithSymbol(f.applications["P"], []ast.ASTHandle{f.applications["R"]}, nil, sourceName.name)
				coneMust(t, err)
				defer func() { coneMust(t, source.Close()) }()
				unifier, err := transforms.NewRuleUnifier(f.manager)
				coneMust(t, err)
				defer func() { coneMust(t, unifier.Close()) }()
				unified, err := unifier.UnifyRules(target, 0, source)
				coneMust(t, err)
				if !unified {
					t.Fatal("matching positive tail did not unify")
				}
				resolved, err := unifier.ResolveRaw(f.rules, target, 0, source)
				coneMust(t, err)
				defer func() { coneMust(t, resolved.Close()) }()
				// The source creates a new textual symbol even when either
				// input is numerical. Null and present-empty render differently.
				wantText := targetName.text + ";" + sourceName.text
				want, err := f.manager.Symbols.Intern(wantText)
				coneMust(t, err)
				requireResolutionName(t, resolved, want)
				text, err := resolved.NameText()
				coneMust(t, err)
				if text != wantText || want.IsNumerical() || want == (symbol.Symbol{}) {
					t.Fatal("resolved name is not the source-generated text", text, wantText, want)
				}
				head, err := resolved.Head()
				coneMust(t, err)
				count, err := resolved.TailSize()
				coneMust(t, err)
				tail, negative, err := resolved.Tail(0)
				coneMust(t, err)
				if head != f.applications["Q"] || count != 1 || tail != f.applications["R"] || negative {
					t.Fatal("resolution did not remove the pivot and duplicate tail")
				}
				requireResolutionName(t, target, targetName.name)
				requireResolutionName(t, source, sourceName.name)
			})
		}
	}
}

func TestUnboundCompressorSymbolNamesSeparateHeadAndTailRewrites(t *testing.T) {
	for _, negative := range []bool{false, true} {
		for kind, kindName := range []string{"null", "empty", "numerical", "spelling"} {
			prefix := "positive/"
			if negative {
				prefix = "negative/"
			}
			t.Run(prefix+kindName, func(t *testing.T) {
				f := newConeFixture(t)
				name := resolutionNameCases(t, f.manager)[kind]
				boolean, err := f.manager.MkBoolSort()
				coneMust(t, err)
				x, err := f.manager.MkVar(0, boolean)
				coneMust(t, err)
				y, err := f.manager.MkVar(1, boolean)
				coneMust(t, err)
				q := declareTransformPredicate(t, f, "symbol-compress-Q", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
				p := declareTransformPredicate(t, f, "symbol-compress-P", []ast.ASTHandle{boolean, boolean}, []ast.ASTHandle{x, y})
				goal := declareTransformPredicate(t, f, "symbol-compress-Goal", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
				definition, err := f.rules.NewRuleWithSymbol(p.application, []ast.ASTHandle{q.application}, nil, name.name)
				coneMust(t, err)
				defer func() { coneMust(t, definition.Close()) }()
				coneMust(t, f.set.Add(definition))
				tails, flags := []ast.ASTHandle{p.application}, []bool{false}
				if negative {
					e := declareTransformPredicate(t, f, "symbol-compress-E", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
					tails, flags = []ast.ASTHandle{e.application, p.application}, []bool{false, true}
				}
				consumer, err := f.rules.NewRuleWithSymbol(goal.application, tails, flags, name.name)
				coneMust(t, err)
				defer func() { coneMust(t, consumer.Close()) }()
				coneMust(t, f.set.Add(consumer))
				coneMust(t, f.set.SetOutputPredicate(goal.declaration))
				valid, err := f.set.Finalize()
				coneMust(t, err)
				if !valid {
					t.Fatal("compression fixture is not stratified")
				}
				plugin := transforms.NewUnboundCompressorPlugin(f.manager, f.rules, true, 0)
				defer func() { coneMust(t, plugin.Close()) }()
				result, err := plugin.Transform(f.set)
				coneMust(t, err)
				if result == nil {
					t.Fatal("unbound head was not compressed")
				}
				defer func() { coneMust(t, result.Close()) }()
				compressed := plugin.CompressedPredicates()[p.declaration]
				if compressed.IsNull() {
					t.Fatal("missing compressed predicate")
				}
				definitions, err := result.PredicateRules(compressed)
				coneMust(t, err)
				defer func() {
					for _, rule := range definitions {
						coneMust(t, rule.Close())
					}
				}()
				if len(definitions) != 1 {
					t.Fatal("pure head compression count", len(definitions))
				}
				requireResolutionName(t, definitions[0], name.name)
				goals, err := result.PredicateRules(goal.declaration)
				coneMust(t, err)
				defer func() {
					for _, rule := range goals {
						coneMust(t, rule.Close())
					}
				}()
				wantGoals := 2
				if negative {
					wantGoals = 1
				}
				if len(goals) != wantGoals {
					t.Fatal("source/decompression alternative count", len(goals), wantGoals)
				}
				generated := 0
				for _, rule := range goals {
					count, err := rule.TailSize()
					coneMust(t, err)
					originalTail, compressedTail := false, false
					for i := 0; i < count; i++ {
						declaration, err := rule.TailDeclaration(i)
						coneMust(t, err)
						if declaration == p.declaration || declaration == compressed {
							_, isNegative, err := rule.Tail(i)
							coneMust(t, err)
							if isNegative != negative {
								t.Fatal("compression changed the tail polarity")
							}
						}
						originalTail = originalTail || declaration == p.declaration
						compressedTail = compressedTail || declaration == compressed
					}
					if compressedTail {
						generated++
						requireResolutionName(t, rule, symbol.Symbol{})
						if originalTail != negative {
							t.Fatal("source decompression must replace a positive tail or accumulate a negative tail")
						}
					} else {
						if !originalTail {
							t.Fatal("unchanged consumer lost its original tail")
						}
						requireResolutionName(t, rule, name.name)
					}
				}
				if generated != 1 {
					t.Fatal("decompression path was not exercised", generated)
				}
				requireResolutionName(t, definition, name.name)
				requireResolutionName(t, consumer, name.name)
			})
		}
	}
}
