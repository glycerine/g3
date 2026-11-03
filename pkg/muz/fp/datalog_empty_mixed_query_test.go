package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
)

func TestEmptyMixedDatalogQueryPreservesSourcePreludeAndZeroOutputs(t *testing.T) {
	f := newDirectFactFixture(t)
	e, m := f.engine, f.manager
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("empty-mixed-prelude-infinite")
	must(err)
	infinite, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(infinite))
	must(e.SelectEngine("auto-config"))

	finite := f.declarations["p"]
	must(e.relations.MarkSaturated(finite))
	m.Limit.Cancel()
	status, err := e.QueryPredicates([]ast.ASTHandle{infinite, finite})
	m.Limit.ResetCancel()
	if status != lbool.Undef || err == nil || err.Error() != messages.Canceled {
		t.Fatal("empty mixed query ignored preexisting cancellation", status, err)
	}
	saturated, err := e.relations.IsSaturated(finite)
	must(err)
	if saturated {
		t.Fatal("empty mixed query skipped saturated-mark reset")
	}
	if e.boundDatalog != nil {
		t.Fatal("canceled empty mixed query installed a bound runtime")
	}

	status, err = e.QueryPredicates(nil)
	must(err)
	if status != lbool.True {
		t.Fatal("zero-output empty query", status)
	}
	status, err = e.QueryPredicates([]ast.ASTHandle{infinite, finite})
	must(err)
	if status != lbool.False {
		t.Fatal("empty mixed query retry", status)
	}
}
