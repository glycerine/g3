package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogArithmeticNamedUpdateRebindsProgramAndKeepsSeeds(t *testing.T) {
	for _, kind := range []string{"bound_relation", "interval_relation"} {
		t.Run(kind, func(t *testing.T) {
			m, g := newPDRLifecycleServices(t)
			e, err := NewPropositionalEngineWithServices(m, g.Context, g)
			fragmentMust(t, err)
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			defer func() { must(e.Close()) }()
			must(e.SelectEngine("datalog"))
			must(g.SourceSetName("fixedpoint.datalog.default_relation", kind))
			family, err := m.RegisterArithmeticPlugin()
			must(err)
			integer, err := m.MkFamilySort(family, 1, nil)
			must(err)
			b, err := m.MkBoolSort()
			must(err)
			declare := func(name string) ast.ASTHandle {
				sym, err := m.Symbols.Intern(name)
				must(err)
				d, err := m.MkFuncDecl(sym, []ast.ASTHandle{integer}, b, ast.ASTFuncDeclInfo{})
				must(err)
				must(e.RegisterPredicate(d))
				return d
			}
			p, q, r := declare("update-P"), declare("update-Q"), declare("update-R")
			must(e.AddTableFact(p, []uint32{1}))
			x, err := m.MkVar(0, integer)
			must(err)
			app := func(d ast.ASTHandle) ast.ASTHandle { h, err := m.MkApp(d, []ast.ASTHandle{x}); must(err); return h }
			px, qx, rx := app(p), app(q), app(r)
			rule, err := m.MkImplies(px, qx)
			must(err)
			must(e.AddRule(rule, "step"))
			status, err := e.QueryPredicate(q)
			must(err)
			if status != lbool.Undef {
				t.Fatal("initial nonempty arithmetic query", status)
			}
			prior := e.set
			body, err := m.MkAnd(px, rx)
			must(err)
			replacement, err := m.MkImplies(body, qx)
			must(err)
			must(e.UpdateRule(replacement, "step"))
			if e.set == prior {
				t.Fatal("fixture did not replace rule set")
			}
			status, err = e.QueryPredicate(q)
			must(err)
			if status != lbool.False {
				t.Fatal("named update did not use strengthened program", status)
			}
			must(e.AddTableFact(r, []uint32{1}))
			status, err = e.QueryPredicate(q)
			must(err)
			if status != lbool.Undef {
				t.Fatal("program rebind discarded prior seed facts", status)
			}
		})
	}
}
