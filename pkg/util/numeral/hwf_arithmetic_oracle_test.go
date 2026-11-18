package numeral

import (
	"math"
	"math/big"
	"math/rand"
	"testing"
)

// hwfBigOracle is an independent exact model of directed binary64 rounding:
// it computes the exact rational result with math/big and rounds it.
func hwfBigOracle(mode ArbitraryFloatRoundingMode, a, b float64, op byte) float64 {
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) || op == '/' && b == 0 {
		switch op {
		case '+':
			return a + b
		case '-':
			return a - b
		case '*':
			return a * b
		}
		return a / b
	}
	ra, rb := new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(b)
	exact := new(big.Rat)
	negativeZero := false
	switch op {
	case '+':
		exact.Add(ra, rb)
		negativeZero = (a == 0 && b == 0 && math.Signbit(a) && math.Signbit(b)) || (mode == ArbitraryFloatTowardNegative && (a != 0 || b != 0 || math.Signbit(a) != math.Signbit(b)))
	case '-':
		exact.Sub(ra, rb)
		negativeZero = (a == 0 && b == 0 && math.Signbit(a) && !math.Signbit(b)) || (mode == ArbitraryFloatTowardNegative && (a != 0 || b != 0 || math.Signbit(a) == math.Signbit(b)))
	case '*':
		exact.Mul(ra, rb)
		negativeZero = math.Signbit(a) != math.Signbit(b)
	case '/':
		exact.Quo(ra, rb)
		negativeZero = math.Signbit(a) != math.Signbit(b)
	}
	if exact.Sign() == 0 {
		if negativeZero {
			return math.Copysign(0, -1)
		}
		return 0
	}
	// Rat.Float64's exactness flag is not trusted: it reports an exact
	// result for values far below the smallest subnormal that round to 0.
	nearest, _ := exact.Float64()
	if mode == ArbitraryFloatNearestEven {
		return nearest
	}
	var relation int
	if math.IsInf(nearest, 1) {
		relation = 1
	} else if math.IsInf(nearest, -1) {
		relation = -1
	} else {
		relation = new(big.Rat).SetFloat64(nearest).Cmp(exact)
	}
	if (mode == ArbitraryFloatTowardPositive || mode == ArbitraryFloatTowardZero && exact.Sign() < 0) && relation < 0 {
		return math.Nextafter(nearest, math.Inf(1))
	}
	if (mode == ArbitraryFloatTowardNegative || mode == ArbitraryFloatTowardZero && exact.Sign() > 0) && relation > 0 {
		return math.Nextafter(nearest, math.Inf(-1))
	}
	return nearest
}

func hwfOracleCheck(t *testing.T, m *HardwareFloatManager, a, b float64) {
	t.Helper()
	modes := []ArbitraryFloatRoundingMode{ArbitraryFloatNearestEven, ArbitraryFloatTowardPositive, ArbitraryFloatTowardNegative, ArbitraryFloatTowardZero}
	for _, mode := range modes {
		for _, op := range []byte{'+', '-', '*', '/'} {
			x, y, out := HardwareFloat{a}, HardwareFloat{b}, HardwareFloat{}
			var e error
			switch op {
			case '+':
				e = m.Add(mode, &x, &y, &out)
			case '-':
				e = m.Sub(mode, &x, &y, &out)
			case '*':
				e = m.Mul(mode, &x, &y, &out)
			case '/':
				e = m.Div(mode, &x, &y, &out)
			}
			if e != nil {
				t.Fatal(e)
			}
			want := hwfBigOracle(mode, a, b, op)
			if math.IsNaN(want) && math.IsNaN(out.value) {
				continue
			}
			if math.Float64bits(want) != math.Float64bits(out.value) {
				t.Fatalf("mode %v: %x %c %x = %x, oracle %x (%g %c %g = %g, want %g)", mode, math.Float64bits(a), op, math.Float64bits(b), math.Float64bits(out.value), math.Float64bits(want), a, op, b, out.value, want)
			}
		}
	}
}

