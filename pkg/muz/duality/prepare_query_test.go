package duality_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/duality"
)

func TestPrepareQueryRulesMissingHeadsAndBounds(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx, err := cmd_context.NewStandaloneContext()
	must(err)
	defer func() { must(ctx.Close()) }()
	m, err := ctx.Manager()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	decl := func(name string) ast.ASTHandle {
		sym, e := m.Symbols.Intern(name)
		must(e)
		h, e := m.MkFuncDecl(sym, nil, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		return h
	}
	p, q, r := decl("prepared-p"), decl("prepared-q"), decl("prepared-r")
	pApp, err := m.MkApp(p, nil)
	must(err)
	rApp, err := m.MkApp(r, nil)
	must(err)
	truth, err := m.MkTrue()
	must(err)
	ruleP, err := m.MkImplies(truth, pApp)
	must(err)
	pOwner, err := m.NewRef(ruleP)
	must(err)
	ruleR, err := m.MkImplies(truth, rApp)
	must(err)
	rOwner, err := m.NewRef(ruleR)
	must(err)
	prepared, err := duality.PrepareQuery(m, []ast.ASTHandle{ruleP, ruleR}, []uint32{math.MaxUint32, 7}, pApp, []ast.ASTHandle{p, q, r}, 10)
	must(err)
	defer func() { must(prepared.Close()) }()
	must(m.DelRef(&pOwner))
	must(m.DelRef(&rOwner))
	clauses, err := prepared.Clauses()
	must(err)
	if len(clauses) != 4 {
		t.Fatalf("got %d clauses", len(clauses))
	}
	if clauses[0] != ruleP || clauses[1] != ruleR {
		t.Fatal("rule order changed")
	}
	head, err := m.GetArg(clauses[3], 1)
	must(err)
	d, err := m.GetDecl(head)
	must(err)
	if d != q {
		t.Fatal("missing predicate head was not added")
	}
	bounds, err := prepared.Bounds()
	must(err)
	if !reflect.DeepEqual(bounds, []uint32{10, 7, 10, 10}) {
		t.Fatal("bounds", bounds)
	}
	bounds[0] = 0
	again, err := prepared.Bounds()
	must(err)
	if again[0] != 10 {
		t.Fatal("bounds were aliased")
	}
	if _, err = duality.PrepareQuery(m, []ast.ASTHandle{ruleP}, nil, pApp, nil, 10); err == nil {
		t.Fatal("accepted mismatched bounds")
	}
}
