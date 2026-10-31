package api

import (
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"testing"
)

func TestSMTContextConstantQuantifierAbstraction(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	x := f.pin(f.m.MkConst(astTestSymbol(t, f.m, "constant-x"), boolean))
	decl := f.pin(f.m.MkFuncDecl(astTestSymbol(t, f.m, "constant-pattern-f"), []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
	body := f.pin(f.m.MkApp(decl, []ast.ASTHandle{x}))
	pattern := f.pin(f.c.Pattern(body))
	quantifier := f.pin(f.c.ConstantQuantifier(ConstantQuantifierSpec{Forall: true, Bound: []ast.ASTHandle{x}, Body: body, Weight: 3, Patterns: []ast.ASTHandle{pattern}}))
	if yes, err := f.c.IsForall(quantifier); err != nil || !yes {
		t.Fatal("forall", yes, err)
	}
	if name, err := f.c.QuantifierBoundName(quantifier, 0); err != nil || name != "constant-x" {
		t.Fatal("name", name, err)
	}
	abstracted := f.pin(f.c.QuantifierBody(quantifier))
	left := f.pin(f.m.GetArg(abstracted, 0))
	if kind, err := f.m.GetKind(left); err != nil || kind != ast.ASTVar {
		t.Fatal("abstracted bound", kind, err)
	}
	if index, err := f.m.GetVarIndex(left); err != nil || index != 0 {
		t.Fatal("bound index", index, err)
	}
	gotPattern := f.pin(f.c.QuantifierPattern(quantifier, 0))
	term := f.pin(f.c.PatternTerm(gotPattern, 0))
	triggerArg := f.pin(f.m.GetArg(term, 0))
	if kind, err := f.m.GetKind(triggerArg); err != nil || kind != ast.ASTVar {
		t.Fatal("abstracted pattern", kind, err)
	}
}

func TestSMTContextConstantQuantifierValidation(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	body := f.pin(f.m.MkTrue())
	if _, err := f.c.ForallConstants(nil, body); err != ast.ErrASTDomain {
		t.Fatal("empty bound", err)
	}
	if _, err := f.c.ForallConstants([]ast.ASTHandle{body}, body); err != ast.ErrASTDomain {
		t.Fatal("non-uninterpreted bound", err)
	}
}

func TestSMTContextConstantQuantifierPreservesNumericName(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	constant := f.pin(f.m.MkConst(usymbol.Numerical(37), boolean))
	quantifier := f.pin(f.c.ForallConstants([]ast.ASTHandle{constant}, constant))
	name, err := f.m.GetDeclarationName(quantifier, 0)
	if err != nil || !name.IsNumerical() || name.Value != 37 {
		t.Fatal("numeric binder name", name, err)
	}
}
