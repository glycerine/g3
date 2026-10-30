package arith

import (
	"bytes"
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func equationManager(t *testing.T) (*numeral.RationalManager, *LinearEquationManager) {
	t.Helper()
	b := arena.NewBudget(1 << 30)
	m, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
		Cells: arena.Config{Budget: b},
		Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	l, err := NewLinearEquationManager(m)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := l.Close(); err != nil {
			t.Fatal(err)
		}
		if err := m.CloseChecked(); err != nil {
			t.Fatal(err)
		}
		if used := b.Used(); used != 0 {
			t.Fatalf("numeral owner retained %d bytes", used)
		}
	})
	return m, l
}

func TestLinearEquationCloseResumesAfterStoppedCoefficientRelease(t *testing.T) {
	m, l := equationManager(t)
	z := m.Integers()
	as := []numeral.Int{numeral.NewInt(0), numeral.NewInt(0)}
	for i, value := range []string{
		"12345678901234567890123456789012345678901",
		"12345678901234567890123456789012345678902",
	} {
		equationMust(t, z.SetString(&as[i], value))
	}
	defer func() {
		for i := range as {
			equationMust(t, z.Del(&as[i]))
		}
	}()
	eq, err := l.MkInt(as, []uint32{1, 2}, true)
	equationMust(t, err)
	if eq.as[0].Cell == 0 || eq.as[1].Cell == 0 {
		t.Fatal("fixture did not retain two physical coefficient cells")
	}
	second := eq.as[1].Cell
	eq.as[1].Cell = ^numeral.NumericCellRef(0)
	if err = eq.Close(); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatalf("stopped second coefficient release: %v", err)
	}
	if eq.closeIndex != 1 || eq.as[0].Cell != 0 || eq.closed {
		t.Fatal("completed first coefficient was not retained as cleanup prefix")
	}
	eq.as[1].Cell = second
	eq.as[0].Cell = ^numeral.NumericCellRef(0)
	equationMust(t, eq.Close())
	if !eq.closed {
		t.Fatal("equation did not close after continuing at second coefficient")
	}
}

func equationMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLinearEquationIntegerCoalesceSortNormalizeAndReuse(t *testing.T) {
	m, l := equationManager(t)
	z := m.Integers()
	as := []numeral.Int{numeral.NewInt(8), numeral.NewInt(4), numeral.NewInt(-8), numeral.NewInt(-12), numeral.NewInt(20)}
	xs := []uint32{5, 1, 5, 3, 3}
	eq, err := l.MkInt(as, xs, false)
	equationMust(t, err)
	if eq == nil || eq.Size() != 2 || eq.Pos(1) != 0 || eq.Pos(3) != 1 || eq.Pos(5) != noEquationPosition {
		t.Fatalf("coalesced equation variables: eq=%v xs=%v", eq, xs)
	}
	if x, err := eq.Var(0); err != nil || x != 1 {
		t.Fatalf("first sorted variable = %d, %v", x, err)
	}
	var out bytes.Buffer
	equationMust(t, l.Display(&out, eq))
	if got := out.String(); got != "1*x1 + 2*x3 = 0" {
		t.Fatalf("normalized equation = %q", got)
	}
	if got, err := eq.Approx(1); err != nil || got != 2 {
		t.Fatalf("approx coefficient = %g, %v", got, err)
	}
	copy := numeral.NewInt(0)
	equationMust(t, eq.CopyCoefficient(1, &copy))
	equationMust(t, z.SetInt64(&copy, 91))
	out.Reset()
	equationMust(t, l.Display(&out, eq))
	if got := out.String(); got != "1*x1 + 2*x3 = 0" {
		t.Fatalf("copy changed equation: %q", got)
	}
	equationMust(t, z.Del(&copy))
	equationMust(t, eq.Close())
	for i := range as {
		equationMust(t, z.Del(&as[i]))
	}

	// The same manager reuses its indexed numeral buffer after the first row.
	as2 := []numeral.Int{numeral.NewInt(-6), numeral.NewInt(9)}
	xs2 := []uint32{3, 0}
	eq2, err := l.MkInt(as2, xs2, true)
	equationMust(t, err)
	out.Reset()
	equationMust(t, l.Display(&out, eq2))
	if got := out.String(); got != "3*x0 + -2*x3 = 0" {
		t.Fatalf("reused equation = %q", got)
	}
	equationMust(t, eq2.Close())
	for i := range as2 {
		equationMust(t, z.Del(&as2[i]))
	}
}

