package tactic

import (
	"bytes"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/subpaving"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestRationalExprConverterExactLeafGoalAndCleanup(t *testing.T) {
	b := arena.NewBudget(1 << 27)
	integers := numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}
	qm, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: integers})
	if err != nil {
		t.Fatal(err)
	}
	sy, err := symbol.NewManager(symbol.Config{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: integers})
	if err != nil {
		t.Fatal(err)
	}
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	sort, err := m.MkFamilySort(family, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc, err := params.NewParamsContext(sy, q, params.ParamsConfig{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	p, err := pc.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	depth := "max_depth"
	if err := pc.SetUIntName(&p, &depth, 0); err != nil {
		t.Fatal(err)
	}
	engine, err := NewRationalEngine(&control.Reslimit{Limit: 1000}, qm, pc, &p)
	if err != nil {
		t.Fatal(err)
	}
	converter, err := NewRationalExprConverter(m, engine, qm)
	if err != nil {
		t.Fatal(err)
	}
	goal, err := goaltactic.NewGoal(m, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	value, err := q.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	var n, d numeral.Int
	t.Cleanup(func() {
		for _, e := range []error{goal.Close(), converter.Close(), engine.Close(), qm.Z.Del(&d), qm.Z.Del(&n), pc.Del(&p), pc.Close(), q.Del(&value), m.Close(), q.Close(), sy.Close(), qm.CloseChecked()} {
			if e != nil {
				t.Error(e)
			}
		}
		if b.Used() != 0 {
			t.Error("rational tactic owner roots", b.Used())
		}
	})
	xName, err := sy.Intern("x")
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkConst(xName, sort)
	if err != nil {
		t.Fatal(err)
	}
	variable, err := converter.InternalizeTerm(x, &n, &d)
	if err != nil || variable == math.MaxUint32 {
		t.Fatal("arithmetic leaf", variable, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "1" {
		t.Fatal("variable coefficient", got, err)
	}
	if err := q.SetString(&value, "3/2"); err != nil {
		t.Fatal(err)
	}
	threshold, err := m.ArithmeticNumeral(&value, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := converter.InternalizeTerm(threshold, &n, &d); err != nil || got != math.MaxUint32 {
		t.Fatal("exact rational numeral", got, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "3" {
		t.Fatal("numerator", got, err)
	}
	if got, err := qm.Z.String(&d); err != nil || got != "2" {
		t.Fatal("denominator", got, err)
	}
	if err := q.SetString(&value, "1/2"); err != nil {
		t.Fatal(err)
	}
	half, err := m.ArithmeticNumeral(&value, false)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{x, half})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := m.IncRef(sum); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		for i := 0; i < 2; i++ {
			if err := m.DecRef(sum); err != nil {
				t.Error(err)
			}
		}
	}()
	sumVar, err := converter.InternalizeTerm(sum, &n, &d)
	if err != nil || sumVar == variable || sumVar == math.MaxUint32 {
		t.Fatal("exact rational sum definition", sumVar, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "1" {
		t.Fatal("sum numerator", got, err)
	}
	if got, err := qm.Z.String(&d); err != nil || got != "2" {
		t.Fatal("sum denominator", got, err)
	}
	if again, err := converter.InternalizeTerm(sum, &n, &d); err != nil || again != sumVar {
		t.Fatal("shared rational sum cache", again, err)
	}
	product, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{x, x})
	if err != nil {
		t.Fatal(err)
	}
	productVar, err := converter.InternalizeTerm(product, &n, &d)
	if err != nil || productVar == math.MaxUint32 || productVar == variable {
		t.Fatal("rational monomial", productVar, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "1" {
		t.Fatal("product numerator", got, err)
	}
	scaled, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{threshold, product})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := converter.InternalizeTerm(scaled, &n, &d); err != nil || got == math.MaxUint32 || got == variable {
		t.Fatal("scaled rational monomial", got, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "3" {
		t.Fatal("scaled product numerator", got, err)
	}
	if got, err := qm.Z.String(&d); err != nil || got != "2" {
		t.Fatal("scaled product denominator", got, err)
	}
	if err := q.SetString(&value, "2"); err != nil {
		t.Fatal(err)
	}
	two, err := m.ArithmeticNumeral(&value, false)
	if err != nil {
		t.Fatal(err)
	}
	power, err := m.MkFamilyApp(family, 18, nil, []ast.ASTHandle{x, two})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := converter.InternalizeTerm(power, &n, &d); err != nil || got == math.MaxUint32 || got == variable {
		t.Fatal("rational power definition", got, err)
	}
	if err := q.SetString(&value, "0"); err != nil {
		t.Fatal(err)
	}
	zero, err := m.ArithmeticNumeral(&value, false)
	if err != nil {
		t.Fatal(err)
	}
	zeroPower, err := m.MkFamilyApp(family, 18, nil, []ast.ASTHandle{x, zero})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.IncRef(zeroPower); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.DecRef(zeroPower); err != nil {
			t.Error(err)
		}
	}()
	zeroPowerVar, err := converter.InternalizeTerm(zeroPower, &n, &d)
	if err != nil || zeroPowerVar == math.MaxUint32 || zeroPowerVar == variable {
		t.Fatal("zero power must remain an atomic variable", zeroPowerVar, err)
	}
	if again, err := converter.InternalizeTerm(zeroPower, &n, &d); err != nil || again != zeroPowerVar {
		t.Fatal("zero power atom mapping", again, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "1" {
		t.Fatal("zero power numerator", got, err)
	}
	if base, degree, err := converter.asPower(zeroPower); err != nil || base != zeroPower || degree != 1 {
		t.Fatal("zero power must have degree one in a product", base, degree, err)
	}
	zeroProduct, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{zero, x, sum})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := converter.InternalizeTerm(zeroProduct, &n, &d); err != nil || got == math.MaxUint32 {
		t.Fatal("n-ary zero product", got, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "0" {
		t.Fatal("n-ary zero product numerator", got, err)
	}
	if got, err := qm.Z.String(&d); err != nil || got != "2" {
		t.Fatal("n-ary zero product denominator", got, err)
	}
	sumLE, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{sum, two})
	if err != nil {
		t.Fatal(err)
	}
	if err := goal.Assert(sumLE); err != nil {
		t.Fatal(err)
	}
	le, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{x, threshold})
	if err != nil {
		t.Fatal(err)
	}
	if err := goal.Assert(le); err != nil {
		t.Fatal(err)
	}
	notLE, err := m.MkNot(le)
	if err != nil {
		t.Fatal(err)
	}
	disjunction, err := m.MkOr(le, notLE)
	if err != nil {
		t.Fatal(err)
	}
	if err := goal.Assert(disjunction); err != nil {
		t.Fatal(err)
	}
	if err := converter.SearchGoal(goal); err != nil {
		t.Fatal("MPQ goal search", err)
	}
	context, err := engine.Context()
	if err != nil {
		t.Fatal(err)
	}
	display, err := converter.VariableDisplay(sy)
	if err != nil {
		t.Fatal(err)
	}
	if err := context.SetVariableDisplay(display); err != nil {
		t.Fatal(err)
	}
	var named bytes.Buffer
	if err := context.DisplayConstraints(&named, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(named.String(), "k!"+strconv.FormatUint(uint64(sumVar), 10)+" = ") || !strings.Contains(named.String(), "x <= 3/2") {
		t.Fatal("inverse AST and generated variable display", named.String())
	}
	root, err := context.Root()
	if err != nil {
		t.Fatal(err)
	}
	view, err := subpaving.NewRationalInterval(qm)
	if err != nil {
		t.Fatal(err)
	}
	if err := view.SetConstant(root, variable); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := view.Display(&out); err != nil || out.String() != "(-oo, 3/2]" {
		t.Fatal("exact searched bound", out.String(), err)
	}
	if err := view.Close(); err != nil {
		t.Fatal(err)
	}
	view, err = subpaving.NewRationalInterval(qm)
	if err != nil {
		t.Fatal(err)
	}
	if err := view.SetConstant(root, sumVar); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	// The source definition propagator omits the stored constant, so the
	// tighter x <= 3/2 bound derives a propagated sum upper endpoint of 3.
	if err := view.Display(&out); err != nil || out.String() != "(-oo, 3]" {
		t.Fatal("compound goal bound", out.String(), err)
	}
	if err := view.Close(); err != nil {
		t.Fatal(err)
	}
	st := statistics.New()
	if err := engine.CollectStatistics(st); err != nil || st.Size() < 2 || st.Key(0) != "new bounds" || st.UintValue(0) < 2 || st.Key(1) != "nodes" || st.UintValue(1) != 1 {
		t.Fatal("source goal statistics", err)
	}
	refsBefore, err := m.GetRefCount(sum)
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := display(&named, variable); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed variable display borrow", err)
	}
	refsAfter, err := m.GetRefCount(sum)
	if err != nil || refsBefore != refsAfter+1 {
		t.Fatal("cached AST reference cleanup", refsBefore, refsAfter, err)
	}
	if _, err := converter.InternalizeTerm(x, &n, &d); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed converter", err)
	}
}
