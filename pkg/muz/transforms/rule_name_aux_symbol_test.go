package transforms_test

// Source auxiliary transforms pass rule::name() to each replacement rule,
// including symbolic-magic seeds and loop-counter restoration.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func auxiliaryRuleName(t *testing.T, m *ast.ASTManager, kind string) symbol.Symbol {
	t.Helper()
	switch kind {
	case "null":
		return symbol.Symbol{}
	case "numerical":
		return symbol.Numerical(7)
	case "empty", "spelling":
		text := ""
		if kind == "spelling" {
			text = "k!7"
		}
		name, err := m.Symbols.Intern(text)
		coneMust(t, err)
		return name
	default:
		t.Fatal("unknown symbol fixture", kind)
		return symbol.Symbol{}
	}
}

func addAuxiliaryNamedRule(t *testing.T, f *coneFixture, head ast.ASTHandle, tails []ast.ASTHandle, name symbol.Symbol) {
	t.Helper()
	rule, err := f.rules.NewRuleWithSymbol(head, tails, nil, name)
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
}

func checkAuxiliaryRuleNames(t *testing.T, set *base.RuleSet, count int, name symbol.Symbol) {
	t.Helper()
	if set == nil {
		t.Fatal("transform did not produce changed rules")
	}
	got, err := set.Len()
	coneMust(t, err)
	if got != count {
		t.Fatal("transformed rule count", got, count)
	}
	for i := 0; i < got; i++ {
		rule, err := set.Rule(i)
		coneMust(t, err)
		actual, err := rule.NameSymbol()
		coneMust(t, err)
		coneMust(t, rule.Close())
		if actual != name {
			t.Fatalf("replacement %d changed source symbol: got %v want %v", i, actual, name)
		}
	}
}

func TestAuxiliaryTransformRuleNameSymbols(t *testing.T) {
	for _, transform := range []string{"backwards", "loop-counter", "scale", "magic-symbolic", "karr"} {
		t.Run(transform, func(t *testing.T) {
			for _, kind := range []string{"null", "empty", "numerical", "spelling"} {
				t.Run(kind, func(t *testing.T) {
					f := newConeFixture(t, "P", "Q", "R")
					m := f.manager
					name := auxiliaryRuleName(t, m, kind)
					var result *base.RuleSet
					var err error
					switch transform {
					case "backwards":
						addAuxiliaryNamedRule(t, f, f.applications["P"], nil, name)
						addAuxiliaryNamedRule(t, f, f.applications["Q"], []ast.ASTHandle{f.applications["P"]}, name)
						coneMust(t, f.set.SetOutputPredicate(f.declarations["Q"]))
						plugin := transforms.NewBackwardsPlugin(m, f.rules, 1)
						t.Cleanup(func() { coneMust(t, plugin.Close()) })
						result, err = plugin.Transform(f.set)
						coneMust(t, err)
						t.Cleanup(func() { coneMust(t, result.Close()) })
						checkAuxiliaryRuleNames(t, result, 2, name)
						outputs, err := result.OutputPredicates()
						coneMust(t, err)
						if len(outputs) != 1 || outputs[0] == f.declarations["Q"] {
							t.Fatal("backwards transform omitted its fresh query")
						}
					case "loop-counter":
						addAuxiliaryNamedRule(t, f, f.applications["P"], nil, name)
						addAuxiliaryNamedRule(t, f, f.applications["P"], []ast.ASTHandle{f.applications["P"]}, name)
						coneMust(t, f.set.SetOutputPredicate(f.declarations["P"]))
						plugin := transforms.NewLoopCounterPlugin(m, f.rules, 1)
						t.Cleanup(func() { coneMust(t, plugin.Close()) })
						result, err = plugin.Transform(f.set)
						coneMust(t, err)
						t.Cleanup(func() { coneMust(t, result.Close()) })
						checkAuxiliaryRuleNames(t, result, 2, name)
						derived := plugin.FreshPredicates()[f.declarations["P"]]
						if arity, err := m.GetArity(derived); err != nil || arity != 1 {
							t.Fatal("loop-counter transform did not add an argument", arity, err)
						}
						reverted, err := plugin.Revert(result)
						coneMust(t, err)
						t.Cleanup(func() { coneMust(t, reverted.Close()) })
						checkAuxiliaryRuleNames(t, reverted, 2, name)
					case "magic-symbolic":
						truth, err := m.MkTrue()
						coneMust(t, err)
						addAuxiliaryNamedRule(t, f, f.applications["Q"], []ast.ASTHandle{f.applications["P"], f.applications["R"], truth}, name)
						coneMust(t, f.set.SetOutputPredicate(f.declarations["Q"]))
						plugin := transforms.NewMagicSymbolicPlugin(m, f.rules, 1)
						t.Cleanup(func() { coneMust(t, plugin.Close()) })
						result, err = plugin.Transform(f.set)
						coneMust(t, err)
						t.Cleanup(func() { coneMust(t, result.Close()) })
						// The answer, output-query fact and both prefix queries all
						// inherit the original label, including a non-null empty one.
						checkAuxiliaryRuleNames(t, result, 4, name)
					case "scale":
						boolean, err := m.MkBoolSort()
						coneMust(t, err)
						two, err := m.ArithmeticRealNumeralText("2")
						coneMust(t, err)
						realSort, err := m.GetSort(two)
						coneMust(t, err)
						predicate := separateDeclaration(t, f, "named-scale", []ast.ASTHandle{realSort}, boolean)
						head, err := m.MkApp(predicate, []ast.ASTHandle{two})
						coneMust(t, err)
						addAuxiliaryNamedRule(t, f, head, nil, name)
						plugin := transforms.NewScalePlugin(m, f.rules, 1)
						t.Cleanup(func() { coneMust(t, plugin.Close()) })
						result, err = plugin.Transform(f.set)
						coneMust(t, err)
						t.Cleanup(func() { coneMust(t, result.Close()) })
						checkAuxiliaryRuleNames(t, result, 1, name)
						scaled, err := result.Rule(0)
						coneMust(t, err)
						tails, err := scaled.TailSize()
						coneMust(t, err)
						coneMust(t, scaled.Close())
						if tails != 2 {
							t.Fatal("scale transform omitted equality/positivity constraints", tails)
						}
					case "karr":
						predicate := auxiliaryNamedKarrProgram(t, f, name)
						plugin := transforms.NewKarrInvariantsPlugin(m, f.rules, 1)
						t.Cleanup(func() { coneMust(t, plugin.Close()) })
						result, err = plugin.Transform(f.set)
						coneMust(t, err)
						t.Cleanup(func() { coneMust(t, result.Close()) })
						checkAuxiliaryRuleNames(t, result, 2, name)
						step, err := result.Rule(1)
						coneMust(t, err)
						tails, err := step.TailSize()
						coneMust(t, err)
						head, err := step.HeadDeclaration()
						coneMust(t, err)
						coneMust(t, step.Close())
						if tails <= 3 || head != predicate {
							t.Fatal("Karr did not append invariants and restore the original head", tails, head)
						}
					}
					count, err := f.set.Len()
					coneMust(t, err)
					checkAuxiliaryRuleNames(t, f.set, count, name)
				})
			}
		})
	}
}

