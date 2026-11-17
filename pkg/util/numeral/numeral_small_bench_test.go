package numeral

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

// Small-value micro-benchmarks: 7/3 op 5/2 (and 7 op 5 for Int), the
// operands theory_arith sees most. Run with
//   go test -run XXX -bench Small ./util/numeral

func benchRationalManager(b *testing.B, synch bool) *RationalManager {
	b.Helper()
	budget := arena.NewBudget(1 << 24)
	m, e := NewRationalManager(RationalConfig{Integers: IntegerConfig{Cells: arena.Config{PageSize: 2, Budget: budget}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: budget}}}, Synchronized: synch})
	if e != nil {
		b.Fatal(e)
	}
	b.Cleanup(m.Close)
	return m
}

func benchSmallPair(b *testing.B, m *RationalManager) (x, y, out Rational) {
	x, y, out = NewRational(), NewRational(), NewRational()
	if e := m.SetFraction(&x, 7, 3); e != nil {
		b.Fatal(e)
	}
	if e := m.SetFraction(&y, 5, 2); e != nil {
		b.Fatal(e)
	}
	return
}

func benchRationalOps(b *testing.B, synch bool) {
	m := benchRationalManager(b, synch)
	x, y, out := benchSmallPair(b, m)
	run := func(name string, f func() error) {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if e := f(); e != nil {
					b.Fatal(e)
				}
			}
		})
	}
	run("add", func() error { return m.Add(&x, &y, &out) })
	run("sub", func() error { return m.Sub(&x, &y, &out) })
	run("mul", func() error { return m.Mul(&x, &y, &out) })
	run("div", func() error { return m.Div(&x, &y, &out) })
	run("cmp", func() error { _, e := m.Compare(&x, &y); return e })
	run("eq", func() error { _, e := m.Equal(&x, &y); return e })
	run("copy", func() error { return m.Copy(&out, &x) })
	run("reset", func() error { return m.Reset(&out) })
	run("addmul", func() error { return m.AddMul(&x, &y, &x, &out) })
}

func BenchmarkSmallRationalUnsync(b *testing.B) { benchRationalOps(b, false) }
func BenchmarkSmallRationalSync(b *testing.B)   { benchRationalOps(b, true) }

func BenchmarkSmallRationalService(b *testing.B) {
	budget := arena.NewBudget(1 << 24)
	s, e := NewRationalService(serviceConfig(budget))
	if e != nil {
		b.Fatal(e)
	}
	b.Cleanup(func() { s.Close() })
	x, _ := s.NewValue()
	y, _ := s.NewValue()
	out, _ := s.NewValue()
	if e := s.SetFraction(&x, 7, 3); e != nil {
		b.Fatal(e)
	}
	if e := s.SetFraction(&y, 5, 2); e != nil {
		b.Fatal(e)
	}
	run := func(name string, f func() error) {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if e := f(); e != nil {
					b.Fatal(e)
				}
			}
		})
	}
	run("add", func() error {
		if e := s.Copy(&out, &x); e != nil {
			return e
		}
		return s.AddAssign(&out, &y)
	})
	run("addvalue", func() error {
		v, e := s.Add(&x, &y)
		if e != nil {
			return e
		}
		return s.Del(&v)
	})
	run("mulassign", func() error {
		if e := s.Copy(&out, &x); e != nil {
			return e
		}
		return s.MulAssign(&out, &y)
	})
	run("less", func() error { _, e := s.Less(&x, &y); return e })
	run("copy", func() error { return s.Copy(&out, &x) })
}

func BenchmarkSmallInt(b *testing.B) {
	m := benchRationalManager(b, false).Z
	x, y, out := NewInt(7), NewInt(5), NewInt(0)
	run := func(name string, f func() error) {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if e := f(); e != nil {
					b.Fatal(e)
				}
			}
		})
	}
	run("add", func() error { return m.Add(&x, &y, &out) })
	run("sub", func() error { return m.Sub(&x, &y, &out) })
	run("mul", func() error { return m.Mul(&x, &y, &out) })
	run("div", func() error { return m.Div(&x, &y, &out) })
	run("gcd", func() error { return m.GCD(&x, &y, &out) })
	run("cmp", func() error { _, e := m.Compare(&x, &y); return e })
	run("copy", func() error { return m.Copy(&out, &x) })
	run("reset", func() error { return m.Reset(&out) })
}

func BenchmarkSmallBigRat(b *testing.B) {
	x, y, out := big.NewRat(7, 3), big.NewRat(5, 2), new(big.Rat)
	b.Run("add", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			out.Add(x, y)
		}
	})
	b.Run("mul", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			out.Mul(x, y)
		}
	})
	b.Run("cmp", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = x.Cmp(y)
		}
	})
	b.Run("copy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			out.Set(x)
		}
	})
}
