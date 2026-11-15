package model_test

// Source: src/ast/bv_decl_plugin.h:271-285;
// src/model/model_evaluator.cpp:121-140,244-253,258-276.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestModelEvaluatorBVUnspecifiedFunctionCompletion(t *testing.T) {
	m, md, ev, c, p, _ := evaluatorFixture(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	s, e := m.BitVectorSort(8)
	astTestMust(t, e)
	n, e := c.NumeralsField.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, c.NumeralsField.Del(&n)) }()
	zero, e := m.BitVectorNumeral(&n, 8)
	astTestMust(t, e)
	astTestPin(t, m, zero)
	astTestMust(t, c.NumeralsField.SetInt64(&n, 7))
	seven, e := m.BitVectorNumeral(&n, 8)
	astTestMust(t, e)
	astTestPin(t, m, seven)
	out := astTestPin(t, m, ast.ASTHandle{})
	for kind := int32(12); kind <= 16; kind++ {
		x, e := m.MkConst(astTestSymbol(t, m, fmt.Sprintf("bv-unspecified-x-%d", kind)), s)
		astTestMust(t, e)
		astTestPin(t, m, x)
		app, e := m.MkFamilyApp(fid, kind, nil, []ast.ASTHandle{x})
		astTestMust(t, e)
		astTestPin(t, m, app)
		d, e := m.GetDecl(app)
		astTestMust(t, e)
		key := "completion"
		astTestMust(t, c.SetBoolName(p, &key, false))
		astTestMust(t, ev.Reset(p))
		astTestMust(t, ev.Evaluate(app, out))
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if got != app {
			t.Fatal("disabled completion changed unspecified function", kind, got)
		}
		astTestMust(t, c.SetBoolName(p, &key, true))
		astTestMust(t, ev.Reset(p))
		astTestMust(t, ev.Evaluate(app, out))
		got, e = m.RefGet(out)
		astTestMust(t, e)
		if got != zero {
			t.Fatal("BV function completion", kind, got, zero)
		}
		fi, e := md.Core().FunctionInterpretation(d)
		astTestMust(t, e)
		if fi == nil {
			t.Fatal("completion did not register interpretation", kind)
		}
		// Existing interpretations take priority over generating another default.
		astTestMust(t, fi.SetElse(seven))
		astTestMust(t, c.SetBoolName(p, &key, false))
		astTestMust(t, ev.Reset(p))
		astTestMust(t, ev.Evaluate(app, out))
		got, e = m.RefGet(out)
		astTestMust(t, e)
		if got != seven {
			t.Fatal("registered theory interpretation ignored", kind, got, seven)
		}
	}
}

func TestModelEvaluatorArithmeticUnspecifiedRegisteredInterpretations(t *testing.T) {
	m, md, ev, c, _, _ := evaluatorFixture(t)
	fid, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	s, e := m.MkFamilySort(fid, 0, nil)
	astTestMust(t, e)
	x, e := m.MkConst(astTestSymbol(t, m, "arith-unspecified-x"), s)
	astTestMust(t, e)
	astTestPin(t, m, x)
	n, e := c.NumeralsField.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, c.NumeralsField.Del(&n)) }()
	astTestMust(t, c.NumeralsField.SetInt64(&n, 7))
	seven, e := m.ArithmeticNumeral(&n, false)
	astTestMust(t, e)
	astTestPin(t, m, seven)
	out := astTestPin(t, m, ast.ASTHandle{})
	astTestMust(t, ev.SetModelCompletion(false))
	for kind := int32(33); kind <= 35; kind++ {
		args := []ast.ASTHandle{x}
		if kind == 33 {
			args = append(args, x)
		}
		app, e := m.MkFamilyApp(fid, kind, nil, args)
		astTestMust(t, e)
		astTestPin(t, m, app)
		d, e := m.GetDecl(app)
		astTestMust(t, e)
		fi := modelTestFunction(t, m, uint32(len(args)))
		astTestMust(t, fi.SetElse(seven))
		astTestMust(t, md.Core().RegisterFunction(d, fi))
		astTestMust(t, ev.Reset(nil))
		astTestMust(t, ev.Evaluate(app, out))
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if got != seven {
			t.Fatal("arithmetic unspecified interpretation", kind, got, seven)
		}
	}
}