func auxiliaryNamedKarrProgram(t *testing.T, f *coneFixture, name symbol.Symbol) ast.ASTHandle {
	t.Helper()
	m := f.manager
	makeAST := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		coneMust(t, err)
		ref, err := m.NewRef(h)
		coneMust(t, err)
		t.Cleanup(func() { coneMust(t, m.DelRef(&ref)) })
		return h
	}
	boolean := makeAST(m.MkBoolSort())
	zero := makeAST(m.ArithmeticIntegerNumeralText("0"))
	one := makeAST(m.ArithmeticIntegerNumeralText("1"))
	two := makeAST(m.ArithmeticIntegerNumeralText("2"))
	integer, err := m.GetSort(zero)
	coneMust(t, err)
	predicate := separateDeclaration(t, f, "named-karr", []ast.ASTHandle{integer, integer}, boolean)
	fact := makeAST(m.MkApp(predicate, []ast.ASTHandle{zero, two}))
	addAuxiliaryNamedRule(t, f, fact, nil, name)
	variables := make([]ast.ASTHandle, 4)
	for i := range variables {
		variables[i] = makeAST(m.MkVar(uint32(i), integer))
	}
	tail := makeAST(m.MkApp(predicate, variables[:2]))
	head := makeAST(m.MkApp(predicate, variables[2:]))
	xSuccessor := makeAST(m.MkArithmeticAdd(variables[0], one))
	xStep := makeAST(m.MkEq(variables[2], xSuccessor))
	ySuccessor := makeAST(m.MkArithmeticAdd(variables[1], two))
	yStep := makeAST(m.MkEq(variables[3], ySuccessor))
	addAuxiliaryNamedRule(t, f, head, []ast.ASTHandle{tail, xStep, yStep}, name)
	coneMust(t, f.set.SetOutputPredicate(predicate))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("named Karr fixture is not stratified")
	}
	return predicate
}
