package subpaving

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	umm "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestArbitraryShellCoefficientSourceConversionAndCleanup(t *testing.T) {
	b := arena.NewBudget(1 << 27)
	r, err := umm.NewRuntime(umm.RuntimeConfig{Budget: b, SizeTBits: 64})
	if err != nil {
		t.Fatal(err)
	}
	config := numeral.IntegerConfig{
		Memory: r, Profile: umm.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}},
	}
	core, err := numeral.NewArbitraryFloatManager(config)
	if err != nil {
		t.Fatal(err)
	}
	numbers, err := numeral.NewArbitraryFloatNumeralManager(core, 11, 53)
	if err != nil {
		t.Fatal(err)
	}
	qm, err := core.BorrowRationalManager()
	if err != nil {
		t.Fatal(err)
	}
	context, err := NewArbitraryContext(numbers, &control.Reslimit{Limit: 1000}, 0, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewArbitraryShellCoefficients(context)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewArbitraryShellCoefficients(nil); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("missing source context: %v", err)
	}
	copyOwner := *w
	if _, _, err := copyOwner.PrepareSum(&numeral.Int{Val: 1}, nil); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("copied coefficient owner: %v", err)
	}
	if err := copyOwner.Close(); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("copied owner cleanup: %v", err)
	}
	one, big, three := numeral.NewInt(1), numeral.Int{}, numeral.NewInt(3)
	q := numeral.NewRational()
	t.Cleanup(func() {
		for _, e := range []error{w.Close(), context.Close(), qm.Del(&q), qm.Z.Del(&big), numbers.Close(), core.Close(), r.Close()} {
			if e != nil {
				t.Error(e)
			}
		}
		if b.Used() != 0 {
			t.Error("MPF coefficient owners", b.Used())
		}
	})
	if err := qm.Z.Power(&numeral.Int{Val: 2}, 53, &big); err != nil {
		t.Fatal(err)
	}
	if err := qm.Z.Add(&big, &one, &big); err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.PrepareSum(&three, []*numeral.Int{&one, &big}); !errors.Is(err, ErrArbitraryShellInexactInteger) || !errors.Is(err, ErrSubpavingFailure) {
		t.Fatalf("rounded large coefficient: %v", err)
	}
	if err := qm.Z.Sub(&big, &one, &big); err != nil {
		t.Fatal(err)
	}
	constant, coefficients, err := w.PrepareSum(&three, []*numeral.Int{&one, &big})
	if err != nil {
		t.Fatal(err)
	}
	if len(coefficients) != 2 {
		t.Fatal("coefficient order and count", len(coefficients))
	}
	for i, tc := range []struct {
		value *numeral.ArbitraryFloat
		want  float64
	}{{&coefficients[0], 1}, {&coefficients[1], math.Exp2(53)}, {constant, 3}} {
		got, err := core.ToDouble(tc.value)
		if err != nil || got != tc.want {
			t.Fatalf("coefficient %d: %v, %v; want %v", i, got, err, tc.want)
		}
	}
	_, coefficients, err = w.PrepareSum(&three, []*numeral.Int{&one})
	if err != nil || len(coefficients) != 1 || len(w.coefficients) != 2 {
		t.Fatalf("source reserve keeps older slot: %d, %d, %v", len(coefficients), len(w.coefficients), err)
	}
	// reserve(sz) never shrinks the source vector. Its previous high slot is
	// still an owned MPF numeral and remains available for a later larger sum.
	retained, err := core.ToDouble(&w.coefficients[1])
	if err != nil || retained != math.Exp2(53) {
		t.Fatalf("smaller sum lost the retained coefficient: %v, %v", retained, err)
	}
	_, coefficients, err = w.PrepareSum(&three, []*numeral.Int{&one, &big})
	if err != nil || len(coefficients) != 2 || len(w.coefficients) != 2 {
		t.Fatalf("source reserve reuses older slot: %d, %d, %v", len(coefficients), len(w.coefficients), err)
	}
	if err := qm.SetFractionSigned(&q, 1, 3); err != nil {
		t.Fatal(err)
	}
	x, err := w.MkVar(false)
	if err != nil || x != 0 {
		t.Fatalf("first context variable: %d, %v", x, err)
	}
	y, err := w.MkVar(true)
	if err != nil || y != 1 {
		t.Fatalf("second context variable: %d, %v", y, err)
	}
	z, err := w.MkSum(&three, []*numeral.Int{&one, &big}, []uint32{y, x})
	if err != nil || z != 2 || context.variables[z] {
		t.Fatalf("sorted MPF sum variable: %d, %v", z, err)
	}
	p := context.polynomials[z]
	if p == nil || len(p.terms) != 2 || p.terms[0].variable != x || p.terms[1].variable != y {
		t.Fatalf("sorted polynomial terms: %+v", p)
	}
	for i, tc := range []struct {
		value *numeral.ArbitraryFloat
		want  float64
	}{{&p.terms[0].coefficient, math.Exp2(53)}, {&p.terms[1].coefficient, 1}, {&p.constant, 3}} {
		got, e := core.ToDouble(tc.value)
		if e != nil || got != tc.want {
			t.Fatalf("owned sorted sum %d: %v, %v", i, got, e)
		}
	}
	if len(context.watchers[x]) != 1 || context.watchers[x][0] != z || len(context.watchers[y]) != 1 || context.watchers[y][0] != z {
		t.Fatalf("sum watcher registration: %v, %v", context.watchers[x], context.watchers[y])
	}
	duplicate, err := w.MkSum(&three, []*numeral.Int{&one, &big}, []uint32{x, x})
	if err != nil || duplicate != 3 {
		t.Fatalf("duplicate-variable sum: %d, %v", duplicate, err)
	}
	duplicateTerms := context.polynomials[duplicate].terms
	first, err := core.ToDouble(&duplicateTerms[0].coefficient)
	if err != nil || first != math.Exp2(53) || duplicateTerms[1].coefficient != (numeral.ArbitraryFloat{}) {
		t.Fatalf("source duplicate overwrite/swap: %v, %v, %+v", first, err, duplicateTerms[1].coefficient)
	}
	if len(context.watchers[x]) != 3 || context.watchers[x][1] != duplicate || context.watchers[x][2] != duplicate {
		t.Fatalf("duplicate watcher registrations: %v", context.watchers[x])
	}
	if _, err := w.MkSum(&three, []*numeral.Int{&one}, []uint32{x, y}); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("mismatched sum inputs: %v", err)
	}
	preparedConstant, preparedCoefficients, err := w.PrepareSum(&three, []*numeral.Int{&big})
	if err != nil {
		t.Fatal(err)
	}
	beforeStoppedSum := len(context.variables)
	b.FailAfter(0)
	_, stoppedSum := context.DefinePolynomial(preparedConstant, preparedCoefficients, []uint32{x})
	b.FailAfter(-1)
	if stoppedSum == nil || len(context.variables) != beforeStoppedSum {
		t.Fatalf("stopped MPF sum: %v, %d", stoppedSum, len(context.variables))
	}
	inputPowers := []ArbitraryPower{{Variable: y, Degree: 2}, {Variable: x, Degree: 1}, {Variable: y, Degree: 3}}
	monomial, err := w.MkMonomial(inputPowers)
	if err != nil || monomial != 4 || !context.variables[monomial] {
		t.Fatalf("merged MPF monomial: %d, %v", monomial, err)
	}
	merged := context.monomials[monomial].powers
	if len(merged) != 2 || merged[0] != (ArbitraryPower{x, 1}) || merged[1] != (ArbitraryPower{y, 5}) || inputPowers[0].Variable != y {
		t.Fatalf("sorted/merged monomial and borrowed input: %v, %v", merged, inputPowers)
	}
	if len(context.watchers[x]) != 4 || context.watchers[x][3] != monomial || len(context.watchers[y]) != 2 || context.watchers[y][1] != monomial {
		t.Fatalf("merged monomial watchers: %v, %v", context.watchers[x], context.watchers[y])
	}
	secondMonomial, err := w.MkMonomial([]ArbitraryPower{{Variable: x, Degree: 2}})
	if err != nil || secondMonomial != 5 || context.variables[secondMonomial] || len(context.monomials[monomial].powers) != 2 {
		t.Fatalf("scratch reuse and noninteger monomial: %d, %v", secondMonomial, err)
	}
	wrappedMonomial, err := w.MkMonomial([]ArbitraryPower{{Variable: x, Degree: math.MaxUint32}, {Variable: x, Degree: 1}})
	if err != nil || context.monomials[wrappedMonomial].powers[0].Degree != 0 {
		t.Fatalf("source unsigned degree wrap: %d, %v", wrappedMonomial, err)
	}
	if _, err := w.MkMonomial([]ArbitraryPower{{Variable: uint32(len(context.variables)), Degree: 1}}); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("unknown monomial factor: %v", err)
	}
	lower, err := w.PrepareInequality(&q, true)
	if err != nil {
		t.Fatal(err)
	}
	down, err := core.ToDouble(lower)
	if err != nil || down != 1.0/3.0 {
		t.Fatalf("lower direction: %v, %v", down, err)
	}
	upper, err := w.PrepareInequality(&q, false)
	if err != nil {
		t.Fatal(err)
	}
	up, err := core.ToDouble(upper)
	if err != nil || up != math.Nextafter(1.0/3.0, math.Inf(1)) {
		t.Fatalf("upper direction: %v, %v", up, err)
	}
	a, err := w.MkInequality(x, &q, true, false)
	if err != nil || a == nil || a.variable != x || !a.lower || a.open {
		t.Fatalf("context inequality registration: %v, %v", a, err)
	}
	stored, err := core.ToDouble(&a.value)
	if err != nil || stored != down {
		t.Fatalf("context owns directed scalar: %v, %v", stored, err)
	}
	if _, err := w.MkInequality(uint32(len(context.variables)), &q, false, true); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("unknown context variable: %v", err)
	}
	bAtom, err := w.MkInequality(y, &q, false, true)
	if err != nil {
		t.Fatal(err)
	}
	cAtom, err := w.MkInequality(x, &q, false, false)
	if err != nil {
		t.Fatal(err)
	}
	clause, err := w.AddClause([]*ArbitraryInequality{bAtom, a, cAtom})
	if err != nil || clause == nil || len(clause.atoms) != 3 || clause.atoms[0] != a || clause.atoms[1] != cAtom || clause.atoms[2] != bAtom {
		t.Fatalf("stable MPF clause sort: %+v, %v", clause, err)
	}
	if a.refs != 1 || bAtom.refs != 1 || cAtom.refs != 1 || len(context.clauseWatchers[x]) != 1 || context.clauseWatchers[x][0] != clause || len(context.clauseWatchers[y]) != 1 || context.clauseWatchers[y][0] != clause {
		t.Fatalf("MPF clause references and unique watches: %d, %d, %d, %v, %v", a.refs, bAtom.refs, cAtom.refs, context.clauseWatchers[x], context.clauseWatchers[y])
	}
	if singleton, err := w.AddClause([]*ArbitraryInequality{a}); err != nil || singleton != nil || len(context.unitClauses) != 1 || !context.unitClauses[0].axiom || a.refs != 2 {
		t.Fatalf("singleton MPF axiom unit: %v, %v", singleton, err)
	}
	if _, err := w.AddClause(nil); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("empty MPF clause: %v", err)
	}
	if _, err := w.AddClause([]*ArbitraryInequality{a, nil}); !errors.Is(err, arena.ErrInvalidSlot) || a.refs != 2 {
		t.Fatalf("invalid MPF atom before any ref publication: %v, %d", err, a.refs)
	}
	beforeStopped := len(context.inequalities)
	b.FailAfter(0)
	_, stopped := context.NewInequality(x, &a.value, true, false)
	b.FailAfter(-1)
	if stopped == nil || len(context.inequalities) != beforeStopped+1 {
		t.Fatalf("stopped copied numeral ownership: %v, %d", stopped, len(context.inequalities))
	}
	qOne := numeral.NewRational()
	if err := qm.SetInt32(&qOne, 1); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := qm.Del(&qOne); err != nil {
			t.Error(err)
		}
	}()
	strictLower, err := w.MkInequality(y, &qOne, true, true)
	if err != nil {
		t.Fatal(err)
	}
	closedUpper, err := w.MkInequality(y, &qOne, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := context.AddUnitClause(strictLower, true); err != nil {
		t.Fatal(err)
	}
	if err := context.AddUnitClause(closedUpper, true); err != nil {
		t.Fatal(err)
	}
	context.limit.Limit = context.limit.Steps + 1
	root, err := context.AssertRootUnits()
	if !errors.Is(err, ErrArbitraryContextCanceled) || root == nil || context.unitHead != 1 {
		t.Fatalf("MPF root canceled after first unit: %v, %v, %d", root, err, context.unitHead)
	}
	xLower, xUpper, err := root.Bounds(x)
	if err != nil || xLower == nil || xUpper != nil || !xLower.axiom || xLower.open || root.conflictVar != math.MaxUint32 {
		t.Fatalf("MPF first axiom bound: %+v, %+v, %v", xLower, xUpper, err)
	}
	context.limit.Limit = 0
	resumed, err := context.AssertRootUnits()
	if err != nil || resumed != root || context.unitHead != 3 || root.conflictVar != y {
		t.Fatalf("MPF root conflict after resumed units: %v, %v, %d, %d", resumed, err, context.unitHead, root.conflictVar)
	}
	yLower, yUpper, err := root.Bounds(y)
	if err != nil || yLower == nil || yUpper == nil || yLower.open || yUpper.open || yLower.timestamp != 1 || yUpper.timestamp != 2 {
		t.Fatalf("MPF directed integer bounds: %+v, %+v, %v", yLower, yUpper, err)
	}
	gotLower, err := core.ToDouble(&yLower.value)
	if err != nil || gotLower != 2 {
		t.Fatalf("strict integer lower bound: %v, %v", gotLower, err)
	}
	gotUpper, err := core.ToDouble(&yUpper.value)
	if err != nil || gotUpper != 1 {
		t.Fatalf("closed integer upper bound: %v, %v", gotUpper, err)
	}
	if _, err := w.MkVar(false); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("variable registration after root snapshot: %v", err)
	}
	// The retained scoped_mpf_vector grows its logical size and keeps the
	// untouched high numeral when a later reserve requests fewer rows.
	wide := make([]*numeral.Int, 257)
	for i := range wide {
		wide[i] = &one
	}
	wide[256] = &big
	if _, rows, err := w.PrepareSum(&three, wide); err != nil || len(rows) != 257 {
		t.Fatalf("wide source coefficient reserve: %d, %v", len(rows), err)
	}
	if _, rows, err := w.PrepareSum(&three, []*numeral.Int{&one}); err != nil || len(rows) != 1 || len(w.coefficients) != 257 {
		t.Fatalf("smaller reserve keeps owning high row: %d, %d, %v", len(rows), len(w.coefficients), err)
	}
	retained, err = core.ToDouble(&w.coefficients[256])
	if err != nil || retained != math.Exp2(53) {
		t.Fatalf("high MPF row after native growth and shrink: %v, %v", retained, err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := context.open(); err != nil {
		t.Fatal("borrowed shell closed its context", err)
	}
	if _, _, err := w.PrepareSum(&three, nil); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed coefficient owner: %v", err)
	}
	if err := context.Close(); err != nil {
		t.Fatal(err)
	}
	if a.refs != 0 || bAtom.refs != 0 || cAtom.refs != 0 {
		t.Fatalf("MPF clause/unit references after context cleanup: %d, %d, %d", a.refs, bAtom.refs, cAtom.refs)
	}
	if _, err := context.NewVariable(false); !errors.Is(err, arena.ErrClosed) {
		t.Fatalf("closed MPF context: %v", err)
	}
}

