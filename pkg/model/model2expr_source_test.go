package model_test

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func TestModel2ExprSourceConstantFullAndPartialFunctionOrder(t *testing.T) {
	m, md, _, _, _, _ := evaluatorFixture(t)
	sort := mustBoolSort(t, m)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	constant, e := m.MkFuncDecl(astTestSymbol(t, m, "model2expr-constant"), nil, sort, ast.ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, constant)
	astTestMust(t, md.Core().RegisterConstant(constant, yes))
	full := modelTestFunctionDecl(t, m, "A")
	fullFI := modelTestFunction(t, m, 1)
	astTestMust(t, fullFI.SetElse(yes))
	astTestMust(t, fullFI.InsertEntry([]ast.ASTHandle{yes}, no))
	astTestMust(t, md.Core().RegisterFunction(full, fullFI))
	partial := modelTestFunctionDecl(t, m, "B")
	partialFI := modelTestFunction(t, m, 1)
	astTestMust(t, partialFI.InsertEntry([]ast.ASTHandle{no}, yes))
	astTestMust(t, md.Core().RegisterFunction(partial, partialFI))
	out := astTestPin(t, m, ast.ASTHandle{})
	builder, e := model.NewModel2ExprBuilder(md.Core())
	astTestMust(t, e)
	astTestMust(t, builder.Build(out))
	astTestMust(t, builder.Close())
	result, e := m.RefGet(out)
	astTestMust(t, e)
	op, e := m.GetDeclKind(result)
	astTestMust(t, e)
	n, e := m.GetNumArgs(result)
	astTestMust(t, e)
	if op != ast.ASTOpAnd || n != 3 {
		t.Fatal("source conjunction order", op, n)
	}
	first, e := m.GetArg(result, 0)
	astTestMust(t, e)
	constantApp, e := m.MkApp(constant, nil)
	astTestMust(t, e)
	want, e := m.MkEq(constantApp, yes)
	astTestMust(t, e)
	if first != want {
		t.Fatal("constant clause precedes functions", first, want)
	}
	for i, expected := range []struct {
		name string
		body int32
	}{{"C", ast.ASTOpIff}, {"A", ast.ASTOpImplies}} {
		q, e := m.GetArg(result, uint32(i+1))
		astTestMust(t, e)
		kind, e := m.GetKind(q)
		astTestMust(t, e)
		if kind != ast.ASTQuantifier {
			t.Fatal("missing function quantifier", i, kind)
		}
		name, e := m.GetDeclarationName(q, 0)
		astTestMust(t, e)
		spelling, e := m.Symbols.String(name)
		astTestMust(t, e)
		body, e := m.GetQuantifierBody(q)
		astTestMust(t, e)
		bodyOp, e := m.GetDeclKind(body)
		astTestMust(t, e)
		if spelling != expected.name || bodyOp != expected.body {
			t.Fatal("source function clause", i, spelling, bodyOp)
		}
	}
}

func TestModel2ExprSourceBuilderAllocationRefusalsReleaseRoots(t *testing.T) {
	m, md, _, _, _, _ := evaluatorFixture(t)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	d := modelTestFunctionDecl(t, m, "refusal-function")
	fi := modelTestFunction(t, m, 1)
	astTestMust(t, fi.SetElse(yes))
	astTestMust(t, fi.InsertEntry([]ast.ASTHandle{yes}, no))
	astTestMust(t, md.Core().RegisterFunction(d, fi))
	out := astTestPin(t, m, ast.ASTHandle{})
	roots := []ast.ASTHandle{d, yes, no}
	for fault := int64(0); fault < 160; fault++ {
		counts := make([]uint32, len(roots))
		for i, h := range roots {
			counts[i], e = m.GetRefCount(h)
			astTestMust(t, e)
		}
		builder, e := model.NewModel2ExprBuilder(md.Core())
		astTestMust(t, e)
		m.Budget().FailAfter(fault)
		failure := builder.Build(out)
		m.Budget().FailAfter(-1)
		if failure != nil && !errors.Is(failure, arena.ErrInjectedFailure) {
			t.Fatal("unexpected refusal", fault, failure)
		}
		astTestMust(t, builder.Close())
		astTestMust(t, m.SetRef(out, ast.ASTHandle{}))
		for i, h := range roots {
			count, e := m.GetRefCount(h)
			astTestMust(t, e)
			if count != counts[i] {
				t.Fatal("builder retained root", fault, i, count, counts[i])
			}
		}
		if failure == nil {
			return
		}
	}
	t.Fatal("allocation campaign did not reach success")
}

func TestModel2ExprSourceReverseEntriesAndEmptyPartialSkip(t *testing.T) {
	m, md, _, _, _, _ := evaluatorFixture(t)
	yes, e := m.MkTrue()
	astTestMust(t, e)
	no, e := m.MkFalse()
	astTestMust(t, e)
	full := modelTestFunctionDecl(t, m, "reverse-full")
	fullFI := modelTestFunction(t, m, 1)
	astTestMust(t, fullFI.SetElse(no))
	astTestMust(t, fullFI.InsertEntry([]ast.ASTHandle{yes}, no))
	astTestMust(t, fullFI.InsertEntry([]ast.ASTHandle{no}, yes))
	astTestMust(t, md.Core().RegisterFunction(full, fullFI))
	partial := modelTestFunctionDecl(t, m, "reverse-partial")
	partialFI := modelTestFunction(t, m, 1)
	astTestMust(t, partialFI.InsertEntry([]ast.ASTHandle{yes}, no))
	astTestMust(t, partialFI.InsertEntry([]ast.ASTHandle{no}, yes))
	astTestMust(t, md.Core().RegisterFunction(partial, partialFI))
	empty := modelTestFunctionDecl(t, m, "empty-partial")
	emptyFI := modelTestFunction(t, m, 1)
	astTestMust(t, md.Core().RegisterFunction(empty, emptyFI))
	out := astTestPin(t, m, ast.ASTHandle{})
	builder, e := model.NewModel2ExprBuilder(md.Core())
	astTestMust(t, e)
	astTestMust(t, builder.Build(out))
	result, e := m.RefGet(out)
	astTestMust(t, e)
	n, e := m.GetNumArgs(result)
	astTestMust(t, e)
	if n != 2 {
		t.Fatal("empty partial function contributed a clause", n)
	}
	for i := uint32(0); i < 2; i++ {
		quant, e := m.GetArg(result, i)
		astTestMust(t, e)
		body, e := m.GetQuantifierBody(quant)
		astTestMust(t, e)
		if i == 0 {
			left, e := m.GetArg(body, 0)
			astTestMust(t, e)
			right, e := m.GetArg(body, 1)
			astTestMust(t, e)
			if kind, _ := m.GetDeclKind(left); kind == ast.ASTOpIte {
				body = left
			} else {
				body = right
			}
		}
		op, e := m.GetDeclKind(body)
		astTestMust(t, e)
		if op != ast.ASTOpIte {
			t.Fatal("first entry did not wrap later entry", i, op)
		}
		other, e := m.GetArg(body, 2)
		astTestMust(t, e)
		inner, e := m.GetDeclKind(other)
		astTestMust(t, e)
		if i == 0 && inner != ast.ASTOpIte || i == 1 && inner != ast.ASTOpImplies {
			t.Fatal("reverse entry fallback", i, inner)
		}
	}
}