// TestHardwareFloatDirectedRoundingMatchesExactOracle checks the
// double-only directed rounding against exact math/big rounding on edge
// values (zeros, subnormals, the normal boundary, overflow, ties) and on
// random operands, including operands whose results underflow or overflow.
func TestHardwareFloatDirectedRoundingMatchesExactOracle(t *testing.T) {
	m := NewHardwareFloatManager()
	minSub := math.Float64frombits(1)
	minNorm := math.Float64frombits(0x0010000000000000)
	edges := []float64{0, math.Copysign(0, -1), minSub, 2 * minSub, 3 * minSub, minNorm, math.Nextafter(minNorm, 0), math.Nextafter(minNorm, 1),
		math.MaxFloat64, math.Nextafter(math.MaxFloat64, 0), 1, 1.5, 3, 1.0 / 3, 0.1, math.Nextafter(1, 2), math.Nextafter(1, 0),
		math.Ldexp(1, 512), math.Ldexp(1, -512), math.Ldexp(1, -537), math.Ldexp(1, 1023), math.Ldexp(1, -1022), math.Ldexp(1.5, -1074+1),
		math.Ldexp(1, 53), math.Ldexp(1, 53) + 2, math.Inf(1), math.Inf(-1), math.NaN(), 1e308, 1e-308, 2.5e-324, 4.9e-324}
	var all []float64
	for _, v := range edges {
		all = append(all, v, -v)
	}
	for _, a := range all {
		for _, b := range all {
			hwfOracleCheck(t, m, a, b)
		}
	}
	r := rand.New(rand.NewSource(4711))
	n := 20000
	if testing.Short() {
		n = 300000
	}
	for i := 0; i < n; i++ {
		var a, b float64
		switch i % 4 {
		case 0: // any finite bit patterns
			a = math.Float64frombits(r.Uint64() &^ (0x7ff << 52) | uint64(r.Intn(0x7ff))<<52)
			b = math.Float64frombits(r.Uint64() &^ (0x7ff << 52) | uint64(r.Intn(0x7ff))<<52)
		case 1: // ordinary magnitudes
			a = (r.Float64() - 0.5) * math.Ldexp(1, r.Intn(64)-32)
			b = (r.Float64() - 0.5) * math.Ldexp(1, r.Intn(64)-32)
		case 2: // products and quotients near the subnormal range
			a = (r.Float64() + 0.5) * math.Ldexp(1, -r.Intn(560))
			b = (r.Float64() - 0.5) * math.Ldexp(1, -r.Intn(560))
		case 3: // near cancellation
			a = (r.Float64() + 0.5) * math.Ldexp(1, r.Intn(200)-100)
			b = -math.Nextafter(a, math.Inf(int(r.Intn(2)*2-1)))
			if r.Intn(2) == 0 {
				b = math.Ldexp(b, -r.Intn(60))
			}
		}
		hwfOracleCheck(t, m, a, b)
	}
}

// TestHardwareFloatToRationalStringMatchesBigOracle checks the mpq-based
// rational string against an independent math/big rendering of the same
// bit decomposition.
func TestHardwareFloatToRationalStringMatchesBigOracle(t *testing.T) {
	m := NewHardwareFloatManager()
	r := rand.New(rand.NewSource(17))
	values := []float64{0, math.Copysign(0, -1), 1, -1.5, math.MaxFloat64, math.Float64frombits(1), math.Float64frombits(0x000fffffffffffff), math.Ldexp(1, -1022), 1e300, 1e-300, 0.1}
	for i := 0; i < 2000; i++ {
		values = append(values, math.Float64frombits(r.Uint64()&^(0x7ff<<52)|uint64(r.Intn(0x7ff))<<52))
	}
	for _, v := range values {
		x := HardwareFloat{v}
		got, e := m.ToRationalString(&x)
		if e != nil {
			t.Fatal(e)
		}
		sig := m.Significand(&x)
		if m.IsNormal(&x) {
			sig |= uint64(1) << 52
		}
		n := new(big.Int).SetUint64(sig)
		if m.Sign(&x) {
			n.Neg(n)
		}
		d := new(big.Int).Lsh(big.NewInt(1), 52)
		if exp := m.Exponent(&x); exp >= 0 {
			n.Lsh(n, uint(exp))
		} else {
			d.Lsh(d, uint(-exp))
		}
		if want := new(big.Rat).SetFrac(n, d).RatString(); got != want {
			t.Fatalf("%x: %q, want %q", math.Float64bits(v), got, want)
		}
	}
}
