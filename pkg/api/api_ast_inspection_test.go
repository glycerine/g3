package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestSMTContextASTInspection(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	u := f.pin(f.c.UninterpretedSort("InspectU"))
	boolean := f.pin(f.c.BoolSort())
	fn := f.pin(f.c.Function("inspect-f", []ast.ASTHandle{u}, boolean))
	x := f.pin(f.c.Constant("inspect-x", u))
	app := f.pin(f.c.Apply(fn, x))
	if yes, err := f.c.IsApplication(app); err != nil || !yes {
		t.Fatal("application", yes, err)
	}
	if kind, err := f.c.APIKind(app); err != nil || kind != APIASTApplication {
		t.Fatal("api app kind", kind, err)
	}
	if kind, err := f.c.APIKind(u); err != nil || kind != APIASTSort {
		t.Fatal("api sort kind", kind, err)
	}
	if kind, err := f.c.APIKind(fn); err != nil || kind != APIASTFunctionDeclaration {
		t.Fatal("api decl kind", kind, err)
	}
	if good, err := f.c.IsWellSorted(app); err != nil || !good {
		t.Fatal("well sorted", good, err)
	}
	decl := f.pin(f.c.ApplicationDeclaration(app))
	if decl != fn {
		t.Fatal("declaration", decl, fn)
	}
	if count, err := f.c.ApplicationArgumentCount(app); err != nil || count != 1 {
		t.Fatal("args", count, err)
	}
	if arg := f.pin(f.c.ApplicationArgument(app, 0)); arg != x {
		t.Fatal("arg", arg, x)
	}
	if name, err := f.c.DeclarationName(fn); err != nil || name != "inspect-f" {
		t.Fatal("name", name, err)
	}
	if arity, err := f.c.DeclarationArity(fn); err != nil || arity != 1 {
		t.Fatal("arity", arity, err)
	}
	if domain := f.pin(f.c.DeclarationDomain(fn, 0)); domain != u {
		t.Fatal("domain", domain, u)
	}
	if r := f.pin(f.c.DeclarationRange(fn)); r != boolean {
		t.Fatal("range", r, boolean)
	}
	id, err := f.c.ASTID(app)
	astTestMust(t, err)
	if id == 0 {
		t.Fatal("id", id)
	}
	hash, err := f.c.ASTHash(app)
	astTestMust(t, err)
	if hash == 0 {
		t.Fatal("hash", hash)
	}
	if same, err := f.c.ASTEqual(app, app); err != nil || !same {
		t.Fatal("equality", same, err)
	}
}

func TestSMTContextASTVariableAndBooleanInspection(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.c.BoolSort())
	variable := f.pin(f.m.MkVar(4, boolean))
	if kind, err := f.c.APIKind(variable); err != nil || kind != APIASTVariable {
		t.Fatal("api variable kind", kind, err)
	}
	if index, err := f.c.VariableIndex(variable); err != nil || index != 4 {
		t.Fatal("index", index, err)
	}
	truth := f.pin(f.c.Bool(true))
	falsehood := f.pin(f.c.Bool(false))
	symbolic := f.pin(f.c.Constant("p", boolean))
	ints := f.pin(f.c.IntegerSort())
	numeral := f.pin(f.c.IntNumeral(3, ints))
	if kind, err := f.c.APIKind(numeral); err != nil || kind != APIASTNumeral {
		t.Fatal("api numeral kind", kind, err)
	}
	for _, tc := range []struct {
		term ast.ASTHandle
		want lbool.LBool
	}{{truth, lbool.True}, {falsehood, lbool.False}, {symbolic, lbool.Undef}} {
		got, err := f.c.BooleanValue(tc.term)
		if err != nil || got != tc.want {
			t.Fatal("boolean", got, tc.want, err)
		}
	}
}
