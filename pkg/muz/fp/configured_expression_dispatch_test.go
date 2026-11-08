package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestConfiguredExpressionQueryDispatchBeforeFirstQuery(t *testing.T) {
	for _, tc := range []struct {
		backend   string
		fromLevel bool
	}{{"pdr", false}, {"spacer", false}, {"spacer", true}, {"duality", false}, {"datalog", false}} {
		name := tc.backend
		if tc.fromLevel {
			name += "/from-level"
		}
		t.Run(name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e := f.engine
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.globals.SourceSetName("fixedpoint.engine", tc.backend))
			if tc.backend == "spacer" && !tc.fromLevel {
				must(e.globals.SourceSetName("fixedpoint.engine", "pdr"))
				local, err := e.params.NewValue()
				must(err)
				key := "engine"
				spacer, err := f.manager.Symbols.Intern("spacer")
				must(err)
				must(e.params.SetSymbolName(&local, &key, spacer))
				must(e.SetQueryParameters(&local))
				must(e.params.Del(&local))
			}
			must(e.addRuleEager(f.applications["p"], "configured-expression-fact"))
			var status lbool.LBool
			var err error
			if tc.fromLevel {
				status, err = e.QueryExpressionFromLevel(f.applications["p"], 0)
			} else {
				status, err = e.QueryExpression(f.applications["p"])
			}
			must(err)
			if status != lbool.True {
				t.Fatal("fact query", status)
			}
			selected, err := e.SelectedEngine()
			must(err)
			if selected.String() != tc.backend {
				t.Fatal("configured expression backend", selected, tc.backend)
			}
			if tc.backend == "duality" && !e.rawRules[0].pending {
				t.Fatal("ordinary Duality incorrectly flushed pending raw rule")
			}
			if tc.backend != "duality" && e.rawRules[0].pending {
				t.Fatal("normalized engine did not flush pending rule")
			}
			must(e.globals.SourceSetName("fixedpoint.engine", "bmc"))
			status, err = e.QueryExpression(f.applications["p"])
			must(err)
			if status != lbool.True {
				t.Fatal("repeat configured query", status)
			}
			selected, err = e.SelectedEngine()
			must(err)
			if selected.String() != tc.backend {
				t.Fatal("configured expression choice was not cached", selected)
			}
		})
	}
}

func TestExpressionQueryAutoSelectsNumericProgram(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("configured-expression-numeric")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	fact, err := m.MkApp(p, []ast.ASTHandle{zero})
	must(err)
	must(e.addRuleEager(fact, "numeric-fact"))
	status, err := e.QueryExpression(fact)
	must(err)
	if status != lbool.True {
		t.Fatal("automatic numeric query", status)
	}
	selected, err := e.SelectedEngine()
	must(err)
	if selected != EnginePDR {
		t.Fatal("automatic numeric backend", selected)
	}
}
