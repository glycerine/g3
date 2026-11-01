package pdr

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestPDRModelSubstitutionExpandsFunctionBackedArrayValues(t *testing.T) {
	m, owner, transformer, _, closeBase := closureFixture(t)
	defer func() { closureMust(t, transformer.Close()); closureMust(t, owner.Close()); closeBase() }()
	c, err := NewContext(m, nil, 2, ReachableNoCache, owner.contexts.config)
	closureMust(t, err)
	defer func() { closureMust(t, c.Close()) }()
	var pins []ast.ASTRef
	defer func() { closureMust(t, closeASTRefs(m, pins)) }()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		closureMust(t, err)
		ref, err := m.NewRef(h)
		closureMust(t, err)
		pins = append(pins, ref)
		return h
	}
	boolean := own(m.MkBoolSort())
	truth, falsity := own(m.MkTrue()), own(m.MkFalse())
	name, err := m.Symbols.Intern("pdr-array-value-function")
	closureMust(t, err)
	function := own(m.MkFuncDecl(name, []ast.ASTHandle{boolean}, boolean, ast.ASTFuncDeclInfo{}))
	graph, err := model.NewFunctionInterpretation(m, 1)
	closureMust(t, err)
	closureMust(t, graph.SetElse(falsity))
	closureMust(t, graph.InsertEntry([]ast.ASTHandle{truth}, truth))
	md, err := model.NewModel(m)
	closureMust(t, err)
	defer func() { closureMust(t, md.Close()) }()
	closureMust(t, md.Core().RegisterFunction(function, graph))
	array := own(m.MkAsArray(function))
	sort, err := m.GetSort(array)
	closureMust(t, err)
	variable := own(m.MkFreshConst("pdr-array-source", sort))
	declaration, err := m.GetDecl(variable)
	closureMust(t, err)
	closureMust(t, md.Core().RegisterConstant(declaration, array))
	family, err := m.ArrayFamilyID()
	closureMust(t, err)
	expression := own(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{variable, truth}))
	before := m.Stats().LiveReferences
	result, err := c.substituteModelValues(md.Core(), expression, []ast.ASTHandle{variable})
	closureMust(t, err)
	defer func() {
		if result != (ast.ASTRef{}) {
			closureMust(t, m.DelRef(&result))
		}
	}()
	h, err := m.RefGet(result)
	closureMust(t, err)
	if !m.IsTrue(h) {
		t.Fatal("function-backed array was not evaluated before substitution")
	}
	closureMust(t, m.DelRef(&result))
	if got := m.Stats().LiveReferences; got != before {
		t.Fatal("array substitution leaked reference slots", got, before)
	}
	original, err := md.Core().ConstantInterpretation(declaration)
	closureMust(t, err)
	if original != array {
		t.Fatal("array substitution rewrote the stored model interpretation")
	}
}
