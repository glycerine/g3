package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogBoundFinitePrecedenceAndMixedGuard(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "default-finite", true: "explicit-bound"}[explicit], func(t *testing.T) {
			f := newDirectFactFixture(t)
			e := f.engine
			must := func(err error) { t.Helper(); fragmentMust(t, err) }
			must(e.SelectEngine("datalog"))
			if explicit {
				must(e.globals.SourceSetName("fixedpoint.datalog.default_relation", "bound_relation"))
			}
			if explicit { // Exercise insertion after an empty selected runtime exists.
				status, err := e.QueryPredicate(f.declarations["p"])
				must(err)
				if status != lbool.False {
					t.Fatal(status)
				}
			}
			must(e.AddTableFact(f.declarations["p"], nil))
			status, err := e.QueryExpression(f.applications["p"])
			must(err)
			want := lbool.True
			if explicit {
				want = lbool.Undef
			}
			if status != want {
				t.Fatal("nullary source representation precedence", status, want)
			}
		})
	}
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	must(e.SelectEngine("datalog"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("mixed-original-arithmetic")
	must(err)
	d, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(d))
	// relation_manager picks a plugin per signature: the finite predicates
	// keep tables and the rule-free Int predicate is an empty relation
	// (AUDIT.md muz_fp_01-F3; this formerly asserted ErrBoundDatalogFragment).
	status, err := e.QueryPredicate(d)
	must(err)
	if status != lbool.False {
		t.Fatal("mixed original finite/arithmetic program: empty Int relation", status)
	}
	if e.boundDatalog != nil {
		t.Fatal("mixed program installed whole-program bound store")
	}
}
