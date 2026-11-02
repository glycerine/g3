package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"math/big"
	"testing"
)

func TestRPFPModelConstraintsRenameWithoutScalarCompletion(t *testing.T) {
	ctx, m, s := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	x, err := ctx.BoolConst("constraint-x")
	must(err)
	closeExpr(t, x)
	y, err := ctx.BoolConst("constraint-y")
	must(err)
	closeExpr(t, y)
	boolean, err := x.Sort()
	must(err)
	defer boolean.Close()
	name, err := ctx.StringSymbol("constraint-R")
	must(err)
	decl, err := ctx.Function(name, []*Sort{boolean, boolean}, boolean)
	must(err)
	closeDecl(t, decl)
	term, err := decl.Call(x, y)
	must(err)
	closeExpr(t, term)
	node, err := g.CreateNode(term)
	must(err)
	if _, err = g.EvalNodeAsConstraint(node); err == nil {
		t.Fatal("accepted absent model")
	}
	activation, variables, err := g.RedVars(node)
	must(err)
	defer activation.Close()
	for _, variable := range variables {
		defer variable.Close()
	}
	indexed, err := variables[0].Raw()
	must(err)
	must(s.Assert(indexed))
	status, err := g.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	missing, err := variables[1].Raw()
	must(err)
	missingDecl, err := m.GetDecl(missing)
	must(err)
	before, err := g.dualModel.ConstantInterpretation(missingDecl)
	must(err)
	if !before.IsNull() {
		t.Fatal("missing parameter already interpreted")
	}
	fact, err := g.EvalNodeAsConstraint(node)
	must(err)
	defer fact.Close()
	formula, err := fact.Formula()
	must(err)
	defer formula.Close()
	raw, err := formula.Raw()
	must(err)
	xh, err := x.Raw()
	must(err)
	truth, err := m.MkTrue()
	must(err)
	expected, err := m.MkEq(xh, truth)
	must(err)
	if raw != expected {
		t.Fatal("model constraint failed to rename indexed parameter", raw, expected)
	}
	after, err := g.dualModel.ConstantInterpretation(missingDecl)
	must(err)
	if !after.IsNull() {
		t.Fatal("constraint extraction completed absent scalar")
	}
	must(g.Close())
	// Returned facts own their ASTs independently of the graph/model.
	if _, err = formula.Raw(); err != nil {
		t.Fatal(err)
	}
}

func TestRPFPModelConstraintsZeroArityWithoutModel(t *testing.T) {
	ctx, m, _ := dualityNativeSolverFixture(t)
	term, err := ctx.BoolConst("constraint-zero")
	if err != nil {
		t.Fatal(err)
	}
	closeExpr(t, term)
	g, err := NewRPFP(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	node, err := g.CreateNode(term)
	if err != nil {
		t.Fatal(err)
	}
	fact, err := g.EvalNodeAsConstraint(node)
	if err != nil {
		t.Fatal(err)
	}
	defer fact.Close()
	formula, err := fact.Formula()
	if err != nil {
		t.Fatal(err)
	}
	defer formula.Close()
	raw, err := formula.Raw()
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsTrue(raw) {
		t.Fatal("zero-arity state should have no constraints")
	}
}

func TestRPFPModelConstraintsArrayEntriesOmitDefault(t *testing.T) {
	ctx, m, s, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		must(err)
		ref, e := m.NewRef(h)
		must(e)
		t.Cleanup(func() { must(m.DelRef(&ref)) })
		return h
	}
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	must(g.BindSolver(s))
	must(g.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	status, err := g.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	zero := pin(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
	one := pin(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	two := pin(m.ArithmeticNumeralRat(big.NewRat(2, 1), true))
	integer := pin(m.GetSort(zero))
	arraySort := pin(m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), integer))
	name, err := m.Symbols.Intern("constraint-array-function")
	must(err)
	function := pin(m.MkFuncDecl(name, []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{}))
	table, err := model.NewFunctionInterpretation(m, 1)
	must(err)
	must(table.SetElse(zero))
	must(table.InsertEntry([]ast.ASTHandle{one}, two))
	must(g.dualModel.RegisterFunction(function, table))
	value := pin(m.MkAsArray(function))
	name, err = m.Symbols.Intern("constraint-array")
	must(err)
	array := pin(m.MkConst(name, arraySort))
	declaration, err := m.GetDecl(array)
	must(err)
	must(g.dualModel.RegisterConstant(declaration, value))
	term, err := NewExpr(ctx, array)
	must(err)
	defer term.Close()
	// Source model evaluation expands as-array values into stores over a constant
	// array, which EvalArrayTerm does not recognize. Verify that native behavior
	// separately from explicit-table constraint construction.
	native, err := g.ModelValueAsConstraint(term)
	must(err)
	defer native.Close()
	nativeRaw, err := native.Raw()
	must(err)
	if !m.IsTrue(nativeRaw) {
		t.Fatal("expanded array should use source unsupported-representation fallback")
	}
	finite, err := g.evalArrayTerm(value)
	must(err)
	constraint, err := g.modelArrayEntriesAsConstraint(array, finite)
	must(err)
	defer constraint.Close()
	family, err := m.ArrayFamilyID()
	must(err)
	selected := pin(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, one}))
	expected := pin(m.MkEq(selected, two))
	raw, err := constraint.Raw()
	must(err)
	if raw != expected {
		t.Fatal("array constraint must contain only explicit table entry", raw, expected)
	}

	// Keep explicit rows even when a row equals the default; only the default
	// itself is omitted. Exercise conjunction and empty-table construction too.
	finite.entries[two] = zero
	multiple, err := g.modelArrayEntriesAsConstraint(array, finite)
	must(err)
	defer multiple.Close()
	secondSelect := pin(m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array, two}))
	second := pin(m.MkEq(secondSelect, zero))
	wantMultiple := pin(m.MkAnd(expected, second))
	raw, err = multiple.Raw()
	must(err)
	if raw != wantMultiple {
		t.Fatal("lost explicit default-valued row or changed stable entry order")
	}
	finite.entries = nil
	empty, err := g.modelArrayEntriesAsConstraint(array, finite)
	must(err)
	defer empty.Close()
	raw, err = empty.Raw()
	must(err)
	if !m.IsTrue(raw) {
		t.Fatal("empty table should add no constraint")
	}
	// Constant arrays are not a representation recognized by source EvalArrayTerm.
	constant := pin(m.MkConstArray(arraySort, zero))
	must(g.dualModel.RegisterConstant(declaration, constant))
	fallback, err := g.ModelValueAsConstraint(term)
	must(err)
	defer fallback.Close()
	raw, err = fallback.Raw()
	must(err)
	if !m.IsTrue(raw) {
		t.Fatal("unsupported constant-array representation should add no constraint")
	}
}
