package tactic

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/subpaving"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestArbitraryExprConverterTermsAndClause(t *testing.T) {
	b := arena.NewBudget(1 << 27)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: b, SizeTBits: 64})
	if err != nil {
		t.Fatal(err)
	}
	core, err := numeral.NewArbitraryFloatManager(numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	if err != nil {
		t.Fatal(err)
	}
	numbers, err := numeral.NewArbitraryFloatNumeralManager(core, 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	sy, err := symbol.NewManager(symbol.Config{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
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
	sort, err := m.MkFamilySort(family, 1, nil)
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
	selected, err := sy.Intern("mpf")
	if err != nil {
		t.Fatal(err)
	}
	key, depth := "numeral", "max_depth"
	if err := pc.SetSymbolName(&p, &key, selected); err != nil {
		t.Fatal(err)
	}
	if err := pc.SetUIntName(&p, &depth, 0); err != nil {
		t.Fatal(err)
	}
	qm, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewArbitraryEngine(&control.Reslimit{Limit: 1000}, numbers, pc, &p)
	if err != nil {
		t.Fatal(err)
	}
	converter, err := NewArbitraryExprConverter(m, engine, qm)
	if err != nil {
		t.Fatal(err)
	}
	var n, d numeral.Int
	value, err := q.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, e := range []error{converter.Close(), engine.Close(), qm.Z.Del(&n), qm.Z.Del(&d), qm.CloseChecked(), pc.Del(&p), pc.Close(), q.Del(&value), m.Close(), q.Close(), sy.Close(), numbers.Close(), core.Close(), runtime.Close()} {
			if e != nil {
				t.Error(e)
			}
		}
		if b.Used() != 0 {
			t.Error("retained owner bytes", b.Used())
		}
	})
	name, err := sy.Intern("x")
	if err != nil {
		t.Fatal(err)
	}
	x, err := m.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	yName, err := sy.Intern("y")
	if err != nil {
		t.Fatal(err)
	}
	y, err := m.MkConst(yName, sort)
	if err != nil {
		t.Fatal(err)
	}
	first, err := converter.InternalizeTerm(x, &n, &d)
	if err != nil || first == math.MaxUint32 {
		t.Fatal("variable", first, err)
	}
	if err := q.SetString(&value, "1"); err != nil {
		t.Fatal(err)
	}
	one, err := m.ArithmeticNumeral(&value, true)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{x, y, one})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.IncRef(sum); err != nil {
		t.Fatal(err)
	}
	if err := m.IncRef(sum); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.DecRef(sum); err != nil {
			t.Error(err)
		}
		if err := m.DecRef(sum); err != nil {
			t.Error(err)
		}
	}()
	variable, err := converter.InternalizeTerm(sum, &n, &d)
	if err != nil || variable == first || variable == math.MaxUint32 {
		t.Fatal("sum", variable, err)
	}
	if got, err := qm.Z.String(&d); err != nil || got != "1" {
		t.Fatal("sum denominator", got, err)
	}
	wantNumerator, err := qm.Z.String(&n)
	if err != nil {
		t.Fatal(err)
	}
	if err := qm.Z.SetInt32(&n, 17); err != nil {
		t.Fatal(err)
	}
	if err := qm.Z.SetInt32(&d, 19); err != nil {
		t.Fatal(err)
	}
	if again, err := converter.InternalizeTerm(sum, &n, &d); err != nil || again != variable {
		t.Fatal("cached sum", again, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != wantNumerator {
		t.Fatal("cached numerator was not independently copied", got, wantNumerator, err)
	}
	if got, err := qm.Z.String(&d); err != nil || got != "1" {
		t.Fatal("cached denominator was not independently copied", got, err)
	}
	if refs, err := m.GetRefCount(sum); err != nil || refs != 3 {
		t.Fatal("cached AST ownership", refs, err)
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
		t.Fatal("power", v, err)
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
		t.Fatal("zero power must remain an atomic variable", zeroPowerVar, err)
	}
	if yes, err := converter.IsVar(zeroPower); err != nil || !yes {
		t.Fatal("zero power atom mapping", yes, err)
	}
	zeroPowerProduct, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{zeroPower, y})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := converter.InternalizeTerm(zeroPowerProduct, &n, &d); err != nil || got == math.MaxUint32 || got == zeroPowerVar {
		t.Fatal("zero power must have degree one in a product", got, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "1" {
		t.Fatal("zero power product numerator", got, err)
	}
	zeroProduct, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{zero, x, y})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := converter.InternalizeTerm(zeroProduct, &n, &d); !errors.Is(err, ErrExprConverterNonSimplified) {
		t.Fatal("source rejects n-ary product with numeric first factor", err)
	}
	zeroScalar, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{zero, x})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := converter.InternalizeTerm(zeroScalar, &n, &d); err != nil || got != first {
		t.Fatal("binary zero scalar product", got, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "0" {
		t.Fatal("binary zero scalar product numerator", got, err)
	}
	if got, err := qm.Z.String(&d); err != nil || got != "1" {
		t.Fatal("binary zero scalar product denominator", got, err)
	}
	ineq, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{sum, two})
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.InternalizeClause(ineq); err != nil {
		t.Fatal("clause", err)
	}
	shell, err := engine.Shell()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := shell.DisplayConstraints(&out, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "2") {
		t.Fatal("missing bound", out.String())
	}
	// An unbounded even power makes interval::power reset the lower endpoint
	// to formatted zero. The next interval::set copies it through f2n::set,
	// whose is_regular check refuses zero (interval_def.h:1333-1363,668-684).
	if err := shell.Search(); !errors.Is(err, numeral.ErrArbitraryFloatNonRegular) {
		t.Fatal("source MPF even-power refusal", err)
	}
	zeroConstant, err := m.MkFamilyApp(family, 6, nil, []ast.ASTHandle{x, y})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := converter.InternalizeTerm(zeroConstant, &n, &d); !errors.Is(err, subpaving.ErrSubpavingFailure) || errors.Is(err, numeral.ErrArbitraryFloatNonRegular) {
		t.Fatal("MPF wrapper replaces zero-constant f2n refusal", err)
	}
	refsBefore, err := m.GetRefCount(sum)
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.Close(); err != nil {
		t.Fatal(err)
	}
	if refs, err := m.GetRefCount(sum); err != nil || refsBefore != refs+1 {
		t.Fatal("released cached AST key after refused expression", refs, err)
	}
}

func TestArbitraryExprConverterSearchSimpleGoal(t *testing.T) {
	b := arena.NewBudget(1 << 26)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: b, SizeTBits: 64})
	if err != nil {
		t.Fatal(err)
	}
	core, err := numeral.NewArbitraryFloatManager(numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}})
	if err != nil {
		t.Fatal(err)
	}
	numbers, err := numeral.NewArbitraryFloatNumeralManager(core, 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	sy, err := symbol.NewManager(symbol.Config{Budget: b})
	if err != nil {
		t.Fatal(err)
	}
	q, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
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
	sort, err := m.MkFamilySort(family, 1, nil)
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
	selected, err := sy.Intern("mpf")
	if err != nil {
		t.Fatal(err)
	}
	key, depth := "numeral", "max_depth"
	if err := pc.SetSymbolName(&p, &key, selected); err != nil {
		t.Fatal(err)
	}
	if err := pc.SetUIntName(&p, &depth, 0); err != nil {
		t.Fatal(err)
	}
	qm, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewArbitraryEngine(&control.Reslimit{Limit: 1000}, numbers, pc, &p)
	if err != nil {
		t.Fatal(err)
	}
	converter, err := NewArbitraryExprConverter(m, engine, qm)
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
	t.Cleanup(func() {
		for _, e := range []error{goal.Close(), converter.Close(), engine.Close(), qm.CloseChecked(), pc.Del(&p), pc.Close(), q.Del(&value), m.Close(), q.Close(), sy.Close(), numbers.Close(), core.Close(), runtime.Close()} {
			if e != nil {
				t.Error(e)
			}
		}
		if b.Used() != 0 {
			t.Error("retained goal owner bytes", b.Used())
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
	if err := q.SetString(&value, "2"); err != nil {
		t.Fatal(err)
	}
	two, err := m.ArithmeticNumeral(&value, true)
	if err != nil {
		t.Fatal(err)
	}
	le, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{x, two})
	if err != nil {
		t.Fatal(err)
	}
	if err := goal.Assert(le); err != nil {
		t.Fatal(err)
	}
	if err := converter.SearchGoal(goal); err != nil {
		t.Fatal("MPF simple goal search", err)
	}
	shell, err := engine.Shell()
	if err != nil {
		t.Fatal(err)
	}
	var display bytes.Buffer
	if err := shell.DisplayBounds(&display); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(display.String(), "2") {
		t.Fatal("missing searched upper bound", display.String())
	}
}
