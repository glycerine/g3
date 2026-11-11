package transforms_test

// The three source array transforms pass r.name() to their replacement rules.
// Equal displayed spellings must not collapse distinct symbol kinds.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestArrayTransformsPreserveRuleNameSymbols(t *testing.T) {
	for _, transform := range []string{"blast", "equalities", "instantiation"} {
		for _, kind := range []string{"null", "empty", "numerical", "numerical-spelling"} {
			t.Run(transform+"/"+kind, func(t *testing.T) {
				f := newConeFixture(t, "Goal")
				m := f.manager
				empty, err := m.Symbols.Intern("")
				coneMust(t, err)
				spelling, err := m.Symbols.Intern("k!7")
				coneMust(t, err)
				names := map[string]usymbol.Symbol{
					"null": {}, "empty": empty,
					"numerical": usymbol.Numerical(7), "numerical-spelling": spelling,
				}
				if names["null"] == empty || names["numerical"] == spelling {
					t.Fatal("fixture collapsed distinct symbol kinds")
				}
				want := names[kind]
				boolean, err := m.MkBoolSort()
				coneMust(t, err)
				arraySort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{boolean}), boolean)
				coneMust(t, err)
				left, err := m.MkVar(0, arraySort)
				coneMust(t, err)
				right, err := m.MkVar(1, arraySort)
				coneMust(t, err)
				truth, err := m.MkTrue()
				coneMust(t, err)
				falsity, err := m.MkFalse()
				coneMust(t, err)
				family, err := m.ArrayFamilyID()
				coneMust(t, err)
				stored, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{right, truth, falsity})
				coneMust(t, err)
				selected, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{left, truth})
				coneMust(t, err)
				definition, err := m.MkEq(left, stored)
				coneMust(t, err)
				observation, err := m.MkEq(selected, falsity)
				coneMust(t, err)
				head := f.applications["Goal"]
				tails := []ast.ASTHandle{definition, observation}
				var predicate transformPredicate
				if transform == "instantiation" {
					predicate = declareTransformPredicate(t, f, "array-name-source", []ast.ASTHandle{arraySort}, []ast.ASTHandle{left})
					head = predicate.application
					tails = []ast.ASTHandle{predicate.application, observation}
				}
				original, err := f.rules.NewRuleWithSymbol(head, tails, nil, want)
				coneMust(t, err)
				t.Cleanup(func() { coneMust(t, original.Close()) })
				coneMust(t, f.set.Add(original))
				var plugin base.RuleTransformPlugin
				switch transform {
				case "blast":
					plugin, err = transforms.NewArrayBlastPlugin(m, f.rules, nil, f.globals.Context, f.globals, 35999)
				case "equalities":
					plugin, err = transforms.NewArrayEqRewritePlugin(m, f.rules, f.globals.Context, f.globals, 34998)
				case "instantiation":
					plugin = transforms.NewArrayInstantiationPlugin(m, f.rules, 1, false, 34999)
				}
				coneMust(t, err)
				t.Cleanup(func() { coneMust(t, plugin.Close()) })
				result, err := plugin.Transform(f.set)
				coneMust(t, err)
				if result == nil {
					t.Fatal("array transform did not rebuild the rule")
				}
				t.Cleanup(func() { coneMust(t, result.Close()) })
				if count, e := result.Len(); e != nil || count != 1 {
					t.Fatalf("transformed rule count %d: %v", count, e)
				}
				replacement, err := result.Rule(0)
				coneMust(t, err)
				t.Cleanup(func() { coneMust(t, replacement.Close()) })
				for _, rule := range []*base.Rule{original, replacement} {
					got, e := rule.NameSymbol()
					coneMust(t, e)
					if got != want {
						t.Fatalf("rule name symbol %v, want %v", got, want)
					}
				}
				originalTails, err := original.TailSize()
				coneMust(t, err)
				if originalTails != 2 {
					t.Fatal("transform mutated the source rule", originalTails)
				}
				switch transform {
				case "blast":
					if count, e := replacement.TailSize(); e != nil || count != 0 {
						t.Fatalf("store/select blast left %d tails: %v", count, e)
					}
				case "equalities":
					count, e := replacement.TailSize()
					coneMust(t, e)
					trueCount := 0
					for i := 0; i < count; i++ {
						tail, _, e := replacement.Tail(i)
						coneMust(t, e)
						if m.IsTrue(tail) {
							trueCount++
						}
					}
					if count != 3 || trueCount != 2 {
						t.Fatalf("rewritten equality tails=%d true=%d", count, trueCount)
					}
				case "instantiation":
					declaration, e := replacement.HeadDeclaration()
					coneMust(t, e)
					arity, e := m.GetArity(declaration)
					coneMust(t, e)
					if declaration == predicate.declaration || arity != 3 {
						t.Fatal("array predicate was not expanded", declaration, arity)
					}
				}
			})
		}
	}
}
