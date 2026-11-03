package ddnf_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	muzddnf "github.com/glycerine/g3/pkg/muz/ddnf"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source ddnf::imp::compile_rule1 copies r.name() while rebuilding the rule.
func TestDDNFCompilerPreservesRuleNameSymbols(t *testing.T) {
	for _, label := range []string{"null", "empty", "numeric", "numeric-spelling"} {
		t.Run(label, func(t *testing.T) {
			f := newPreprocessFixture(t)
			must := func(err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			}
			var name symbol.Symbol
			var err error
			switch label {
			case "empty":
				name, err = f.manager.Symbols.Intern("")
			case "numeric":
				name = symbol.Numerical(7)
			case "numeric-spelling":
				name, err = f.manager.Symbols.Intern("k!7")
			}
			must(err)
			one := ddnfBVNumeral(t, f.manager, 1, 4)
			equality, err := f.manager.MkEq(f.x, one)
			must(err)
			rule, err := f.rules.NewRuleWithSymbol(f.head, []ast.ASTHandle{equality}, nil, name)
			must(err)
			defer func() { must(rule.Close()) }()
			source, err := base.NewRuleSet(f.rules)
			must(err)
			defer func() { must(source.Close()) }()
			must(source.Add(rule))
			preprocessor, err := muzddnf.NewPreprocessor(f.manager, f.registry)
			must(err)
			defer func() { must(preprocessor.Close()) }()
			ok, err := preprocessor.PreprocessRules(source)
			must(err)
			if !ok {
				t.Fatal("supported equality was rejected")
			}
			compiler, err := muzddnf.NewCompiler(preprocessor, f.rules)
			must(err)
			defer func() { must(compiler.Close()) }()
			compiled, err := compiler.CompileRuleSet(source)
			must(err)
			defer func() { must(compiled.Close()) }()
			copy, err := compiled.Rule(0)
			must(err)
			defer func() { must(copy.Close()) }()
			got, err := copy.NameSymbol()
			must(err)
			if got != name {
				t.Fatalf("compiled name %v, want source symbol %v", got, name)
			}
			head, err := copy.Head()
			must(err)
			argument, err := f.manager.GetArg(head, 0)
			must(err)
			sort, err := f.manager.GetSort(argument)
			must(err)
			width, err := f.manager.BitVectorSize(sort)
			must(err)
			if width != 2 {
				t.Fatal("rule was not compiled to the DDNF domain", width)
			}
		})
	}
}
