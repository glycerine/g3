package rel_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestExplanationTransformerAddsRuleRecords(t *testing.T) {
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
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	integer, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	sort, err := m.GetSort(integer)
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	declare := func(name string) ast.ASTHandle {
		symbol, e := m.Symbols.Intern(name)
		must(e)
		decl, e := m.MkFuncDecl(symbol, []ast.ASTHandle{sort}, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rm.RegisterPredicate(decl))
		return decl
	}
	h, p, n := declare("expl-head"), declare("expl-pos"), declare("expl-neg")
	x, err := m.MkVar(0, sort)
	must(err)
	app := func(decl ast.ASTHandle) ast.ASTHandle {
		value, e := m.MkApp(decl, []ast.ASTHandle{x})
		must(e)
		return value
	}
	original, err := rm.NewRule(app(h), []ast.ASTHandle{app(p), app(n)}, []bool{false, true}, "named-rule")
	must(err)
	defer func() { must(original.Close()) }()
	transformer, err := rel.NewExplanationTransformer(m, rm)
	must(err)
	decl, err := transformer.ExplanationDeclaration(h)
	must(err)
	repeated, err := transformer.ExplanationDeclaration(h)
	must(err)
	if decl != repeated {
		t.Fatal("explanation declaration not reused")
	}
	arity, err := m.GetArity(decl)
	must(err)
	if arity != 2 {
		t.Fatal("explanation declaration arity", arity)
	}
	ruleSort, err := m.GetDomain(decl, 1)
	must(err)
	isRule, err := m.IsDatalogRuleSort(ruleSort)
	must(err)
	if !isRule {
		t.Fatal("explanation column is not rule sort")
	}
	transformed, err := transformer.ExplanationRule(original)
	must(err)
	defer func() { must(transformed.Close()) }()
	must(transformer.Close())
	head, err := transformed.Head()
	must(err)
	headDecl, err := m.GetDecl(head)
	must(err)
	if headDecl != decl {
		t.Fatal("head declaration", headDecl, decl)
	}
	positive, err := transformed.PositiveTailSize()
	must(err)
	total, err := transformed.TailSize()
	must(err)
	if positive != 1 || total != 3 {
		t.Fatal("tail layout", positive, total)
	}
	pos, negated, err := transformed.Tail(0)
	must(err)
	if negated {
		t.Fatal("positive explanation tail negated")
	}
	count, err := m.GetNumArgs(pos)
	must(err)
	if count != 2 {
		t.Fatal("explanation tail arity", count)
	}
	posExpl, err := m.GetArg(pos, 1)
	must(err)
	neg, negated, err := transformed.Tail(1)
	must(err)
	negDecl, err := m.GetDecl(neg)
	must(err)
	if !negated || negDecl != n {
		t.Fatal("negative tail changed", negated, negDecl)
	}
	equality, _, err := transformed.Tail(2)
	must(err)
	left, err := m.GetArg(equality, 0)
	must(err)
	right, err := m.GetArg(equality, 1)
	must(err)
	headExpl, err := m.GetArg(head, 1)
	must(err)
	if left != headExpl {
		t.Fatal("head explanation variable", left, headExpl)
	}
	recordDecl, err := m.GetDecl(right)
	must(err)
	recordName, err := m.GetASTName(recordDecl)
	must(err)
	text, err := m.Symbols.Display(recordName)
	must(err)
	argument, err := m.GetArg(right, 0)
	must(err)
	if text != "named-rule" || argument != posExpl {
		t.Fatal("rule record", text, argument, posExpl)
	}
}

func TestExplanationTransformerUsesUnnamedRuleDisplay(t *testing.T) {
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
	rm, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rm.Close()) }()
	boolean, err := m.MkBoolSort()
	must(err)
	name, err := m.Symbols.Intern("unnamed-fact")
	must(err)
	decl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rm.RegisterPredicate(decl))
	head, err := m.MkApp(decl, nil)
	must(err)
	original, err := rm.NewRule(head, nil, nil, "")
	must(err)
	defer func() { must(original.Close()) }()
	var display bytes.Buffer
	must(rm.WriteRuleDisplay(&display, original))
	want := strings.TrimRight(display.String(), "\n")
	transformer, err := rel.NewExplanationTransformer(m, rm)
	must(err)
	defer func() { must(transformer.Close()) }()
	rule, err := transformer.ExplanationRule(original)
	must(err)
	defer func() { must(rule.Close()) }()
	equality, _, err := rule.Tail(0)
	must(err)
	record, err := m.GetArg(equality, 1)
	must(err)
	recordDecl, err := m.GetDecl(record)
	must(err)
	symbol, err := m.GetASTName(recordDecl)
	must(err)
	got, err := m.Symbols.Display(symbol)
	must(err)
	if got != want {
		t.Fatalf("unnamed rule symbol = %q, want %q", got, want)
	}
	arity, err := m.GetArity(recordDecl)
	must(err)
	if arity != 0 {
		t.Fatal("fact record arity", arity)
	}
}
