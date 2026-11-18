package protomodel

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestProtoModelCleanupOwnsRewrittenFallbackUntilInstalled(t *testing.T) {
	m, p := protoFixture(t)
	b, err := m.MkBoolSort()
	protoMust(t, err)
	x, err := m.MkVar(0, b)
	protoMust(t, err)
	c, err := m.MkConst(protoSymbol(t, m, "cleanup-late"), b)
	protoMust(t, err)
	yes, err := m.MkTrue()
	protoMust(t, err)
	no, err := m.MkFalse()
	protoMust(t, err)
	body, err := m.MkIte(x, c, yes)
	protoMust(t, err)
	d, err := m.MkFuncDecl(protoSymbol(t, m, "cleanup-rewritten"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	protoMust(t, err)
	protoPin(t, m, d)
	fi, err := model.NewFunctionInterpretation(m, 1)
	protoMust(t, err)
	protoMust(t, fi.SetElse(body))
	protoMust(t, p.Core().RegisterFunction(d, fi))
	protoMust(t, p.Cleanup())
	cleaned, err := fi.Else()
	protoMust(t, err)
	if cleaned == body {
		t.Fatal("cleanup did not replace the uninterpreted constant")
	}
	out := protoPin(t, m, ast.ASTHandle{})
	for _, argument := range []ast.ASTHandle{yes, no} {
		application, err := m.MkApp(d, []ast.ASTHandle{argument})
		protoMust(t, err)
		_, err = p.Eval(application, out, true)
		protoMust(t, err)
		value, err := m.RefGet(out)
		protoMust(t, err)
		if (argument == yes && !m.IsFalse(value)) || (argument != yes && !m.IsTrue(value)) {
			t.Fatal("rewritten fallback lost its Boolean meaning", argument, value)
		}
	}
}

// AUDIT.md smtcore_14-F4: cleanup removes unused auxiliary declarations from
// m_decls (remove_aux_decls_not_in_set), not only from the tables.
func TestProtoModelCleanupDropsUnusedAuxiliaryDeclarations(t *testing.T) {
	m, p := protoFixture(t)
	b, err := m.MkBoolSort()
	protoMust(t, err)
	yes, err := m.MkTrue()
	protoMust(t, err)
	d, err := m.MkFuncDecl(protoSymbol(t, m, "cleanup-kept"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	protoMust(t, err)
	protoPin(t, m, d)
	aux, err := m.MkFuncDecl(protoSymbol(t, m, "cleanup-aux"), []ast.ASTHandle{b}, b, ast.ASTFuncDeclInfo{})
	protoMust(t, err)
	protoPin(t, m, aux)
	fi, err := model.NewFunctionInterpretation(m, 1)
	protoMust(t, err)
	protoMust(t, fi.SetElse(yes))
	protoMust(t, p.Core().RegisterFunction(d, fi))
	auxFI, err := model.NewFunctionInterpretation(m, 1)
	protoMust(t, err)
	protoMust(t, auxFI.SetElse(yes))
	protoMust(t, p.RegisterAuxFunction(aux, auxFI))
	protoMust(t, p.Cleanup())
	n, err := p.Core().NumDeclarations()
	protoMust(t, err)
	for i := uint32(0); i < n; i++ {
		decl, err := p.Core().Declaration(i)
		protoMust(t, err)
		if decl == aux {
			t.Fatal("unused auxiliary declaration kept in m_decls")
		}
	}
	if n != 1 {
		t.Fatal("declarations after cleanup", n)
	}
}
