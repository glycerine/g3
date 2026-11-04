package subpaving

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestFastShellExactCoefficientsAndDirectedInequalities(t *testing.T) {
	b := arena.NewBudget(1 << 27)
	cfg := numeral.DefaultFastFloatConfig(arena.VectorConfig{PageSize: 64, Descriptors: arena.Config{PageSize: 2, Budget: b}})
	cfg.InitialCapacity = 4
	m, err := numeral.NewFastFloatManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	qm, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: b}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: b}}}})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewFastContext(m, &control.Reslimit{Limit: 1000}, nil, 0, 2, 8192)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewFastShellCoefficients(c, qm)
	if err != nil {
		t.Fatal(err)
	}
	stats := statistics.New()
	if err := w.CollectStatistics(stats); err != nil || stats.Size() != 0 {
		t.Fatal("source shell starts before root creation", err)
	}
	var one, three, big, read numeral.Int
	q, lowerValue, upperValue := numeral.NewRational(), numeral.NewRational(), numeral.NewRational()
	t.Cleanup(func() {
		for _, e := range []error{w.Close(), c.Close(), qm.Del(&upperValue), qm.Del(&lowerValue), qm.Del(&q), qm.Z.Del(&read), qm.Z.Del(&big), qm.Z.Del(&three), qm.Z.Del(&one), qm.CloseChecked(), m.CloseChecked()} {
			if e != nil {
				t.Error(e)
			}
		}
		if b.Used() != 0 {
			t.Error("retained MPFF shell owners", b.Used())
		}
	})
	if err := qm.Z.SetInt32(&one, 1); err != nil {
		t.Fatal(err)
	}
	if err := qm.Z.SetInt32(&three, 3); err != nil {
		t.Fatal(err)
	}
	if err := qm.Z.Power(&numeral.Int{Val: 2}, 64, &big); err != nil {
		t.Fatal(err)
	}
	if err := qm.Z.Add(&big, &one, &big); err != nil {
		t.Fatal(err)
	}
	if _, _, err := w.PrepareSum(&three, []*numeral.Int{&one, &big}); !errors.Is(err, ErrFastShellInexactInteger) || !errors.Is(err, ErrSubpavingFailure) {
		t.Fatal("rounded coefficient refusal", err)
	}
	if err := qm.Z.Sub(&big, &one, &big); err != nil {
		t.Fatal(err)
	}
	constant, coefficients, err := w.PrepareSum(&three, []*numeral.Int{&one, &big})
	if err != nil || len(coefficients) != 2 {
		t.Fatal("exact coefficient prefix", len(coefficients), err)
	}
	for i, tc := range []struct {
		value *numeral.FastFloat
		want  *numeral.Int
	}{{constant, &three}, {&coefficients[0], &one}, {&coefficients[1], &big}} {
		if err := m.ToInteger(tc.value, qm.Z, &read); err != nil {
			t.Fatal(i, err)
		}
		equal, err := qm.Z.Equal(&read, tc.want)
		if err != nil || !equal {
			t.Fatal("coefficient", i, equal, err)
		}
	}
	x, err := w.MkVar(false)
	if err != nil || x != 0 {
		t.Fatal("x", x, err)
	}
	y, err := w.MkVar(true)
	if err != nil || y != 1 {
		t.Fatal("y", y, err)
	}
	if c.isInt[x] || !c.isInt[y] {
		t.Fatal("wrapper must preserve each variable's integer flag")
	}
	sum, err := w.MkSum(&three, []*numeral.Int{&big, &one}, []uint32{y, x})
	if err != nil || sum != 2 || c.isInt[sum] {
		t.Fatal("owned sum", sum, err)
	}
	if p := c.polynomials[sum]; p == nil || len(p.terms) != 2 || p.terms[0].Variable != x || p.terms[1].Variable != y {
		t.Fatal("sorted MPFF terms", p)
	}
	if err := qm.SetFractionSigned(&q, 1, 3); err != nil {
		t.Fatal(err)
	}
	lower, err := w.MkInequality(x, &q, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if !lower.lower || !lower.open {
		t.Fatal("lower open inequality flags")
	}
	plus, err := m.RoundingToPlusInf()
	if err != nil || plus {
		t.Fatal("lower directed rounding", plus, err)
	}
	upper, err := w.MkInequality(x, &q, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if upper.lower || upper.open {
		t.Fatal("upper closed inequality flags")
	}
	plus, err = m.RoundingToPlusInf()
	if err != nil || !plus {
		t.Fatal("upper directed rounding", plus, err)
	}
	if err := m.ToRational(lower.value, qm, &lowerValue); err != nil {
		t.Fatal(err)
	}
	if err := m.ToRational(upper.value, qm, &upperValue); err != nil {
		t.Fatal(err)
	}
	if cmp, err := qm.Compare(&lowerValue, &q); err != nil || cmp > 0 {
		t.Fatal("lower enclosure", cmp, err)
	}
	if cmp, err := qm.Compare(&upperValue, &q); err != nil || cmp < 0 {
		t.Fatal("upper enclosure", cmp, err)
	}
	if _, err := w.AddClause([]*FastInequality{lower, upper}); err != nil {
		t.Fatal("clause", err)
	}
	if _, err := w.MkMonomial([]FastPower{{Variable: y, Degree: 2}}); err != nil {
		t.Fatal("monomial", err)
	}
	var display bytes.Buffer
	if err := w.DisplayConstraints(&display, true); err != nil {
		t.Fatal("MPFF display", err)
	}
	if got := display.String(); !strings.HasPrefix(got, "x2 = 3 + x0 + ") || !strings.Contains(got, "*x1\n") || !strings.Contains(got, " or ") || !strings.Contains(got, "x3 = x1^2\n") {
		t.Fatal("MPFF source display", got)
	}
	if err := w.SetVariableDisplay(func(out io.Writer, x uint32) error { return fastWrite(out, "v"+string(rune('0'+x))) }); err != nil {
		t.Fatal(err)
	}
	display.Reset()
	if err := w.DisplayConstraints(&display, false); err != nil || !strings.HasPrefix(display.String(), "v2 = ") {
		t.Fatal("borrowed variable printer", display.String(), err)
	}
	if err := w.SetVariableDisplay(nil); err != nil {
		t.Fatal(err)
	}
	if err := c.InitializeRoot(); err != nil {
		t.Fatal("root initialization", err)
	}
	if err := w.ResetStatistics(); err != nil {
		t.Fatal("shell statistics reset", err)
	}
	stats.Reset()
	if err := w.CollectStatistics(stats); err != nil || stats.Size() != 1 || stats.Key(0) != "nodes" || stats.UintValue(0) != 1 {
		t.Fatal("shell statistics forwarding", err)
	}
	if err := w.ResetStatistics(); err != nil {
		t.Fatal("shell statistics reset", err)
	}
	c.maxDepth = 0
	if err := w.Search(); err != nil {
		t.Fatal("shell search forwarding", err)
	}
}
