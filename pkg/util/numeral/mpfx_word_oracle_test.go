package numeral

import (
	"errors"
	"math/big"
	"math/rand"
	"testing"
)

// mpfxOracleRound is an independent exact math/big model of mpfx's
// directed rounding: the exact value on the 2^-(32*frac) grid, rounded up
// toward plus infinity when up is set and down otherwise, and an overflow
// when the rounded magnitude needs more than the integer words.
func mpfxOracleRound(x *big.Rat, intWords, fracWords uint32, up bool) (*big.Rat, bool) {
	scale := new(big.Int).Lsh(big.NewInt(1), uint(32*fracWords))
	scaled := new(big.Rat).Mul(x, new(big.Rat).SetInt(scale))
	q, r := new(big.Int).QuoRem(scaled.Num(), scaled.Denom(), new(big.Int))
	if r.Sign() != 0 {
		if up && scaled.Sign() > 0 {
			q.Add(q, big.NewInt(1))
		}
		if !up && scaled.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		}
	}
	limit := new(big.Int).Lsh(big.NewInt(1), uint(32*(intWords+fracWords)))
	if new(big.Int).Abs(q).Cmp(limit) >= 0 {
		return nil, false
	}
	return new(big.Rat).SetFrac(q, scale), true
}

func mpfxRandomValue(r *rand.Rand, m *FixedPointManager, v *FixedPoint) {
	m.reset(v)
	if r.Intn(8) == 0 {
		return
	}
	m.allocateIfNeeded(v)
	// Mostly small values, sometimes near the integer limit or epsilon.
	used := 1 + r.Intn(int(m.totalWords))
	for i := 0; i < used; i++ {
		v.words[i] = r.Uint32()
		if r.Intn(4) == 0 {
			v.words[i] = 0xffffffff
		}
	}
	if r.Intn(3) == 0 {
		hi := m.totalWords - 1
		v.words[hi] >>= uint(r.Intn(32))
	} else if used == int(m.totalWords) {
		v.words[m.totalWords-1] >>= 16
	}
	v.sign = r.Intn(2) == 0
	if fixedWordsZero(v.words) {
		m.reset(v)
	}
}

