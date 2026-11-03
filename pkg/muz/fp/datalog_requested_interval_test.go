package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDatalogRequestedIntervalOverridesFavourite(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("requested-interval")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	kinds := []string{"interval_relation"}
	must(e.SetPredicateRepresentation(p, kinds))
	kinds[0] = "tr_sparse"
	if e.predicates[p].requestedRepresentations[0] != "interval_relation" {
		t.Fatal("requested kind aliases caller")
	}
	must(e.AddTableFact(p, []uint32{7}))
	result, err := e.QueryPredicate(p)
	must(err)
	if result != lbool.Undef || e.intervalDatalog == nil || e.boundDatalog != nil {
		t.Fatal("requested interval did not override bound favourite", result)
	}
	answer, err := e.Answer(p)
	must(err)
	must(m.DelRef(&answer))
}