func TestModelEvaluatorBVUnspecifiedConstantLookupAndCompletion(t *testing.T) {
	m, md, ev, c, _, _ := evaluatorFixture(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	s, e := m.BitVectorSort(8)
	astTestMust(t, e)
	n, e := c.NumeralsField.NewValue()
	astTestMust(t, e)
	defer func() { astTestMust(t, c.NumeralsField.Del(&n)) }()
	zero, e := m.BitVectorNumeral(&n, 8)
	astTestMust(t, e)
	astTestPin(t, m, zero)
	astTestMust(t, c.NumeralsField.SetInt64(&n, 7))
	seven, e := m.BitVectorNumeral(&n, 8)
	astTestMust(t, e)
	astTestPin(t, m, seven)
	out := astTestPin(t, m, ast.ASTHandle{})
	for kind := int32(12); kind <= 16; kind++ {
		// Raw declarations exercise the source hook's arity-independent identity
		// predicate and reduce_app's num==0 branch, without changing plugin caches.
		info, e := m.NewFuncDeclInfo(fid, kind, nil, ast.ASTFuncFlags{}, false)
		astTestMust(t, e)
		d, e := m.MkFuncDecl(astTestSymbol(t, m, "bv-unspecified-constant"), nil, s, info)
		astTestMust(t, e)
		astTestMust(t, m.DelFuncDeclInfo(&info))
		astTestPin(t, m, d)
		app, e := m.MkApp(d, nil)
		astTestMust(t, e)
		astTestPin(t, m, app)
		astTestMust(t, ev.Reset(nil))
		astTestMust(t, ev.SetModelCompletion(true))
		astTestMust(t, ev.Evaluate(app, out))
		got, e := m.RefGet(out)
		astTestMust(t, e)
		registered, e := md.Core().ConstantInterpretation(d)
		astTestMust(t, e)
		if got != zero || registered != zero {
			t.Fatal("constant completion", kind, got, registered)
		}
		astTestMust(t, md.Core().RegisterConstant(d, seven))
		astTestMust(t, ev.Reset(nil))
		astTestMust(t, ev.SetModelCompletion(false))
		astTestMust(t, ev.Evaluate(app, out))
		got, e = m.RefGet(out)
		astTestMust(t, e)
		if got != seven {
			t.Fatal("constant interpretation priority", kind, got)
		}
	}
}

func TestModelEvaluatorBVUnspecifiedCompletionEveryReservation(t *testing.T) {
	for ordinal := int64(0); ordinal < 128; ordinal++ {
		success := false
		t.Run(fmt.Sprint(ordinal), func(t *testing.T) {
			m, md, ev, c, _, _ := evaluatorFixture(t)
			_, e := m.RegisterArithmeticPlugin()
			astTestMust(t, e)
			fid, e := m.RegisterBitVectorPlugin()
			astTestMust(t, e)
			n, e := c.NumeralsField.NewValue()
			astTestMust(t, e)
			zero, e := m.BitVectorNumeral(&n, 8)
			astTestMust(t, e)
			astTestPin(t, m, zero)
			astTestMust(t, c.NumeralsField.Del(&n))
			app, e := m.MkFamilyApp(fid, 13, nil, []ast.ASTHandle{zero})
			astTestMust(t, e)
			astTestPin(t, m, app)
			d, e := m.GetDecl(app)
			astTestMust(t, e)
			out := astTestPin(t, m, ast.ASTHandle{})
			astTestMust(t, ev.SetModelCompletion(true))
			m.Budget().FailAfter(ordinal)
			body := ev.Evaluate(app, out)
			m.Budget().FailAfter(-1)
			if body == nil {
				got, e := m.RefGet(out)
				astTestMust(t, e)
				fi, e := md.Core().FunctionInterpretation(d)
				astTestMust(t, e)
				if got != zero || fi == nil {
					t.Fatal("completed result/registration", got, fi)
				}
				success = true
			} else if !errors.Is(body, arena.ErrInjectedFailure) {
				t.Fatal("unexpected refusal", body)
			}
			// The fixture checks all owners' budget after explicit evaluator close.
			astTestMust(t, ev.Close())
		})
		if success {
			if ordinal == 0 {
				t.Fatal("missing completion allocations")
			}
			t.Log("first complete completion reservation", ordinal)
			return
		}
	}
	t.Fatal("completion reservation horizon exhausted")
}

// AUDIT.md model_01-F2: when the BV rewriter fails, reduce_app calls
// evaluate_partial_theory_func for the non-builtin family, even without
// model completion: a partial interpretation gets the else value
// get_some_value(range), and the interpretation is instantiated.
func TestModelEvaluatorBVUnspecifiedPartialInterpretationWithoutCompletion(t *testing.T) {
	m, md, ev, c, p, _ := evaluatorFixture(t)
	_, e := m.RegisterArithmeticPlugin()
	astTestMust(t, e)
	fid, e := m.RegisterBitVectorPlugin()
	astTestMust(t, e)
	s, e := m.BitVectorSort(8)
	astTestMust(t, e)
	numeral := func(v int64) ast.ASTHandle {
		n, e := c.NumeralsField.NewValue()
		astTestMust(t, e)
		defer func() { astTestMust(t, c.NumeralsField.Del(&n)) }()
		astTestMust(t, c.NumeralsField.SetInt64(&n, v))
		h, e := m.BitVectorNumeral(&n, 8)
		astTestMust(t, e)
		astTestPin(t, m, h)
		return h
	}
	zero, one, three, seven := numeral(0), numeral(1), numeral(3), numeral(7)
	x, e := m.MkConst(astTestSymbol(t, m, "bv-partial-x"), s)
	astTestMust(t, e)
	astTestPin(t, m, x)
	app, e := m.MkFamilyApp(fid, 13, nil, []ast.ASTHandle{x})
	astTestMust(t, e)
	astTestPin(t, m, app)
	d, e := m.GetDecl(app)
	astTestMust(t, e)
	fi := modelTestFunction(t, m, 1)
	astTestMust(t, fi.InsertNewEntry([]ast.ASTHandle{one}, three))
	astTestMust(t, md.Core().RegisterFunction(d, fi))
	key := "completion"
	astTestMust(t, c.SetBoolName(p, &key, false))
	astTestMust(t, ev.Reset(p))
	out := astTestPin(t, m, ast.ASTHandle{})
	for _, row := range []struct{ arg, want ast.ASTHandle }{{one, three}, {seven, zero}} {
		ground, e := m.MkFamilyApp(fid, 13, nil, []ast.ASTHandle{row.arg})
		astTestMust(t, e)
		astTestPin(t, m, ground)
		astTestMust(t, ev.Evaluate(ground, out))
		got, e := m.RefGet(out)
		astTestMust(t, e)
		if got != row.want {
			t.Fatal("partial bvudiv0 interpretation", row.arg, got, row.want)
		}
	}
	registered, e := md.Core().FunctionInterpretation(d)
	astTestMust(t, e)
	elseValue, e := registered.Else()
	astTestMust(t, e)
	if elseValue != zero {
		t.Fatal("partial interpretation else", elseValue, zero)
	}
}
