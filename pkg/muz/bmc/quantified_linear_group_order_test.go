package bmc_test

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/bmc"
)

func TestQuantifiedLinearUsesSourcePredicateGroupOrder(t *testing.T) {
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
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	rules, err := base.NewRuleSet(rm)
	must(err)
	defer func() { must(rules.Close()) }()
	boolean, err := m.MkBoolSort()
	must(err)
	var decls []ast.ASTHandle
	for i := 0; i < 16; i++ {
		s, e := m.Symbols.Intern(fmt.Sprintf("qgroup-%d", i))
		must(e)
		d, e := m.MkFuncDecl(s, nil, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rm.RegisterPredicate(d))
		decls = append(decls, d)
	}
	var high, low ast.ASTHandle
	for _, a := range decls {
		ha, e := m.GetHash(a)
		must(e)
		for _, b := range decls {
			hb, e := m.GetHash(b)
			must(e)
			if ha&7 > hb&7 {
				high, low = a, b
				break
			}
		}
		if !high.IsNull() {
			break
		}
	}
	if high.IsNull() {
		t.Fatal("fixture needs distinct buckets")
	}
	for _, d := range []ast.ASTHandle{high, low} {
		h, e := m.MkApp(d, nil)
		must(e)
		rule, e := rm.NewRule(h, nil, nil, "")
		must(e)
		must(rules.Add(rule))
		must(rule.Close())
	}
	compiler, err := bmc.NewCompiler(m)
	must(err)
	defer func() { must(compiler.Close()) }()
	formulas, err := compiler.CompileQuantifiedLinear(rules, 4)
	must(err)
	defer func() {
		for i := range formulas {
			must(m.DelRef(&formulas[i]))
		}
	}()
	if len(formulas) != 4 {
		t.Fatal("unexpected grouped formula count", len(formulas))
	}
	first, err := m.RefGet(formulas[0])
	must(err)
	body, err := m.GetQuantifierBody(first)
	must(err)
	selector, err := m.GetArg(body, 0)
	must(err)
	d, err := m.GetDecl(selector)
	must(err)
	name, err := m.GetASTName(d)
	must(err)
	actual, err := m.Symbols.Display(name)
	must(err)
	sourceName, err := m.GetASTName(low)
	must(err)
	expected, err := m.Symbols.Display(sourceName)
	must(err)
	if actual != expected+"#0" {
		t.Fatalf("first compiled selector %q, want %q", actual, expected+"#0")
	}
}
