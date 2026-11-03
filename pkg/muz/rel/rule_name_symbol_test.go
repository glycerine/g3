package rel

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRelationalRuleConsumersPreserveNameSymbols(t *testing.T) {
	for _, label := range []string{"null", "empty", "numeric", "numeric-spelling"} {
		t.Run(label, func(t *testing.T) {
			m := astTestManager(t)
			var name symbol.Symbol
			var err error
			switch label {
			case "empty":
				name, err = m.Symbols.Intern("")
			case "numeric":
				name = symbol.Numerical(7)
			case "numeric-spelling":
				name, err = m.Symbols.Intern("k!7")
			}
			astTestMust(t, err)
			family, err := m.RegisterArithmeticPlugin()
			astTestMust(t, err)
			integer, err := m.MkFamilySort(family, 1, nil)
			astTestMust(t, err)
			boolean, err := m.MkBoolSort()
			astTestMust(t, err)
			rules, err := base.NewRuleManager(m)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, rules.Close()) })
			declare := func(text string) ast.ASTHandle {
				d, err := m.MkFuncDecl(astTestSymbol(t, m, text), []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
				astTestMust(t, err)
				astTestMust(t, rules.RegisterPredicate(d))
				return d
			}
			predicate, auxiliary := declare("symbol-source"), declare("symbol-auxiliary")
			makeRule := func(value string) *base.Rule {
				constant, err := m.ArithmeticIntegerNumeralText(value)
				astTestMust(t, err)
				head, err := m.MkApp(predicate, []ast.ASTHandle{constant})
				astTestMust(t, err)
				rule, err := rules.NewRuleWithSymbol(head, nil, nil, name)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, rule.Close()) })
				return rule
			}
			first, second := makeRule("1"), makeRule("2")
			positions, err := similarityVaryingConstants([]*base.Rule{first, second})
			astTestMust(t, err)
			rewritten, err := similarityRewriteRule(rules, first, positions, auxiliary)
			astTestMust(t, err)
			defer func() { astTestMust(t, rewritten.Close()) }()
			got, err := rewritten.NameSymbol()
			astTestMust(t, err)
			if got != name {
				t.Fatalf("similarity name %v, want %v", got, name)
			}
			head, err := rewritten.Head()
			astTestMust(t, err)
			argument, err := m.GetArg(head, 0)
			astTestMust(t, err)
			kind, err := m.GetKind(argument)
			astTestMust(t, err)
			if kind != ast.ASTVar {
				t.Fatal("similarity did not replace the varying constant")
			}
			transformer, err := NewExplanationTransformer(m, rules)
			astTestMust(t, err)
			defer func() { astTestMust(t, transformer.Close()) }()
			explained, err := transformer.ExplanationRule(first)
			astTestMust(t, err)
			defer func() { astTestMust(t, explained.Close()) }()
			equality, _, err := explained.Tail(0)
			astTestMust(t, err)
			record, err := m.GetArg(equality, 1)
			astTestMust(t, err)
			recordDecl, err := m.GetDecl(record)
			astTestMust(t, err)
			recordName, err := m.GetASTName(recordDecl)
			astTestMust(t, err)
			want := name
			if name == (symbol.Symbol{}) {
				var display bytes.Buffer
				astTestMust(t, rules.WriteRuleDisplay(&display, first))
				want, err = m.Symbols.Intern(strings.TrimRight(display.String(), "\n"))
				astTestMust(t, err)
			}
			if recordName != want {
				t.Fatalf("explanation name %v, want exact rule symbol %v", recordName, want)
			}
			outputName, err := explained.NameSymbol()
			astTestMust(t, err)
			if outputName != (symbol.Symbol{}) {
				t.Fatal("source explanation rule should be unnamed", outputName)
			}
		})
	}
}
