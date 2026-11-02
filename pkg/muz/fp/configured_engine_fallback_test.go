package fp

// Source: fixedpoint_params.pyg engine default and dl_context.cpp:801-859.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestConfiguredUnknownEngineFallsBackToSourceAutoSelection(t *testing.T) {
	for _, tc := range []struct {
		name, module, local, program string
		want                         EngineKind
	}{
		{"module-unknown", "unknown-engine", "", "boolean", EngineDatalog},
		{"local-unknown-overrides-module", "pdr", "unknown-engine", "boolean", EngineDatalog},
		{"local-known-overrides-module", "unknown-engine", "pdr", "boolean", EnginePDR},
		{"module-unknown-numeric", "unknown-engine", "", "integer", EnginePDR},
		{"local-unknown-quantified", "datalog", "unknown-engine", "quantified", EngineQPDR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.globals.SourceSetName("fixedpoint.engine", tc.module))
			setLocal := func(name string) {
				value, err := e.params.NewValue()
				must(err)
				key := "engine"
				v, err := m.Symbols.Intern(name)
				must(err)
				must(e.params.SetSymbolName(&value, &key, v))
				must(e.SetQueryParameters(&value))
				must(e.params.Del(&value))
			}
			if tc.local != "" {
				setLocal(tc.local)
			}
			query := f.declarations["p"]
			switch tc.program {
			case "boolean":
				must(e.AddRule(f.applications["p"], "fact"))
			case "integer":
				zero, err := m.ArithmeticIntegerNumeralText("0")
				must(err)
				integer, err := m.GetSort(zero)
				must(err)
				b, err := m.MkBoolSort()
				must(err)
				name, err := m.Symbols.Intern("fallback-integer")
				must(err)
				d, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, b, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(d))
				query = d
				fact, err := m.MkApp(d, []ast.ASTHandle{zero})
				must(err)
				must(e.AddRule(fact, "integer"))
			case "quantified":
				zero, err := m.ArithmeticIntegerNumeralText("0")
				must(err)
				integer, err := m.GetSort(zero)
				must(err)
				x, err := m.MkVar(0, integer)
				must(err)
				positive, err := m.MkArithmeticGT(x, zero)
				must(err)
				q, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{integer}, Names: []symbol.Symbol{symbol.Numerical(0)}, Body: positive})
				must(err)
				rule, err := m.MkImplies(q, f.applications["p"])
				must(err)
				must(e.AddRule(rule, "quantified"))
			}
			must(e.SelectConfiguredEngine())
			got, err := e.SelectedEngine()
			must(err)
			if got != tc.want {
				t.Fatal("configured fallback", got, tc.want)
			}
			if tc.program != "quantified" {
				status, err := e.QueryPredicate(query)
				must(err)
				if status != lbool.True {
					t.Fatal("selected backend did not solve fact", status)
				}
			}
			setLocal("spacer")
			must(e.SelectConfiguredEngine())
			got, err = e.SelectedEngine()
			must(err)
			if got != tc.want {
				t.Fatal("parameter update replaced cached source choice", got, tc.want)
			}
			if err := e.SelectEngine("unknown-engine"); err == nil {
				t.Fatal("explicit Go engine selector lost unknown-name guard")
			}
			got, err = e.SelectedEngine()
			must(err)
			if got != tc.want {
				t.Fatal("rejected explicit selection mutated source choice", got, tc.want)
			}
		})
	}
}
