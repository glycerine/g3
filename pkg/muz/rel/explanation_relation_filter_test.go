package rel_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestExplanationRelationAssignmentAndIntersectionFilters(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	sort, err := m.MkDatalogRuleSort()
	must(err)
	mk := func(name string) ast.ASTHandle {
		symbol, e := m.Symbols.Intern(name)
		must(e)
		h, e := m.MkConst(symbol, sort)
		must(e)
		return h
	}
	a, b := mk("filter-a"), mk("filter-b")
	aRef, err := m.NewRef(a)
	must(err)
	defer func() { must(m.DelRef(&aRef)) }()
	bRef, err := m.NewRef(b)
	must(err)
	defer func() { must(m.DelRef(&bRef)) }()
	newRelation := func() *rel.ExplanationRelation {
		r, e := rel.NewExplanationRelation(m, []ast.ASTHandle{sort})
		must(e)
		return r
	}
	blank := newRelation()
	defer func() { must(blank.Close()) }()
	target, err := blank.Complement()
	must(err)
	defer func() { must(target.Close()) }()
	variable, err := m.MkVar(0, sort)
	must(err)
	condition, err := m.MkEq(variable, a)
	must(err)
	matched, err := target.FilterInterpreted(condition)
	must(err)
	if !matched {
		t.Fatal("ground assignment not recognized")
	}
	row, err := target.Data()
	must(err)
	if len(row) != 1 || row[0] != a {
		t.Fatal("assignment", row)
	}
	if matched, err = target.FilterInterpreted(mustTrue(t, m)); err != nil || matched {
		t.Fatal("unsupported filter", matched, err)
	}
	first, second := newRelation(), newRelation()
	defer func() { must(second.Close()); must(first.Close()) }()
	must(first.AddFact([]ast.ASTHandle{a}))
	must(second.AddFact([]ast.ASTHandle{b}))
	must(target.Union(second, nil, true))
	must(target.FilterIntersection(first))
	row, err = target.Data()
	must(err)
	if len(row) != 1 || row[0] != a {
		t.Fatal("intersection did not select union child", row)
	}
	undefined, err := blank.Complement()
	must(err)
	defer func() { must(undefined.Close()) }()
	must(undefined.FilterIntersection(second))
	row, err = undefined.Data()
	must(err)
	if len(row) != 1 || row[0] != b {
		t.Fatal("intersection did not fill undefined column", row)
	}
	must(target.FilterIntersection(blank))
	isEmpty, err := target.Empty()
	must(err)
	if !isEmpty {
		t.Fatal("empty intersection did not reset target")
	}
}

func TestExplanationRelationNegationFilter(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	context, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(context.Close()) }()
	m, err := context.Manager()
	must(err)
	sort, err := m.MkDatalogRuleSort()
	must(err)
	name, err := m.Symbols.Intern("negation-rule")
	must(err)
	value, err := m.MkConst(name, sort)
	must(err)
	target, err := rel.NewExplanationRelation(m, []ast.ASTHandle{sort})
	must(err)
	defer func() { must(target.Close()) }()
	negative, err := rel.NewExplanationRelation(m, []ast.ASTHandle{sort})
	must(err)
	defer func() { must(negative.Close()) }()
	must(target.AddFact([]ast.ASTHandle{value}))
	must(target.FilterNegation(negative))
	isEmpty, err := target.Empty()
	must(err)
	if isEmpty {
		t.Fatal("empty negation removed row")
	}
	must(negative.AddFact([]ast.ASTHandle{value}))
	must(target.FilterNegation(negative))
	isEmpty, err = target.Empty()
	must(err)
	if !isEmpty {
		t.Fatal("nonempty negation did not remove row")
	}
}

func mustTrue(t *testing.T, m *ast.ASTManager) ast.ASTHandle {
	t.Helper()
	h, err := m.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	return h
}
