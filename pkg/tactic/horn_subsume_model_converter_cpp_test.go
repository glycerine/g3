package tactic

// Source: src/test/horn_subsume_model_converter.cpp.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"testing"
)

func TestHornSubsumeSourceDriverProgressiveDefinitions(t *testing.T) {
	m, _, out, config := extensionFixture(t)
	c, err := NewHornSubsumeModelConverter(m, config)
	satOK(t, err)
	defer func() { satOK(t, c.Close()) }()
	one, err := m.ArithmeticIntegerNumeralText("1")
	satOK(t, err)
	astTestPin(t, m, one)
	sort, err := m.GetSort(one)
	satOK(t, err)
	boolSort, err := m.MkBoolSort()
	satOK(t, err)
	mkPred := func(name string) ast.ASTHandle {
		d, e := m.MkFuncDecl(astTestSymbol(t, m, name), []ast.ASTHandle{sort, sort}, boolSort, ast.ASTFuncDeclInfo{})
		satOK(t, e)
		astTestPin(t, m, d)
		return d
	}
	p, q, r := mkPred("p"), mkPred("q"), mkPred("r")
	numeral := func(n string) ast.ASTHandle {
		h, e := m.ArithmeticIntegerNumeralText(n)
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle {
		h, e := m.MkApp(d, args)
		satOK(t, e)
		astTestPin(t, m, h)
		return h
	}
	variable := func(i uint32) ast.ASTHandle { h, e := m.MkVar(i, sort); satOK(t, e); astTestPin(t, m, h); return h }
	v0, v1, v2 := variable(0), variable(1), variable(2)
	apply := func(stage int) {
		t.Helper()
		md, e := model.NewModel(m)
		satOK(t, e)
		satOK(t, out.Set(md.Core()))
		satOK(t, c.ApplyUnindexed(out))
		fi, e := md.Core().FunctionInterpretation(p)
		satOK(t, e)
		if fi == nil {
			t.Fatalf("stage %d: no p interpretation", stage)
		}
		fallback, e := fi.Else()
		satOK(t, e)
		if !m.IsFalse(fallback) {
			t.Fatalf("stage %d: p is not false", stage)
		}
	}
	satOK(t, c.InsertDefinition(p, app(q, one, numeral("2"))))
	apply(1)
	satOK(t, c.InsertDefinition(p, app(q, numeral("3"), numeral("5"))))
	apply(2)
	satOK(t, c.InsertDefinition(p, app(r, v0, v1)))
	apply(3)
	head := app(p, v0, v0)
	for i, body := range []ast.ASTHandle{app(q, v1, v2), app(q, v1, v0)} {
		pred, bodyRef, ok, e := c.MkHorn(head, body)
		satOK(t, e)
		if !ok || pred != p {
			t.Fatalf("mk_horn %d: %v", i, ok)
		}
		converted, e := m.RefGet(bodyRef)
		satOK(t, e)
		satOK(t, c.InsertDefinition(pred, converted))
		satOK(t, m.DelRef(&bodyRef))
		apply(i + 4)
	}
}
