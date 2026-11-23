package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestExistentialDefinitions(t *testing.T) {
	if ast.ASTSourceDebug || diagnostic.TraceCompiled() {
		t.Skip("the optional rewrite is disabled in tracing builds")
	}
	const declarations = "(declare-fun P (Int) Bool)(declare-fun f (Int) Int)(declare-fun R (Int Bool) Bool)"
	for _, tc := range []struct{ name, input, expected string }{
		{"definition", "(exists ((x Int)) (and (= 8 x) (P x)))", "(P 8)"},
		{"nested-conjunction", "(exists ((x Int)) (and (P x) (and (P (f x)) (= x 4))))", "(and (P 4) (P (f 4)))"},
		{"contradiction", "(exists ((x Int)) (and (= x 5) (= x 6)))", "(= 5 6)"},
		{"outer-binding", "(forall ((y Int)) (exists ((x Int)) (and (= x (f y)) (P x))))", "(forall ((y Int)) (P (f y)))"},
		{"shadowing", "(forall ((x Int)) (exists ((x Int)) (and (= x 5) (P x))))", "(forall ((u Int)) (P 5))"},
		{"two-sorts", "(exists ((x Int) (b Bool)) (and (= x 8) (and (= b true) (R x b))))", "(R 8 true)"},
		{"occurs-check", "(exists ((x Int)) (and (= x (f x)) (P x)))", ""},
		{"no-definition", "(exists ((x Int)) (not (P x)))", ""},
		{"universal-equality", "(forall ((x Int)) (and (= x 5) (P x)))", ""},
		{"annotation", "(exists ((x Int)) (! (and (= x 5) (P x)) :pattern ((P x))))", ""},
		{"no-pattern", "(exists ((x Int)) (! (and (= x 5) (P x)) :no-pattern (P x)))", ""},
		{"inside-annotation", "(forall ((y Int)) (! (exists ((x Int)) (and (= x (f y)) (P x))) :pattern ((f y))))", ""},
		{"different-weights", "(exists ((x Int)) (! (exists ((z Bool)) (! (and (= x 8) (R x z)) :weight 7)) :weight 2))", ""},
		{"named-inner-quantifier", "(exists ((x Int)) (exists ((z Bool)) (! (and (= x 8) (R x z)) :qid named)))", ""},
		{"skolem-id", "(exists ((x Int)) (exists ((z Bool)) (! (and (= x 8) (R x z)) :skolemid sid)))", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewSMTContext()
			astTestMust(t, err)
			defer func() { astTestMust(t, c.Close()) }()
			m, err := c.Manager()
			astTestMust(t, err)
			expected := tc.expected
			if expected == "" {
				expected = "true"
			}
			input, err := c.ParseSMT2String(declarations+"(assert (and "+tc.input+" "+expected+"))", SMT2Bindings{})
			astTestMust(t, err)
			defer func() { astTestMust(t, m.DelRef(&input)) }()
			pair, err := m.RefGet(input)
			astTestMust(t, err)
			original, err := m.GetArg(pair, 0)
			astTestMust(t, err)
			want, err := m.GetArg(pair, 1)
			astTestMust(t, err)
			result, err := c.EliminateExistentialDefinitions(original)
			astTestMust(t, err)
			defer func() { astTestMust(t, m.DelRef(&result)) }()
			got, err := m.RefGet(result)
			astTestMust(t, err)
			if tc.expected == "" {
				if got != original {
					t.Fatalf("untouched formula changed: %v to %v", original, got)
				}
				return
			}
			if got == original {
				t.Fatal("definition was retained")
			}
			// The parser assigns source locations as quantifier IDs. Compare
			// declaration sorts, binder kind and the de Bruijn body.
			for {
				k, err := m.GetKind(want)
				astTestMust(t, err)
				if k != ast.ASTQuantifier {
					break
				}
				gk, err := m.GetKind(got)
				astTestMust(t, err)
				if gk != k {
					t.Fatalf("expected quantifier, got %v", gk)
				}
				wa, err := m.IsForall(want)
				astTestMust(t, err)
				ga, err := m.IsForall(got)
				astTestMust(t, err)
				wn, err := m.GetNumDeclarations(want)
				astTestMust(t, err)
				gn, err := m.GetNumDeclarations(got)
				astTestMust(t, err)
				if wa != ga || wn != gn {
					t.Fatal("unexpected binder")
				}
				for i := uint32(0); i < wn; i++ {
					ws, err := m.GetDeclarationSort(want, i)
					astTestMust(t, err)
					gs, err := m.GetDeclarationSort(got, i)
					astTestMust(t, err)
					if ws != gs {
						t.Fatal("binder sort changed")
					}
				}
				want, err = m.GetQuantifierBody(want)
				astTestMust(t, err)
				got, err = m.GetQuantifierBody(got)
				astTestMust(t, err)
			}
			if got != want {
				actual, e := c.BenchmarkSMT2(got, APIBenchmarkConfig{})
				astTestMust(t, e)
				t.Fatalf("expected %s; got %s", tc.expected, actual)
			}
		})
	}
}

