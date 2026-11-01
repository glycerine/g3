package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"math/big"
	"testing"
)

func TestRPFPArrayEqualityFunctionTablesAndStores(t *testing.T) {
	ctx, m, s := dualityNativeSolverFixture(t)
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
	status, err := g.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	zero := pin(m.ArithmeticNumeralRat(big.NewRat(0, 1), true))
	one := pin(m.ArithmeticNumeralRat(big.NewRat(1, 1), true))
	two := pin(m.ArithmeticNumeralRat(big.NewRat(2, 1), true))
	integer := pin(m.GetSort(zero))
	boolean := pin(m.MkTrue())
	numeral, err := g.arrayStoreNumeral(boolean)
	must(err)
	if numeral {
		t.Fatal("Boolean unique value accepted as arithmetic store numeral")
	}
	arrayFamily, err := m.ArrayFamilyID()
	must(err)
	functionArray := func(name string, fallback ast.ASTHandle, entries map[ast.ASTHandle]ast.ASTHandle) ast.ASTHandle {
		t.Helper()
		rawName, err := m.Symbols.Intern(name)
		must(err)
		decl := pin(m.MkFuncDecl(rawName, []ast.ASTHandle{integer}, integer, ast.ASTFuncDeclInfo{}))
		graph, err := model.NewFunctionInterpretation(m, 1)
		must(err)
		must(graph.SetElse(fallback))
		for key, value := range entries {
			must(graph.InsertEntry([]ast.ASTHandle{key}, value))
		}
		must(g.dualModel.RegisterFunction(decl, graph))
		return pin(m.MkAsArray(decl))
	}
	a := functionArray("array-eval-a", zero, map[ast.ASTHandle]ast.ASTHandle{one: two})
	b := functionArray("array-eval-b", zero, map[ast.ASTHandle]ast.ASTHandle{one: two})
	c := functionArray("array-eval-c", one, map[ast.ASTHandle]ast.ASTHandle{one: two})
	empty := functionArray("array-eval-empty", zero, nil)
	store := func(base, index, value ast.ASTHandle) ast.ASTHandle {
		return pin(m.MkFamilyApp(arrayFamily, ast.ASTArrayOpStore, nil, []ast.ASTHandle{base, index, value}))
	}
	check := func(left, right ast.ASTHandle, want int) {
		t.Helper()
		h := pin(m.MkEq(left, right))
		formula, err := NewExpr(ctx, h)
		must(err)
		defer formula.Close()
		got, err := g.EvalArrayEquality(formula)
		must(err)
		defer got.Close()
		raw, err := got.Raw()
		must(err)
		if (want == 1 && !m.IsTrue(raw)) || (want == 0 && !m.IsFalse(raw)) || (want == 2 && raw != h) {
			t.Fatalf("array comparison result %v, want %d", raw, want)
		}
	}
	check(a, b, 1)
	check(a, c, 0)
	check(a, empty, 0)
	check(store(a, one, zero), empty, 1)
	check(store(empty, one, two), b, 1)
	check(store(store(empty, one, two), one, zero), empty, 1)
	name, err := m.Symbols.Intern("array-eval-symbolic-index")
	must(err)
	symbolic := pin(m.MkConst(name, integer))
	check(store(a, symbolic, zero), b, 2)
	check(store(a, one, symbolic), b, 2)
	arraySort := pin(m.GetSort(a))
	constant := pin(m.MkConstArray(arraySort, zero))
	// The source recognizes as-array/store only, even for constant arrays.
	check(constant, empty, 2)
	// A default-valued source table entry is not canonicalized away by the source.
	redundant := functionArray("array-eval-redundant", zero, map[ast.ASTHandle]ast.ASTHandle{one: zero})
	check(redundant, empty, 0)
}
