package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogKarrExplicitRepresentation(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	family, err := m.RegisterArithmeticPlugin()
	must(err)
	integer, err := m.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("karr-explicit-p")
	must(err)
	p, err := m.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(e.RegisterPredicate(p))
	must(e.SetPredicateRepresentation(p, []string{"karr_relation"}))
	must(e.AddTableFact(p, []uint32{3}))
	status, err := e.QueryPredicate(p)
	must(err)
	// The requested karr_relation kind is the predicate's relation.
	relation, isRelation := rel.AsRelation(e.predicates[p].table)
	if status != lbool.Undef || !isRelation || relation.RelationPlugin().Name() != "karr_relation" {
		t.Fatal("explicit kind not selected", status)
	}
}

func TestDatalogKarrRelationsAnswerApproximateAndCancellation(t *testing.T) {
	m, g := newPDRLifecycleServices(t)
	e, err := NewPropositionalEngineWithServices(m, g.Context, g)
	fragmentMust(t, err)
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	defer func() { must(e.Close()) }()
	must(e.SelectEngine("datalog"))
	must(g.SourceSetName("fixedpoint.datalog.default_relation", "karr_relation"))
	must(g.SourceSetName("fixedpoint.xform.karr", "true"))
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string) ast.ASTHandle {
		s, err := m.Symbols.Intern(name)
		must(err)
		d, err := m.MkFuncDecl(s, nil, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		must(e.RegisterPredicate(d))
		return d
	}
	p, q := declare("karr-relations-p"), declare("karr-relations-q")
	must(e.AddTableFact(p, nil))
	must(e.AddTableFact(q, nil))
	for _, predicates := range [][]ast.ASTHandle{{p}, {p, q}} {
		status, err := e.QueryPredicates(predicates)
		must(err)
		if status != lbool.Undef {
			t.Fatal("approximate status", status)
		}
		answer, err := e.RelationsAnswer()
		must(err)
		h, err := m.RefGet(answer)
		must(err)
		reduced, err := m.NewRef(ast.ASTHandle{})
		must(err)
		must(e.guard.RewriteWithBindings(h, nil, reduced))
		value, err := m.RefGet(reduced)
		must(err)
		if !m.IsTrue(value) {
			t.Fatal("nonempty nullary conjunction", value)
		}
		must(m.DelRef(&reduced))
		must(m.DelRef(&answer))
	}
	m.Limit.Cancel()
	_, err = e.QueryPredicates([]ast.ASTHandle{p, q})
	m.Limit.ResetCancel()
	if err == nil {
		t.Fatal("cancellation accepted")
	}
	if _, err := e.RelationsAnswer(); !errors.Is(err, ErrPropositionalEngine) {
		t.Fatal("failed query exposed relation answer", err)
	}
}
