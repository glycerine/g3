package rel_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestExplanationRelationJoinProjectAndRename(t *testing.T) {
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
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			must(m.DelRef(&refs[i]))
		}
	}()
	value := func(name string) ast.ASTHandle {
		symbol, e := m.Symbols.Intern(name)
		must(e)
		h, e := m.MkConst(symbol, sort)
		must(e)
		ref, e := m.NewRef(h)
		must(e)
		refs = append(refs, ref)
		return h
	}
	a, b, c := value("expl-a"), value("expl-b"), value("expl-c")
	left, err := rel.NewExplanationRelation(m, []ast.ASTHandle{sort, sort})
	must(err)
	defer func() { must(left.Close()) }()
	right, err := rel.NewExplanationRelation(m, []ast.ASTHandle{sort})
	must(err)
	defer func() { must(right.Close()) }()
	must(left.AddFact([]ast.ASTHandle{a, b}))
	must(right.AddFact([]ast.ASTHandle{c}))
	joined, err := left.Join(right)
	must(err)
	defer func() { must(joined.Close()) }()
	row, err := joined.Data()
	must(err)
	if len(row) != 3 || row[0] != a || row[1] != b || row[2] != c {
		t.Fatal("joined row", row)
	}
	projected, err := joined.Project([]int{1})
	must(err)
	defer func() { must(projected.Close()) }()
	row, err = projected.Data()
	must(err)
	if len(row) != 2 || row[0] != a || row[1] != c {
		t.Fatal("projected row", row)
	}
	renamed, err := joined.RenameCycle([]int{0, 1, 2})
	must(err)
	defer func() { must(renamed.Close()) }()
	row, err = renamed.Data()
	must(err)
	if len(row) != 3 || row[0] != b || row[1] != c || row[2] != a {
		t.Fatal("renamed row", row)
	}
	must(right.Reset())
	empty, err := left.Join(right)
	must(err)
	defer func() { must(empty.Close()) }()
	isEmpty, err := empty.Empty()
	must(err)
	if !isEmpty {
		t.Fatal("join with empty relation")
	}
	if _, err = joined.Project([]int{2, 1}); err == nil {
		t.Fatal("unsorted projection accepted")
	}
}

func TestExplanationRelationFactAndRelationLevelUnion(t *testing.T) {
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
		sym, e := m.Symbols.Intern(name)
		must(e)
		h, e := m.MkConst(sym, sort)
		must(e)
		return h
	}
	a, b := mk("union-a"), mk("union-b")
	newRelation := func() *rel.ExplanationRelation {
		r, e := rel.NewExplanationRelation(m, []ast.ASTHandle{sort})
		must(e)
		return r
	}
	first, second, target, delta := newRelation(), newRelation(), newRelation(), newRelation()
	defer func() { must(delta.Close()); must(target.Close()); must(second.Close()); must(first.Close()) }()
	must(first.AddFact([]ast.ASTHandle{a}))
	must(second.AddFact([]ast.ASTHandle{b}))
	must(target.Union(first, delta, false))
	must(target.Union(second, delta, false))
	row, err := target.Data()
	must(err)
	if len(row) != 1 || row[0] != a {
		t.Fatal("fact-level union replaced first explanation", row)
	}
	must(target.Reset())
	must(delta.Reset())
	must(target.Union(first, delta, true))
	must(target.Union(second, delta, true))
	for _, relation := range []*rel.ExplanationRelation{target, delta} {
		row, err = relation.Data()
		must(err)
		if len(row) != 1 {
			t.Fatal("relation-level row", row)
		}
		decl, err := m.GetDecl(row[0])
		must(err)
		name, err := m.GetASTName(decl)
		must(err)
		text, err := m.Symbols.Display(name)
		must(err)
		left, err := m.GetArg(row[0], 0)
		must(err)
		right, err := m.GetArg(row[0], 1)
		must(err)
		if text != "e_union" || left != a || right != b {
			t.Fatal("relation-level union", text, left, right)
		}
	}
	empty := newRelation()
	defer func() { must(empty.Close()) }()
	undefined, err := empty.Complement()
	must(err)
	defer func() { must(undefined.Close()) }()
	if err = target.Union(undefined, nil, true); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("undefined union", err)
	}
}
