package api

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestSMTContextConstantQuantifierAbstraction(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	boolean := f.pin(f.m.MkBoolSort())
	x := f.pin(f.m.MkConst(astTestSymbol(t, f.m, "constant-x"), boolean))
	body := f.pin(f.m.MkEq(x, x))
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