func TestExistentialDefinitionsOwnsResultAndHonorsGuards(t *testing.T) {
	c, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, c.Close()) }()
	m, err := c.Manager()
	astTestMust(t, err)
	input, err := c.ParseSMT2String("(declare-const a Bool)(assert (exists ((x Int)) (and (= x 4) a)))", SMT2Bindings{})
	astTestMust(t, err)
	original, err := m.RefGet(input)
	astTestMust(t, err)
	check := func() {
		t.Helper()
		r, err := c.EliminateExistentialDefinitions(original)
		astTestMust(t, err)
		h, err := m.RefGet(r)
		astTestMust(t, err)
		astTestMust(t, m.DelRef(&r))
		if h != original {
			t.Fatal("guarded formula changed")
		}
	}
	astTestMust(t, m.Limit.Push(12))
	check()
	m.Limit.Pop()
	m.Limit.Cancel()
	check()
	m.Limit.DecCancel()
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	check()
	astTestMust(t, m.ToggleProofMode(ast.ASTProofDisabled))
	n, err := m.ArithmeticIntegerNumeralText("4")
	astTestMust(t, err)
	if _, err = c.EliminateExistentialDefinitions(n); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("integer accepted: %v", err)
	}
	if _, err = c.EliminateExistentialDefinitions(ast.ASTHandle{}); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatalf("null result: %v", err)
	}
	r, err := c.EliminateExistentialDefinitions(original)
	astTestMust(t, err)
	astTestMust(t, m.DelRef(&input))
	defer func() { astTestMust(t, m.DelRef(&r)) }()
	h, err := m.RefGet(r)
	astTestMust(t, err)
	astTestMust(t, c.Assert(h))
	v, err := c.CheckSat()
	astTestMust(t, err)
	if v != lbool.True {
		t.Fatalf("surviving result check returned %v", v)
	}
}

func TestExistentialDefinitionsMergesUnnamedBinders(t *testing.T) {
	if ast.ASTSourceDebug || diagnostic.TraceCompiled() {
		t.Skip("the optional rewrite is disabled in tracing builds")
	}
	f := newSMTAPIFixture(t, "ALL", nil)
	m := f.m
	x := f.arithmeticVariable("merged-x", true)
	bs := f.pin(m.MkBoolSort())
	is := f.pin(m.GetSort(x))
	b := f.pin(m.MkConst(astTestSymbol(t, m, "merged-b"), bs))
	p := f.pin(m.MkFuncDecl(astTestSymbol(t, m, "merged-p"), []ast.ASTHandle{is, bs}, bs, ast.ASTFuncDeclInfo{}))
	five := f.pin(m.ArithmeticIntegerNumeralText("5"))
	fact := f.pin(m.MkEq(x, five))
	pred := f.pin(m.MkApp(p, []ast.ASTHandle{x, b}))
	body := f.pin(m.MkAnd(fact, pred))
	inner := f.pin(f.c.ConstantQuantifier(ConstantQuantifierSpec{Bound: []ast.ASTHandle{b}, Body: body, Weight: 1}))
	original := f.pin(f.c.ConstantQuantifier(ConstantQuantifierSpec{Bound: []ast.ASTHandle{x}, Body: inner, Weight: 1, QID: "keep-outer"}))
	out, err := f.c.EliminateExistentialDefinitions(original)
	astTestMust(t, err)
	defer func() { astTestMust(t, m.DelRef(&out)) }()
	next := f.pin(m.RefGet(out))
	count, err := m.GetNumDeclarations(next)
	astTestMust(t, err)
	if count != 1 {
		t.Fatalf("retained %d binders", count)
	}
	qid, _, err := f.c.QuantifierIdentifiers(next)
	astTestMust(t, err)
	if qid != "keep-outer" {
		t.Fatalf("outer quantifier identifier %q", qid)
	}
	name, err := f.c.QuantifierBoundName(next, 0)
	astTestMust(t, err)
	if name != "merged-b" {
		t.Fatalf("retained binder %q", name)
	}
	remainingSort := f.pin(m.GetDeclarationSort(next, 0))
	v := f.pin(m.MkVar(0, bs))
	expected := f.pin(m.MkApp(p, []ast.ASTHandle{five, v}))
	got := f.pin(m.GetQuantifierBody(next))
	if remainingSort != bs || got != expected {
		t.Fatalf("merged sorts or de Bruijn indices changed, body %v", got)
	}
}
