package tactic

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func TestHardwareExprConverterTerms(t *testing.T) {
	budget := arena.NewBudget(1 << 26)
	sy, err := symbol.NewManager(symbol.Config{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	integerSort, err := m.MkFamilySort(family, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc, err := params.NewParamsContext(sy, q, params.ParamsConfig{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	p, err := pc.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	hwf, err := sy.Intern("hwf")
	if err != nil {
		t.Fatal(err)
	}
	key := "numeral"
	if err := pc.SetSymbolName(&p, &key, hwf); err != nil {
		t.Fatal(err)
	}
	depthKey := "max_depth"
	if err := pc.SetUIntName(&p, &depthKey, 0); err != nil {
		t.Fatal(err)
	}
	qm, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewHardwareEngine(&control.Reslimit{Limit: 1000}, qm, pc, &p)
	if err != nil {
		t.Fatal(err)
	}
	converter, err := NewHardwareExprConverter(m, engine)
	if err != nil {
		t.Fatal(err)
	}
	var n, d numeral.Int
	value, err := q.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, e := range []error{converter.Close(), engine.Close(), qm.Z.Del(&n), qm.Z.Del(&d), qm.CloseChecked(), pc.Del(&p), pc.Close(), q.Del(&value), m.Close(), q.Close(), sy.Close()} {
			if e != nil {
				t.Error(e)
			}
		}
		if budget.Used() != 0 {
			t.Error("retained owner bytes", budget.Used())
		}
	})
	for _, tc := range []struct {
		text         string
		integer      bool
		wantN, wantD string
	}{
		{"42", true, "42", "1"},
		{"18446744073709551617/3", false, "18446744073709551617", "3"},
	} {
		if err := q.SetString(&value, tc.text); err != nil {
			t.Fatal(err)
		}
		h, err := m.ArithmeticNumeral(&value, tc.integer)
		if err != nil {
			t.Fatal(err)
		}
		v, err := converter.InternalizeTerm(h, &n, &d)
		if err != nil || v != math.MaxUint32 {
			t.Fatalf("numeral %s: var %d, %v", tc.text, v, err)
		}
		gotN, err := qm.Z.String(&n)
		if err != nil {
			t.Fatal(err)
		}
		gotD, err := qm.Z.String(&d)
		if err != nil {
			t.Fatal(err)
		}
		if gotN != tc.wantN || gotD != tc.wantD {
			t.Fatalf("numeral %s: %s/%s", tc.text, gotN, gotD)
		}
	}
	name, err := sy.Intern("x")
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkConst(name, integerSort)
	if err != nil {
		t.Fatal(err)
	}
	first, err := converter.InternalizeTerm(x, &n, &d)
	if err != nil {
		t.Fatal(err)
	}
	second, err := converter.InternalizeTerm(x, &n, &d)
	if err != nil || first != second {
		t.Fatalf("variable reuse: %d %d %v", first, second, err)
	}
	gotN, err := qm.Z.String(&n)
	if err != nil {
		t.Fatal(err)
	}
	gotD, err := qm.Z.String(&d)
	if err != nil {
		t.Fatal(err)
	}
	if gotN != "1" || gotD != "1" {
		t.Fatalf("variable coefficient: %s/%s", gotN, gotD)
	}
	isVar, err := converter.IsVar(x)
	if err != nil || !isVar {
		t.Fatalf("mapped variable: %v %v", isVar, err)
	}
	add, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{x, x})
	if err != nil {
		t.Fatal(err)
	}
	// The source caches only expressions whose AST reference count exceeds one.
	if err := m.IncRef(add); err != nil {
		t.Fatal(err)
	}
	if err := m.IncRef(add); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.DecRef(add); err != nil {
			t.Error(err)
		}
		if err := m.DecRef(add); err != nil {
			t.Error(err)
		}
	}()
	sum, err := converter.InternalizeTerm(add, &n, &d)
	if err != nil || sum == math.MaxUint32 || sum == first {
		t.Fatalf("x+x must define a polynomial variable: %d, %v", sum, err)
	}
	gotN, err = qm.Z.String(&n)
	if err != nil {
		t.Fatal(err)
	}
	gotD, err = qm.Z.String(&d)
	if err != nil {
		t.Fatal(err)
	}
	if gotN != "1" || gotD != "1" {
		t.Fatalf("sum coefficient: %s/%s", gotN, gotD)
	}
	if again, err := converter.InternalizeTerm(add, &n, &d); err != nil || again != sum {
		t.Fatalf("cached x+x: %d vs %d, %v", again, sum, err)
	}
	if rc, err := m.GetRefCount(add); err != nil || rc != 3 {
		t.Fatalf("cached AST key reference: %d, %v", rc, err)
	}
	if err := q.SetString(&value, "1/2"); err != nil {
		t.Fatal(err)
	}
	half, err := m.ArithmeticNumeral(&value, false)
	if err != nil {
		t.Fatal(err)
	}
	constantAdd, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{half, half})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := converter.InternalizeTerm(constantAdd, &n, &d); err != nil || v != math.MaxUint32 {
		t.Fatalf("half plus half: %d, %v", v, err)
	}
	gotN, err = qm.Z.String(&n)
	if err != nil {
		t.Fatal(err)
	}
	gotD, err = qm.Z.String(&d)
	if err != nil {
		t.Fatal(err)
	}
	if gotN != "1" || gotD != "1" {
		t.Fatalf("constant sum: %s/%s", gotN, gotD)
	}
	realSort, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	realName, err := sy.Intern("y")
	if err != nil {
		t.Fatal(err)
	}
	y, err := m.MkConst(realName, realSort)
	if err != nil {
		t.Fatal(err)
	}
	mixedAdd, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{y, half})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := converter.InternalizeTerm(mixedAdd, &n, &d); err != nil || v == math.MaxUint32 {
		t.Fatalf("y plus half: %d, %v", v, err)
	}
	gotN, err = qm.Z.String(&n)
	if err != nil {
		t.Fatal(err)
	}
	gotD, err = qm.Z.String(&d)
	if err != nil {
		t.Fatal(err)
	}
	if gotN != "1" || gotD != "2" {
		t.Fatalf("mixed sum: %s/%s", gotN, gotD)
	}
	mul, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{x, x})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := converter.InternalizeTerm(mul, &n, &d); err != nil || v == math.MaxUint32 || v == first {
		t.Fatalf("x*x monomial: %d, %v", v, err)
	}
	if err := q.SetString(&value, "2"); err != nil {
		t.Fatal(err)
	}
	two, err := m.ArithmeticNumeral(&value, true)
	if err != nil {
		t.Fatal(err)
	}
	power, err := m.MkFamilyApp(family, 18, nil, []ast.ASTHandle{x, two})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := converter.InternalizeTerm(power, &n, &d); err != nil || v == math.MaxUint32 {
		t.Fatalf("x^2 monomial: %d, %v", v, err)
	}
	scaled, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{two, power})
	if err != nil {
		t.Fatal(err)
	}
	beforeCoefficient := m.ResourceLimit().Count()
	if v, err := converter.InternalizeTerm(scaled, &n, &d); err != nil || v == math.MaxUint32 {
		t.Fatalf("2*x^2: %d, %v", v, err)
	}
	if charged := m.ResourceLimit().Count() - beforeCoefficient; charged != 2 {
		t.Fatal("coefficient must not consume a process checkpoint", charged)
	}
	gotN, err = qm.Z.String(&n)
	if err != nil {
		t.Fatal(err)
	}
	gotD, err = qm.Z.String(&d)
	if err != nil {
		t.Fatal(err)
	}
	if gotN != "2" || gotD != "1" {
		t.Fatalf("scaled monomial coefficient: %s/%s", gotN, gotD)
	}
	if err := q.SetString(&value, "-1"); err != nil {
		t.Fatal(err)
	}
	minusOne, err := m.ArithmeticNumeral(&value, true)
	if err != nil {
		t.Fatal(err)
	}
	negativePower, err := m.MkFamilyApp(family, 18, nil, []ast.ASTHandle{x, minusOne})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := converter.InternalizeTerm(negativePower, &n, &d); err != nil || v == math.MaxUint32 {
		t.Fatalf("negative exponent must be an atom: %d, %v", v, err)
	}
	if yes, err := converter.IsVar(negativePower); err != nil || !yes {
		t.Fatalf("negative exponent atom mapping: %v, %v", yes, err)
	}
	if err := q.SetString(&value, "0"); err != nil {
		t.Fatal(err)
	}
	zero, err := m.ArithmeticNumeral(&value, true)
	if err != nil {
		t.Fatal(err)
	}
	zeroPower, err := m.MkFamilyApp(family, 18, nil, []ast.ASTHandle{x, zero})
	if err != nil {
		t.Fatal(err)
	}
	zeroPowerVar, err := converter.InternalizeTerm(zeroPower, &n, &d)
	if err != nil || zeroPowerVar == math.MaxUint32 || zeroPowerVar == first {
		t.Fatalf("zero power monomial definition: %d, %v", zeroPowerVar, err)
	}
	if yes, err := converter.IsVar(zeroPower); err != nil || yes {
		t.Fatalf("zero power should map through a monomial definition: %v, %v", yes, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "1" {
		t.Fatalf("zero power numerator: %s, %v", got, err)
	}
	zeroPowerProduct, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{zeroPower, y})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := converter.InternalizeTerm(zeroPowerProduct, &n, &d); err != nil || got == math.MaxUint32 || got == zeroPowerVar {
		t.Fatalf("zero power degree in a product: %d, %v", got, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "1" {
		t.Fatalf("zero power product numerator: %s, %v", got, err)
	}
	zeroProduct, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{zero, x, y})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := converter.InternalizeTerm(zeroProduct, &n, &d); err != nil || got == math.MaxUint32 {
		t.Fatalf("n-ary zero product: %d, %v", got, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "0" {
		t.Fatalf("n-ary zero product numerator: %s, %v", got, err)
	}
	if got, err := qm.Z.String(&d); err != nil || got != "1" {
		t.Fatalf("n-ary zero product denominator: %s, %v", got, err)
	}
	for _, tc := range []struct {
		name string
		op   int32
		args []ast.ASTHandle
		want error
	}{
		{"sub", 7, []ast.ASTHandle{x, x}, ErrExprConverterNonSimplified},
		{"uminus", 7, []ast.ASTHandle{x}, ErrExprConverterNonSimplified},
		{"div", 10, []ast.ASTHandle{y, y}, ErrExprConverterPurifier},
		{"idiv", 11, []ast.ASTHandle{x, x}, ErrExprConverterPurifier},
		{"rem", 12, []ast.ASTHandle{x, x}, ErrExprConverterPurifier},
		{"mod", 13, []ast.ASTHandle{x, x}, ErrExprConverterPurifier},
		{"to-int", 15, []ast.ASTHandle{y}, ErrExprConverterPurifier},
		{"sin", 19, []ast.ASTHandle{y}, ErrExprConverterTranscendental},
		{"cos", 20, []ast.ASTHandle{y}, ErrExprConverterTranscendental},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, err := m.MkFamilyApp(family, tc.op, nil, tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if err := qm.Z.SetInt32(&n, 7); err != nil {
				t.Fatal(err)
			}
			if err := qm.Z.SetInt32(&d, 11); err != nil {
				t.Fatal(err)
			}
			_, err = converter.InternalizeTerm(h, &n, &d)
			var source z3exception.DefaultException
			if !errors.Is(err, tc.want) || !errors.As(err, &source) || source.Message() != tc.want.Error() {
				t.Fatalf("source refusal: %v, want %v", err, tc.want)
			}
			gotN, err := qm.Z.String(&n)
			if err != nil {
				t.Fatal(err)
			}
			gotD, err := qm.Z.String(&d)
			if err != nil {
				t.Fatal(err)
			}
			if gotN != "7" || gotD != "11" {
				t.Fatalf("refusal mutated outputs: %s/%s", gotN, gotD)
			}
		})
	}
	beforeCancel := m.Limit.Count()
	m.Limit.Cancel()
	if _, err := converter.InternalizeTerm(add, &n, &d); !errors.Is(err, ErrExprConverterCanceled) || m.Limit.Count() != beforeCancel+1 {
		t.Fatalf("checkpoint before cached result: %v, steps %d", err, m.Limit.Count())
	}
	m.Limit.ResetCancel()
	letwo, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{x, two})
	if err != nil {
		t.Fatal(err)
	}
	gehalf, err := m.MkFamilyApp(family, 3, nil, []ast.ASTHandle{y, half})
	if err != nil {
		t.Fatal(err)
	}
	notGE, err := m.MkNot(gehalf)
	if err != nil {
		t.Fatal(err)
	}
	clause, err := m.MkOr(letwo, notGE)
	if err != nil {
		t.Fatal(err)
	}
	goal, err := goaltactic.NewGoal(m, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := goal.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := goal.Assert(clause); err != nil {
		t.Fatal(err)
	}
	if err := converter.InternalizeGoal(goal); err != nil {
		t.Fatal(err)
	}
	shell, err := engine.Shell()
	if err != nil {
		t.Fatal(err)
	}
	var display bytes.Buffer
	if err := shell.DisplayConstraints(&display, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(display.String(), "x0 <= 2 or ") || !strings.Contains(display.String(), " < 1/2") {
		t.Fatalf("goal disjunction: %q", display.String())
	}
	if err := q.SetString(&value, "-2"); err != nil {
		t.Fatal(err)
	}
	minusTwo, err := m.ArithmeticNumeral(&value, true)
	if err != nil {
		t.Fatal(err)
	}
	negativeX, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{minusTwo, x})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetString(&value, "4"); err != nil {
		t.Fatal(err)
	}
	four, err := m.ArithmeticNumeral(&value, true)
	if err != nil {
		t.Fatal(err)
	}
	negativeLE, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{negativeX, four})
	if err != nil {
		t.Fatal(err)
	}
	var inequalityTrace bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&inequalityTrace)
	tags.Enable("subpaving_tactic")
	if err := converter.BindTrace(tags); err != nil {
		t.Fatal(err)
	}
	if err := converter.InternalizeClause(negativeLE); err != nil {
		t.Fatal(err)
	}
	if tags.ShouldTrace("subpaving_tactic") {
		want := "-------- [subpaving_tactic] mk_ineq src/math/subpaving/tactic/subpaving_tactic.cpp:158 ---------\n0 -2 1 0\n------------------------------------------------\n"
		if got := inequalityTrace.String(); got != want {
			t.Fatalf("source inequality trace: %q", got)
		}
	} else if inequalityTrace.Len() != 0 {
		t.Fatalf("erased inequality trace: %q", inequalityTrace.String())
	}
	display.Reset()
	if err := shell.DisplayConstraints(&display, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(display.String(), "-2 <= x0") {
		t.Fatalf("negative coefficient reverses direction: %q", display.String())
	}
	doubleNot, err := m.MkNot(notGE)
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.InternalizeClause(doubleNot); err != nil {
		t.Fatal(err)
	}
	display.Reset()
	if err := shell.DisplayConstraints(&display, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(display.String(), "1/2 <= x2") {
		t.Fatalf("double negation restores closed lower bound: %q", display.String())
	}
	badAtom, err := m.MkEq(x, two)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := converter.InternalizeInequality(badAtom); !errors.Is(err, ErrHardwareUnsupportedAtom) {
		t.Fatalf("non-inequality atom: %v", err)
	}
	badRHS, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{x, x})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := converter.InternalizeInequality(badRHS); !errors.Is(err, ErrHardwareArithmeticLHS) {
		t.Fatalf("non-numeral right side: %v", err)
	}
	badGoal, err := goaltactic.NewGoal(m, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := badGoal.Assert(badAtom); err != nil {
		t.Fatal(err)
	}
	if err := converter.SearchGoal(badGoal); !errors.Is(err, ErrHardwareUnsupportedAtom) {
		t.Fatalf("invalid goal should stop before search: %v", err)
	}
	if err := badGoal.Close(); err != nil {
		t.Fatal(err)
	}
	if err := converter.SearchGoal(goal); err != nil {
		t.Fatal(err)
	}
	stats := statistics.New()
	if err := shell.CollectStatistics(stats); err != nil {
		t.Fatal(err)
	}
	foundRoot := false
	for i := 0; i < stats.Size(); i++ {
		if stats.Key(i) == "nodes" && stats.UintValue(i) == 1 {
			foundRoot = true
		}
	}
	if !foundRoot {
		t.Fatalf("goal search did not process the bounded root: %v", stats)
	}
	if err := converter.Close(); err != nil {
		t.Fatal(err)
	}
	if rc, err := m.GetRefCount(add); err != nil || rc != 2 {
		t.Fatalf("released cached AST key: %d, %v", rc, err)
	}
	if _, err := converter.InternalizeTerm(x, &n, &d); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed converter: %v", err)
	}
}