func TestArbitraryShellCatchesNonregularNumeralAtInequalityBoundary(t *testing.T) {
	f := newArbitraryClauseFixture(t)
	w, err := NewArbitraryShellCoefficients(f.context)
	if err != nil {
		t.Fatal(err)
	}
	core, err := f.numbers.BorrowCore()
	if err != nil {
		t.Fatal(err)
	}
	qm, err := core.BorrowRationalManager()
	if err != nil {
		t.Fatal(err)
	}
	var power numeral.Int
	q := numeral.NewRational()
	t.Cleanup(func() {
		for _, e := range []error{w.Close(), qm.Del(&q), qm.Z.Del(&power)} {
			if e != nil {
				t.Error(e)
			}
		}
	})
	if err := qm.Z.Power(&numeral.Int{Val: 2}, 2000, &power); err != nil {
		t.Fatal(err)
	}
	if err := qm.SetInteger(&q, &power); err != nil {
		t.Fatal(err)
	}
	if _, err := w.PrepareInequality(&q, false); !errors.Is(err, ErrSubpavingFailure) || errors.Is(err, numeral.ErrArbitraryFloatNonRegular) {
		t.Fatal("source MPF wrapper must replace its caught numeral exception", err)
	}
	if _, _, err := w.PrepareSum(&power, nil); !errors.Is(err, ErrSubpavingFailure) || errors.Is(err, numeral.ErrArbitraryFloatNonRegular) {
		t.Fatal("source MPF sum conversion must replace its caught numeral exception", err)
	}
}
