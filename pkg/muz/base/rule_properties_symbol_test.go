package base_test

// Source: src/muz/base/rule_properties.cpp rule diagnostics use rule display's
// symbol::str label and stream insertion for an uninterpreted declaration name.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRulePropertiesTypedSymbolDiagnostics(t *testing.T) {
	f := newRulePropertiesFixture(t)
	for _, name := range ruleNameSymbolCases(t, f.manager) {
		for _, violation := range []string{"negative-predicate", "numeric-function"} {
			t.Run(name.label+"/"+violation, func(t *testing.T) {
				set, err := base.NewRuleSet(f.rules)
				ruleMust(t, err)
				t.Cleanup(func() { ruleMust(t, set.Close()) })
				body := f.q
				negative := []bool{true}
				var function ast.ASTHandle
				if violation == "numeric-function" {
					boolean, err := f.manager.MkBoolSort()
					ruleMust(t, err)
					function, err = f.manager.MkFuncDecl(symbol.Numerical(7), nil, boolean, ast.ASTFuncDeclInfo{})
					ruleMust(t, err)
					body, err = f.manager.MkApp(function, nil)
					ruleMust(t, err)
					negative = nil
				}
				rule, err := f.rules.NewRuleWithSymbol(f.p, []ast.ASTHandle{body}, negative, name.symbol)
				ruleMust(t, err)
				t.Cleanup(func() { ruleMust(t, rule.Close()) })
				ruleMust(t, set.Add(rule))
				properties, err := base.NewRuleProperties(f.rules)
				ruleMust(t, err)
				t.Cleanup(func() { ruleMust(t, properties.Close()) })
				ruleMust(t, properties.Collect(set))

				kind := base.RulePropertyNegativePredicate
				wantText := "muz/base: rule contains negative predicate"
				var failure error
				if violation == "numeric-function" {
					kind = base.RulePropertyUninterpreted
					wantText = "muz/base: uninterpreted 'k!7'"
					failure = properties.CheckUninterpretedFree()
				} else {
					failure = properties.CheckNegativePredicates()
				}
				if name.text != "" {
					wantText += " in rule " + name.text
				}
				var property *base.RulePropertyError
				if !errors.As(failure, &property) || property.Kind != kind || property.RuleName != name.text {
					t.Fatalf("typed rule diagnostic = %v, want kind %d and source label %q", failure, kind, name.text)
				}
				if failure.Error() != wantText {
					t.Fatalf("diagnostic = %q, want %q", failure.Error(), wantText)
				}
				gotName, err := rule.NameSymbol()
				ruleMust(t, err)
				if gotName != name.symbol {
					t.Fatal("diagnostic changed the rule label identity", gotName, name.symbol)
				}
				if violation == "numeric-function" {
					if property.Symbol != "k!7" {
						t.Fatalf("numeric function diagnostic = %q, want source stream spelling k!7", property.Symbol)
					}
					gotFunction, err := f.manager.GetASTName(function)
					ruleMust(t, err)
					if gotFunction != symbol.Numerical(7) {
						t.Fatal("diagnostic changed the function name identity", gotFunction)
					}
				}
			})
		}
	}
}
