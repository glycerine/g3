package pdr

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestPDRProjectedChildCubeUsesSourceBooleanRewriter(t *testing.T) {
	m, context, _ := classifierContextFixture(t)
	boolean, err := m.MkBoolSort()
	closureMust(t, err)
	name, err := m.Symbols.Intern("projected-rewrite-head")
	closureMust(t, err)
	head, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	transformer, err := NewPredicateTransformer(context.owner, head, ReachableNoCache)
	closureMust(t, err)
	defer func() { closureMust(t, context.search.Reset()); closureMust(t, transformer.Close()) }()
	truth, err := m.MkTrue()
	closureMust(t, err)
	root, err := NewModelNode(nil, truth, transformer, 1)
	closureMust(t, err)
	closureMust(t, context.search.SetRoot(root))
	name, err = m.Symbols.Intern("projected-rewrite-state")
	closureMust(t, err)
	decl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	p, err := m.MkApp(decl, nil)
	closureMust(t, err)
	pRef, err := m.NewRef(p)
	closureMust(t, err)
	notP, err := m.MkNot(p)
	closureMust(t, err)
	notRef, err := m.NewRef(notP)
	closureMust(t, err)
	defer func() { closureMust(t, m.DelRef(&notRef)); closureMust(t, m.DelRef(&pRef)) }()
	before := m.Stats().LiveReferences
	closureMust(t, context.addProjectedChild(root, transformer, []ast.ASTHandle{p, p}))
	if len(root.children) != 1 || root.children[0].state != p {
		t.Fatal("duplicate child literals did not collapse", root.children)
	}
	if got := m.Stats().LiveReferences; got != before+1 {
		t.Fatalf("duplicate child owns %d references; want %d", got, before+1)
	}
	closureMust(t, context.addProjectedChild(root, transformer, []ast.ASTHandle{p, notP}))
	if len(root.children) != 2 || !m.IsFalse(root.children[1].state) {
		t.Fatal("complementary child literals did not reduce to false")
	}
	if got := m.Stats().LiveReferences; got != before+2 {
		t.Fatalf("children own %d references; want %d", got, before+2)
	}
}
