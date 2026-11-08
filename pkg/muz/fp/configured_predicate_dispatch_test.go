package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestConfiguredDirectPredicateQueryDispatch(t *testing.T) {
	for _, backend := range []string{"pdr", "spacer", "duality", "datalog"} {
		for _, list := range []bool{false, true} {
			name := backend + "/single"
			if list {
				name = backend + "/list"
			}
			t.Run(name, func(t *testing.T) {
				f := newDirectFactFixture(t)
				e := f.engine
				must := func(err error) { t.Helper(); fragmentMust(t, err) }
				must(e.globals.SourceSetName("fixedpoint.engine", backend))
				must(e.addRuleEager(f.applications["p"], "configured-predicate-fact"))
				query := func() (lbool.LBool, error) {
					if list {
						return e.QueryPredicates([]ast.ASTHandle{f.declarations["p"]})
					}
					return e.QueryPredicate(f.declarations["p"])
				}
				status, err := query()
				must(err)
				if status != lbool.True {
					t.Fatal("configured fact query", status)
				}
				selected, err := e.SelectedEngine()
				must(err)
				if selected.String() != backend {
					t.Fatal("configured predicate backend", selected, backend)
				}
				if (backend == "duality") != e.rawRules[0].pending {
					t.Fatal("wrong configured raw-rule phase", backend, e.rawRules[0].pending)
				}
				must(e.globals.SourceSetName("fixedpoint.engine", "bmc"))
				status, err = query()
				must(err)
				if status != lbool.True {
					t.Fatal("repeat predicate query", status)
				}
				selected, err = e.SelectedEngine()
				must(err)
				if selected.String() != backend {
					t.Fatal("predicate engine choice not cached", selected)
				}
			})
		}
	}
}

func TestDirectPredicateQueryAutomaticallySelectsNumericProgram(t *testing.T) {
	for _, list := range []bool{false, true} {
		name := "single"
		if list {
			name = "list"
		}
		t.Run(name, func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			zero, err := m.ArithmeticIntegerNumeralText("0")
			must(err)
			integer, err := m.GetSort(zero)
			must(err)
			boolean, err := m.MkBoolSort()
			must(err)
			name, err := m.Symbols.Intern("direct-query-numeric")
			must(err)
			p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
			must(err)
			must(e.RegisterPredicate(p))
			fact, err := m.MkApp(p, []ast.ASTHandle{zero})
			must(err)
			must(e.addRuleEager(fact, "numeric-fact"))
			var status lbool.LBool
			if list {
				status, err = e.QueryPredicates([]ast.ASTHandle{p})
			} else {
				status, err = e.QueryPredicate(p)
			}
			must(err)
			if status != lbool.True {
				t.Fatal("numeric fact query", status)
			}
			selected, err := e.SelectedEngine()
			must(err)
			if selected != EnginePDR {
				t.Fatal("automatic numeric engine", selected)
			}
		})
	}
}

func TestConfiguredMultiplePredicateUnsupportedGuard(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.engine", "pdr"))
	fragmentMust(t, e.addRuleEager(f.applications["p"], "fact"))
	status, err := e.QueryPredicates([]ast.ASTHandle{f.declarations["p"], f.declarations["q"]})
	fragmentMust(t, err)
	if status != lbool.Undef {
		t.Fatal("configured PDR incorrectly evaluated a multiple-relation request", status)
	}
}

func TestConfiguredPredicateQueryFromLevelDispatch(t *testing.T) {
	f := newDirectFactFixture(t)
	e := f.engine
	fragmentMust(t, e.globals.SourceSetName("fixedpoint.engine", "spacer"))
	fragmentMust(t, e.addRuleEager(f.applications["p"], "fact"))
	status, err := e.QueryPredicateFromLevel(f.declarations["p"], 0)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("configured Spacer predicate from level", status)
	}
}