// TestFixedPointWordsMatchExactOracle checks add, sub, mul, div, floor,
// ceil, inc and dec on the word arrays against exact math/big rounding for
// several word layouts and both rounding directions.
func TestFixedPointWordsMatchExactOracle(t *testing.T) {
	r := rand.New(rand.NewSource(91))
	n := 4000
	if testing.Short() {
		n = 40000
	}
	for _, layout := range [][2]uint32{{1, 1}, {2, 1}, {1, 2}, {3, 2}, {2, 3}} {
		m, err := NewFixedPointManager(layout[0], layout[1])
		if err != nil {
			t.Fatal(err)
		}
		a, _ := m.New()
		b, _ := m.New()
		c, _ := m.New()
		for i := 0; i < n; i++ {
			up := r.Intn(2) == 0
			if err := m.SetRounding(up); err != nil {
				t.Fatal(err)
			}
			mpfxRandomValue(r, m, a)
			mpfxRandomValue(r, m, b)
			ra, _ := m.Rational(a)
			rb, _ := m.Rational(b)
			for op := 0; op < 8; op++ {
				var exact *big.Rat
				var err error
				switch op {
				case 0:
					exact = new(big.Rat).Add(ra, rb)
					err = m.Add(a, b, c)
				case 1:
					exact = new(big.Rat).Sub(ra, rb)
					err = m.Sub(a, b, c)
				case 2:
					exact = new(big.Rat).Mul(ra, rb)
					err = m.Mul(a, b, c)
				case 3:
					if rb.Sign() == 0 {
						if e := m.Div(a, b, c); !errors.Is(e, ErrFixedPointDivZero) {
							t.Fatalf("%v: div by zero = %v", layout, e)
						}
						continue
					}
					exact = new(big.Rat).Quo(ra, rb)
					err = m.Div(a, b, c)
				case 4, 5:
					_ = m.Copy(c, a)
					if op == 4 {
						err = m.Floor(c)
					} else {
						err = m.Ceil(c)
					}
					q := new(big.Int).Quo(ra.Num(), ra.Denom())
					f := new(big.Rat).SetInt(q)
					if f.Cmp(ra) > 0 && op == 4 {
						f.Sub(f, big.NewRat(1, 1))
					}
					if f.Cmp(ra) < 0 && op == 5 {
						f.Add(f, big.NewRat(1, 1))
					}
					exact = f
				case 6:
					_ = m.Copy(c, a)
					err = m.Inc(c)
					exact = new(big.Rat).Add(ra, big.NewRat(1, 1))
				case 7:
					_ = m.Copy(c, a)
					err = m.Dec(c)
					exact = new(big.Rat).Sub(ra, big.NewRat(1, 1))
				}
				want, ok := mpfxOracleRound(exact, layout[0], layout[1], up)
				if !ok {
					if !errors.Is(err, ErrFixedPointOverflow) {
						t.Fatalf("%v op %d up=%v: %v, %v: want overflow, got %v", layout, op, up, ra, rb, err)
					}
					continue
				}
				if err != nil {
					t.Fatalf("%v op %d up=%v: %v, %v: %v", layout, op, up, ra, rb, err)
				}
				got, _ := m.Rational(c)
				if got.Cmp(want) != 0 {
					t.Fatalf("%v op %d up=%v: %v, %v = %v, want %v", layout, op, up, ra.RatString(), rb.RatString(), got.RatString(), want.RatString())
				}
			}
		}
		for _, v := range []*FixedPoint{a, b, c} {
			_ = v.Close()
		}
		if err := m.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestFixedPointDivisionCarryPastQuotientWords pins the quotient whose
// rounding carries out of its q_sz words: (2^64 - 2^-32) / 2^32 rounded up
// is 2^32. The source loses that carry in a stale buffer word and returns
// epsilon; g3 keeps it.
func TestFixedPointDivisionCarryPastQuotientWords(t *testing.T) {
	m, err := NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.New()
	b, _ := m.New()
	c, _ := m.New()
	m.allocateIfNeeded(a)
	a.words[0], a.words[1], a.words[2] = 0xffffffff, 0xffffffff, 0xffffffff
	if err := m.SetUint64(b, 1<<32); err != nil {
		t.Fatal(err)
	}
	if err := m.SetRounding(true); err != nil {
		t.Fatal(err)
	}
	if err := m.Div(a, b, c); err != nil {
		t.Fatal(err)
	}
	if got, _ := m.String(c); got != "4294967296" {
		t.Fatalf("quotient = %s, want 4294967296", got)
	}
}

// TestFixedPointConversionsMatchExactOracle checks SetFromRational,
// SetFromInteger, ToRational and String against math/big.
func TestFixedPointConversionsMatchExactOracle(t *testing.T) {
	qm, _ := rationalManager(t, false)
	r := rand.New(rand.NewSource(5))
	m, err := NewFixedPointManager(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := m.New()
	defer v.Close()
	q := NewRational()
	defer qm.Del(&q)
	for i := 0; i < 3000; i++ {
		up := r.Intn(2) == 0
		_ = m.SetRounding(up)
		num := new(big.Int).Rand(r, new(big.Int).Lsh(big.NewInt(1), uint(r.Intn(90)+1)))
		den := new(big.Int).Rand(r, new(big.Int).Lsh(big.NewInt(1), uint(r.Intn(70)+1)))
		den.Add(den, big.NewInt(1))
		exact := new(big.Rat).SetFrac(num, den)
		if err := qm.SetString(&q, exact.RatString()); err != nil {
			t.Fatal(err)
		}
		// A positive input with a fresh destination: the source's sign quirk
		// (it keeps the destination's sign) does not apply.
		m.reset(v)
		err := m.SetFromRational(v, qm, &q)
		want, ok := mpfxOracleRound(exact, 2, 1, up)
		if !ok {
			if !errors.Is(err, ErrFixedPointOverflow) {
				t.Fatalf("%v: want overflow, got %v", exact, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(exact, err)
		}
		got, _ := m.Rational(v)
		if got.Cmp(want) != 0 {
			t.Fatalf("SetFromRational(%v) = %v, want %v", exact.RatString(), got.RatString(), want.RatString())
		}
		back := NewRational()
		if err := m.ToRational(v, qm, &back); err != nil {
			t.Fatal(err)
		}
		text, _ := qm.String(&back)
		qm.Del(&back)
		if text != want.RatString() {
			t.Fatalf("ToRational = %s, want %s", text, want.RatString())
		}
		s, _ := m.String(v)
		wantS := want.Num().String()
		if !want.IsInt() {
			wantS += "/2"
			if k := want.Denom().BitLen() - 1; k > 1 {
				wantS += "^" + big.NewInt(int64(k)).String()
			}
		}
		if v.isZero() {
			wantS = "0"
		}
		if s != wantS {
			t.Fatalf("String = %s, want %s", s, wantS)
		}
	}
}
