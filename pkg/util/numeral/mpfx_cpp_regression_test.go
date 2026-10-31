package numeral

// Source: src/test/mpfx.cpp: tst_prev_power_2 and tst1.
import (
	"math/big"
	"testing"
)

func TestFixedPointCPPRegression(t *testing.T) {
	m, err := NewFixedPointManager(2, 2)
	if err != nil {
		t.Fatal(err)
	}
	a, err := m.New()
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.New()
	if err != nil {
		t.Fatal(err)
	}
	c, err := m.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, v := range []*FixedPoint{a, b, c} {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	}()
	rows := []struct {
		num  int64
		den  uint64
		want uint32
	}{
		{-10, 1, 0}, {0, 1, 0}, {1, 1, 0}, {2, 1, 1}, {3, 1, 1}, {4, 1, 2}, {5, 1, 2}, {8, 1, 3}, {9, 1, 3}, {9, 2, 2}, {9, 4, 1}, {9, 5, 0},
		{(1 << 60) + 1, 1, 60}, {1 << 60, 1, 60}, {(1 << 60) - 1, 1, 59}, {1 << 60, 3, 58},
	}
	for _, row := range rows {
		if err := m.SetRatio(a, row.num, row.den); err != nil {
			t.Fatal(err)
		}
		got, err := m.PrevPowerOfTwo(a)
		if err != nil || got != row.want {
			t.Fatalf("prev power %d/%d = %d, %v; want %d", row.num, row.den, got, err, row.want)
		}
	}
	for _, op := range []func() error{
		func() error { return m.SetInt64(a, 1) }, func() error { return m.SetInt64(b, 2) }, func() error { return m.Add(a, b, c) },
	} {
		if err := op(); err != nil {
			t.Fatal(err)
		}
	}
	sum, err := m.Rational(c)
	if err != nil || sum.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatalf("1+2 = %v, %v", sum, err)
	}
	for _, op := range []func() error{
		func() error { return m.SetInt64(a, 5) }, func() error { return m.SetInt64(c, 3) },
		func() error { return m.Mul(a, a, a) }, func() error { return m.Mul(a, b, a) }, func() error { return m.Div(a, c, b) },
	} {
		if err := op(); err != nil {
			t.Fatal(err)
		}
	}
	down, err := m.Rational(b)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.SetRounding(true); err != nil {
		t.Fatal(err)
	}
	if err = m.Div(a, c, b); err != nil {
		t.Fatal(err)
	}
	up, err := m.Rational(b)
	if err != nil {
		t.Fatal(err)
	}
	exact := big.NewRat(50, 3)
	if down.Cmp(exact) > 0 || up.Cmp(exact) < 0 {
		t.Fatalf("50/3 outside [%s,%s]", down, up)
	}
	if err = m.SetRatio(a, -1, 4); err != nil {
		t.Fatal(err)
	}
	quarter, err := m.Rational(a)
	if err != nil || quarter.Cmp(big.NewRat(-1, 4)) != 0 {
		t.Fatalf("-1/4 = %v, %v", quarter, err)
	}
}