func TestLinearEquationIntegerCancellationAndLargeOwnership(t *testing.T) {
	m, l := equationManager(t)
	z := m.Integers()
	as := []numeral.Int{numeral.NewInt(7), numeral.NewInt(-7), numeral.NewInt(9)}
	xs := []uint32{4, 4, 2}
	eq, err := l.MkInt(as, xs, false)
	equationMust(t, err)
	if eq != nil {
		t.Fatal("one surviving variable should refuse an equation")
	}
	for i := range as {
		equationMust(t, z.Del(&as[i]))
	}

	large := []numeral.Int{{}, {}}
	equationMust(t, z.SetString(&large[0], "123456789012345678900"))
	equationMust(t, z.SetString(&large[1], "-246913578024691357800"))
	ids := []uint32{7, 2}
	eq, err = l.MkInt(large, ids, true)
	equationMust(t, err)
	var out bytes.Buffer
	equationMust(t, l.Display(&out, eq))
	if got := out.String(); got != "-2*x2 + 1*x7 = 0" {
		t.Fatalf("large normalized equation = %q", got)
	}
	equationMust(t, eq.Close())
	for i := range large {
		equationMust(t, z.Del(&large[i]))
	}
}

func TestLinearEquationRationalDenominatorClearingAndCombination(t *testing.T) {
	m, l := equationManager(t)
	z := m.Integers()
	ras := []numeral.Rational{numeral.NewRational(), numeral.NewRational(), numeral.NewRational()}
	for i, v := range []string{"1/2", "-2/3", "1/6"} {
		equationMust(t, m.SetString(&ras[i], v))
	}
	ids := []uint32{5, 1, 3}
	eq, err := l.MkRational(ras, ids, true)
	equationMust(t, err)
	var out bytes.Buffer
	equationMust(t, l.Display(&out, eq))
	if got := out.String(); got != "-4*x1 + 1*x3 + 3*x5 = 0" {
		t.Fatalf("cleared rational equation = %q", got)
	}
	for i := range ras {
		integer, err := m.IsInt(&ras[i])
		equationMust(t, err)
		if !integer {
			t.Fatalf("caller rational %d was not scaled to an integer", i)
		}
	}
	for i := range ras {
		equationMust(t, m.Del(&ras[i]))
	}

	as2 := []numeral.Int{numeral.NewInt(4), numeral.NewInt(5)}
	ids2 := []uint32{1, 7}
	eq2, err := l.MkInt(as2, ids2, true)
	equationMust(t, err)
	b1, b2 := numeral.NewInt(1), numeral.NewInt(1)
	combined, err := l.Combine(&b1, eq, &b2, eq2)
	equationMust(t, err)
	out.Reset()
	equationMust(t, l.Display(&out, combined))
	if got := out.String(); got != "1*x3 + 3*x5 + 5*x7 = 0" {
		t.Fatalf("combined equation = %q", got)
	}
	minusOne := numeral.NewInt(-1)
	zero, err := l.Combine(&b1, eq, &minusOne, eq)
	equationMust(t, err)
	if zero != nil {
		t.Fatal("exactly cancelled equation should be nil")
	}
	for _, item := range []*LinearEquation{combined, eq2, eq} {
		equationMust(t, item.Close())
	}
	for i := range as2 {
		equationMust(t, z.Del(&as2[i]))
	}
}
