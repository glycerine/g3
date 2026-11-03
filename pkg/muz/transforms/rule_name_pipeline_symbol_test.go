package transforms_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func pipelineRuleLabel(t *testing.T, m *ast.ASTManager, kind string) usymbol.Symbol {
	t.Helper()
	switch kind {
	case "null":
		return usymbol.Symbol{}
	case "numeric":
		return usymbol.Numerical(7)
	case "empty":
		label, err := m.Symbols.Intern("")
		coneMust(t, err)
		return label
	default:
		label, err := m.Symbols.Intern("k!7")
		coneMust(t, err)
		return label
	}
}

func TestTransformRuleNameSymbolsFilterSimpleJoinsMagic(t *testing.T) {
	for _, stage := range []string{"filter", "simple-joins", "magic"} {
		for _, kind := range []string{"null", "empty", "numeric", "numeric-spelling"} {
			t.Run(stage+"/"+kind, func(t *testing.T) {
				f := newConeFixture(t, "A", "B", "C", "Goal")
				label := pipelineRuleLabel(t, f.manager, kind)
				head := f.applications["Goal"]
				tails := []ast.ASTHandle{f.applications["A"]}
				if stage == "simple-joins" {
					tails = append(tails, f.applications["B"], f.applications["C"])
				} else if stage == "filter" {
					boolean, err := f.manager.MkBoolSort()
					coneMust(t, err)
					x, err := f.manager.MkVar(0, boolean)
					coneMust(t, err)
					q := declareTransformPredicate(t, f, "symbol-filter-Q", []ast.ASTHandle{boolean, boolean}, []ast.ASTHandle{x, x})
					tails = []ast.ASTHandle{q.application}
				}
				original, err := f.rules.NewRuleWithSymbol(head, tails, nil, label)
				coneMust(t, err)
				defer func() { coneMust(t, original.Close()) }()
				coneMust(t, f.set.Add(original))
				goal, err := original.HeadDeclaration()
				coneMust(t, err)
				coneMust(t, f.set.SetOutputPredicate(goal))
				valid, err := f.set.Finalize()
				coneMust(t, err)
				if !valid {
					t.Fatal("invalid symbol fixture")
				}
				var result *base.RuleSet
				switch stage {
				case "filter":
					plugin := transforms.NewFilterRulesPlugin(f.manager, f.rules, 0)
					defer func() { coneMust(t, plugin.Close()) }()
					result, err = plugin.Transform(f.set)
				case "simple-joins":
					plugin := transforms.NewSimpleJoinsPlugin(f.manager, f.rules, 0)
					defer func() { coneMust(t, plugin.Close()) }()
					result, err = plugin.Transform(f.set)
				case "magic":
					plugin := transforms.NewMagicSetsPlugin(f.manager, f.rules, goal, true, 0)
					defer func() { coneMust(t, plugin.Close()) }()
					result, err = plugin.Transform(f.set)
				}
				coneMust(t, err)
				if result == nil {
					t.Fatal("fixture did not transform")
				}
				defer func() { coneMust(t, result.Close()) }()
				count, err := result.Len()
				coneMust(t, err)
				main, helpers := 0, 0
				for index := 0; index < count; index++ {
					rule, err := result.Rule(index)
					coneMust(t, err)
					decl, err := rule.HeadDeclaration()
					coneMust(t, err)
					size, err := rule.TailSize()
					coneMust(t, err)
					isMain := decl == goal
					if stage == "magic" {
						isMain = size == 2
					}
					got, err := rule.NameSymbol()
					coneMust(t, err)
					if isMain {
						main++
						if got != label {
							t.Fatalf("inherited symbol %v, want %v", got, label)
						}
					} else {
						helpers++
						if !got.IsNull() {
							t.Fatalf("generated helper inherited label %v", got)
						}
					}
					coneMust(t, rule.Close())
				}
				wantHelpers := 1
				if stage == "magic" {
					wantHelpers = 2
				}
				if main != 1 || helpers != wantHelpers {
					t.Fatalf("transformed shape main=%d helpers=%d", main, helpers)
				}
				got, err := original.NameSymbol()
				coneMust(t, err)
				if got != label {
					t.Fatal("source label changed")
				}
			})
		}
	}
}

func TestInterpTailRuleNameSymbolsFollowChangedHead(t *testing.T) {
	for _, kind := range []string{"null", "empty", "numeric", "numeric-spelling"} {
		t.Run(kind, func(t *testing.T) {
			f := newInterpTailFixture(t)
			boolean, err := f.manager.MkBoolSort()
			coneMust(t, err)
			variable, err := f.manager.MkVar(0, boolean)
			coneMust(t, err)
			truth, err := f.manager.MkTrue()
			coneMust(t, err)
			decl := f.predicate(t, "symbol-head", []ast.ASTHandle{boolean})
			head, err := f.manager.MkApp(decl, []ast.ASTHandle{variable})
			coneMust(t, err)
			condition, err := f.manager.MkEq(variable, truth)
			coneMust(t, err)
			label := pipelineRuleLabel(t, f.manager, kind)
			rule, err := f.rules.NewRuleWithSymbol(head, []ast.ASTHandle{condition}, nil, label)
			coneMust(t, err)
			defer func() { coneMust(t, rule.Close()) }()
			coneMust(t, f.set.Add(rule))
			valid, err := f.set.Finalize()
			coneMust(t, err)
			if !valid {
				t.Fatal("invalid interpreted fixture")
			}
			result, err := f.plugin.Transform(f.set)
			coneMust(t, err)
			if result == nil {
				t.Fatal("equality did not transform rule")
			}
			defer func() { coneMust(t, result.Close()) }()
			replacement, err := result.Rule(0)
			coneMust(t, err)
			defer func() { coneMust(t, replacement.Close()) }()
			got, err := replacement.NameSymbol()
			coneMust(t, err)
			if got != label {
				t.Fatalf("name %v, want %v", got, label)
			}
			updated, err := replacement.Head()
			coneMust(t, err)
			argument, err := f.manager.GetArg(updated, 0)
			coneMust(t, err)
			if argument != truth || updated == head {
				t.Fatal("fixture failed to propagate equality into head")
			}
		})
	}
}
