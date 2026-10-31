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
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestFastExprConverterTermsAndClause(t *testing.T) {
	b := arena.NewBudget(1 << 27)
	cfg := numeral.DefaultFastFloatConfig(arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: b}})
	cfg.InitialCapacity = 4
	numbers, err := numeral.NewFastFloatManager(cfg)
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
	selected, err := sy.Intern("mpff")
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
	engine, err := NewFastEngine(&control.Reslimit{Limit: 1000}, numbers, qm, pc, &p)
	if err != nil {
		t.Fatal(err)
	}
	converter, err := NewFastExprConverter(m, engine, qm)
	if err != nil {
		t.Fatal(err)
	}
	goal, err := goaltactic.NewGoal(m, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	var n, d numeral.Int
	value, err := q.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, e := range []error{goal.Close(), converter.Close(), engine.Close(), qm.Z.Del(&n), qm.Z.Del(&d), qm.CloseChecked(), pc.Del(&p), pc.Close(), q.Del(&value), m.Close(), q.Close(), sy.Close(), numbers.CloseChecked()} {
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
	if again, err := converter.InternalizeTerm(sum, &n, &d); err != nil || again != variable {
		t.Fatal("cached sum", again, err)
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
		t.Fatal("zero power monomial definition", zeroPowerVar, err)
	}
	if yes, err := converter.IsVar(zeroPower); err != nil || yes {
		t.Fatal("zero power should map through a monomial definition", yes, err)
	}
	zeroPowerProduct, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{zeroPower, y})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := converter.InternalizeTerm(zeroPowerProduct, &n, &d); err != nil || got == math.MaxUint32 || got == zeroPowerVar {
		t.Fatal("zero power degree in a product", got, err)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "1" {
		t.Fatal("zero power product numerator", got, err)
	}
	zeroProduct, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{zero, x})
	if err != nil {
		t.Fatal(err)
	}
	beforeCoefficient := m.ResourceLimit().Count()
	if got, err := converter.InternalizeTerm(zeroProduct, &n, &d); err != nil || got != first {
		t.Fatal("binary zero scalar product", got, err)
	}
	if charged := m.ResourceLimit().Count() - beforeCoefficient; charged != 2 {
		t.Fatal("coefficient must not consume a process checkpoint", charged)
	}
	if got, err := qm.Z.String(&n); err != nil || got != "0" {
		t.Fatal("binary zero scalar product numerator", got, err)
	}
	if got, err := qm.Z.String(&d); err != nil || got != "1" {
		t.Fatal("binary zero scalar product denominator", got, err)
	}
	naryZeroProduct, err := m.MkFamilyApp(family, 9, nil, []ast.ASTHandle{zero, x, y})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := converter.InternalizeTerm(naryZeroProduct, &n, &d); !errors.Is(err, ErrExprConverterNonSimplified) {
		t.Fatal("source rejects a numeric scalar in an n-ary product", err)
	}
	ineq, err := m.MkFamilyApp(family, 2, nil, []ast.ASTHandle{sum, two})
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.InternalizeClause(ineq); err != nil {
		t.Fatal("clause", err)
	}
	negated, err := m.MkNot(ineq)
	if err != nil {
		t.Fatal(err)
	}
	disjunction, err := m.MkOr(ineq, negated)
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.InternalizeClause(disjunction); err != nil {
		t.Fatal("ordered disjunction", err)
	}
	beforeUnwind, err := numbers.Stats()
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := m.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	broken, err := m.MkOr(ineq, invalid)
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.InternalizeClause(broken); !errors.Is(err, ErrFastUnsupportedAtom) {
		t.Fatal("second literal should unwind the first temporary inequality", err)
	}
	afterUnwind, err := numbers.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if afterUnwind.Live != beforeUnwind.Live {
		t.Fatal("temporary ref_buffer retained a fast numeral", beforeUnwind, afterUnwind)
	}
	if err := goal.Assert(ineq); err != nil {
		t.Fatal(err)
	}
	if err := converter.SearchGoal(goal); err != nil {
		t.Fatal("goal search", err)
	}
	shell, err := engine.Shell()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := shell.DisplayConstraints(&out, true); err != nil {
		t.Fatal("MPFF constraints", err)
	}
	if got, want := out.String(), "x2 = 1 + x0 + x1\nx3 = x0^2\nx4 = x0\nx5 = x0*x1\nx2 <= 2\nx2 <= 2\nx2 <= 2 or 2 < x2\n"; got != want {
		t.Fatal("source-order MPFF constraints", out.String())
	}
	out.Reset()
	if err := shell.DisplayBounds(&out); err != nil || !strings.Contains(out.String(), "x2 <= 2") {
		t.Fatal("searched MPFF bound", out.String(), err)
	}
	if _, err := converter.InternalizeInequality(ineq); err != nil {
		t.Fatal("inequality", err)
	}
	refsBefore, err := m.GetRefCount(sum)
	if err != nil {
		t.Fatal(err)
	}
	if err := converter.Close(); err != nil {
		t.Fatal(err)
	}
	if refs, err := m.GetRefCount(sum); err != nil || refsBefore != refs+1 {
		t.Fatal("released cached AST key", refs, err)
	}
	if _, err := converter.InternalizeTerm(x, &n, &d); !errors.Is(err, arena.ErrClosed) {
		t.Fatal("closed converter", err)
	}
}
