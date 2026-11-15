package numeral

// Source: src/test/mpz.cpp rand_tst_gcd and tst_gcd_bug; source-only print
// cases receive added mathematical expectations, not invented source asserts.

import (
	"errors"
	"math"
	"math/big"
	"math/rand"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestIntegerScalarGCD(t *testing.T) {
	// Full finite signed-small domain for manager GCD, independently computed by
	// math/big, plus unsigned width-edge fixtures for the source binary helpers.
	m, _ := integerManager(t)
	var a, b, out Int
	for x := int64(-20); x <= 20; x++ {
		for y := int64(-20); y <= 20; y++ {
			m.SetInt64(&a, x)
			m.SetInt64(&b, y)
			expected := new(big.Int).GCD(nil, nil, big.NewInt(x), big.NewInt(y))
			if e := m.GCD(&a, &b, &out); e != nil || integerString(t, m, &out) != expected.String() {
				t.Fatal(x, y, e)
			}
		}
	}
	for _, pair := range [][2]uint64{{0, 0}, {0, math.MaxUint64}, {math.MaxUint64, 0}, {math.MaxUint64, math.MaxUint64}, {1 << 63, 3 << 61}, {math.MaxUint64, math.MaxUint32}, {4294967295, 2147483648}} {
		expected := new(big.Int).GCD(nil, nil, new(big.Int).SetUint64(pair[0]), new(big.Int).SetUint64(pair[1])).Uint64()
		if got := U64GCD(pair[0], pair[1]); got != expected {
			t.Fatal(pair, got, expected)
		}
		aa, bb := uint32(pair[0]), uint32(pair[1])
		want := new(big.Int).GCD(nil, nil, new(big.Int).SetUint64(uint64(aa)), new(big.Int).SetUint64(uint64(bb))).Uint64()
		if got := U32GCD(aa, bb); uint64(got) != want {
			t.Fatal(aa, bb, got, want)
		}
	}
}

func TestIntegerSourceGCDCoherence(t *testing.T) {
	// Port the active finite rand_tst_gcd(50,3,2) invariant. Its source random
	// prime-product input generator is replaced by a bounded independent one,
	// while the result is checked against math/big's independent exact GCD.
	// Production Power is intentionally not substituted into the oracle setup.
	primes := []int64{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47, 53}
	rng := rand.New(rand.NewSource(0x4c45484d))
	m, _ := integerManager(t)
	var a, b, out Int
	generate := func() *big.Int {
		z := big.NewInt(1)
		for _, prime := range primes {
			if rng.Intn(3) == 0 {
				p := big.NewInt(prime)
				if rng.Intn(2) == 0 {
					p.Mul(p, p)
				}
				z.Mul(z, p)
			}
		}
		if rng.Intn(2) == 0 {
			z.Neg(z)
		}
		return z
	}
	for i := 0; i < 50; i++ {
		aa, bb := generate(), generate()
		setIntegerOracle(t, m, &a, aa)
		setIntegerOracle(t, m, &b, bb)
		expected := new(big.Int).GCD(nil, nil, aa, bb)
		if e := m.GCD(&a, &b, &out); e != nil || integerString(t, m, &out) != expected.String() {
			t.Fatal("source coherence", i, e)
		}
	}
	// tst_gcd_bug only prints gcd(INT_MIN,INT_MIN). Added expectation: 2^31.
	a, b = NewInt(math.MinInt32), NewInt(math.MinInt32)
	if e := m.GCD(&a, &b, &out); e != nil || integerString(t, m, &out) != "2147483648" {
		t.Fatal("INT_MIN source print case", e)
	}
}

func TestIntegerGCDSignedLimbAliases(t *testing.T) {
	m, _ := integerManager(t)
	rng := rand.New(rand.NewSource(0x474344))
	var a, b, out Int
	// 250 cases at most 16 limbs, plus exact zero/equal/one and Fibonacci pairs
	// that exercise multiple Lehmer matrix updates rather than only remainders.
	cases := [][2]*big.Int{{big.NewInt(0), big.NewInt(math.MinInt64)}, {big.NewInt(math.MinInt64), big.NewInt(0)}, {big.NewInt(0), big.NewInt(0)}, {big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 200)}}
	x, y := big.NewInt(1), big.NewInt(1)
	for i := 0; i < 400; i++ {
		x, y = y, new(big.Int).Add(x, y)
	}
	cases = append(cases, [2]*big.Int{x, y}, [2]*big.Int{new(big.Int).Neg(new(big.Int).Set(x)), new(big.Int).Set(x)})
	for i := 0; i < 250; i++ {
		aa, bb := make([]uint32, 1+rng.Intn(16)), make([]uint32, 1+rng.Intn(16))
		for j := range aa {
			aa[j] = rng.Uint32()
		}
		for j := range bb {
			bb[j] = rng.Uint32()
		}
		av, bv := oracle(aa), oracle(bb)
		if i%2 == 0 {
			av.Neg(av)
		}
		if i%3 == 0 {
			bv.Neg(bv)
		}
		cases = append(cases, [2]*big.Int{av, bv})
	}
	for i, pair := range cases {
		expected := new(big.Int).GCD(nil, nil, pair[0], pair[1])
		for alias := 0; alias < 3; alias++ {
			setIntegerOracle(t, m, &a, pair[0])
			setIntegerOracle(t, m, &b, pair[1])
			dst := &out
			if alias == 1 {
				dst = &a
			}
			if alias == 2 {
				dst = &b
			}
			if e := m.GCD(&a, &b, dst); e != nil || integerString(t, m, dst) != expected.String() {
				t.Fatal("gcd", i, alias, e)
			}
		}
	}
}

func TestIntegerAddMulSubMulPrerequisites(t *testing.T) {
	m, _ := integerManager(t)
	var a, b, c, out Int
	values := []int64{0, 1, -1, math.MinInt32, math.MaxInt32, 1 << 40, -(1 << 40), math.MinInt64}
	for _, av := range values {
		for _, bv := range values {
			for _, cv := range values {
				for _, sub := range []bool{false, true} {
					for alias := 0; alias < 4; alias++ {
						m.SetInt64(&a, av)
						m.SetInt64(&b, bv)
						m.SetInt64(&c, cv)
						dst := &out
						if alias == 1 {
							dst = &a
						}
						if alias == 2 {
							dst = &b
						}
						if alias == 3 {
							dst = &c
						}
						expected := new(big.Int).Mul(big.NewInt(bv), big.NewInt(cv))
						if sub {
							expected.Sub(big.NewInt(av), expected)
						} else {
							expected.Add(big.NewInt(av), expected)
						}
						var e error
						if sub {
							e = m.SubMul(&a, &b, &c, dst)
						} else {
							e = m.AddMul(&a, &b, &c, dst)
						}
						if e != nil || integerString(t, m, dst) != expected.String() {
							t.Fatal(av, bv, cv, sub, alias, e)
						}
					}
				}
			}
		}
	}
}

func TestIntegerGCDFailureCleanup(t *testing.T) {
	// Consecutive Fibonacci numbers yield large Lehmer batches and gcd=1.
	x, y := big.NewInt(1), big.NewInt(1)
	for i := 0; i < 400; i++ {
		x, y = y, new(big.Int).Add(x, y)
	}
	for alias := 0; alias < 3; alias++ {
		failures, successes := 0, 0
		for after := int64(0); after < 2048; after++ {
			m, budget := integerManager(t)
			var a, b Int
			out := NewInt(17)
			setIntegerOracle(t, m, &a, x)
			setIntegerOracle(t, m, &b, y)
			dst := &out
			if alias == 1 {
				dst = &a
			}
			if alias == 2 {
				dst = &b
			}
			budget.FailAfter(after)
			err := m.GCD(&a, &b, dst)
			if err != nil {
				failures++
				if !numericPhysicalReservationRefusal(err) {
					t.Fatal(alias, after, err)
				}
				// Source big_set/result setters may complete publication before failure.
				mpzPublicationAudit(t, m, &a, &b, &out)
				budget.FailAfter(-1)
				m.Close()
				if budget.Used() != 0 {
					t.Fatal("failed source roots leaked", budget.Used())
				}
				continue
			} else {
				successes++
			}
			budget.FailAfter(-1)
			if integerString(t, m, dst) != "1" {
				t.Fatal("gcd fibonacci", alias, after)
			}
			m.Close()
			if budget.Used() != 0 {
				t.Fatal("close budget", budget.Used())
			}
			break
		}
		if failures == 0 || successes == 0 {
			t.Fatal("fault sweep missed completion boundary", alias, failures, successes)
		}
	}
}

func TestIntegerAddMulSubMulFailureCleanup(t *testing.T) {
	aa, bb, cc := make([]uint32, 33), make([]uint32, 13), make([]uint32, 9)
	for i := range aa {
		aa[i] = uint32(i*17 + 3)
	}
	for i := range bb {
		bb[i] = uint32(i*13 + 5)
	}
	for i := range cc {
		cc[i] = uint32(i*7 + 9)
	}
	av, bv, cv := oracle(aa), oracle(bb), oracle(cc)
	for _, sub := range []bool{false, true} {
		for alias := 0; alias < 4; alias++ {
			failures, successes := 0, 0
			expected := new(big.Int).Mul(bv, cv)
			if sub {
				expected.Sub(av, expected)
			} else {
				expected.Add(av, expected)
			}
			for after := int64(0); after < 24; after++ {
				m, budget := integerManager(t)
				var a, b, c Int
				out := NewInt(17)
				setIntegerOracle(t, m, &a, av)
				setIntegerOracle(t, m, &b, bv)
				setIntegerOracle(t, m, &c, cv)
				dst := &out
				if alias == 1 {
					dst = &a
				}
				if alias == 2 {
					dst = &b
				}
				if alias == 3 {
					dst = &c
				}
				budget.FailAfter(after)
				var err error
				if sub {
					err = m.SubMul(&a, &b, &c, dst)
				} else {
					err = m.AddMul(&a, &b, &c, dst)
				}
				if err != nil {
					failures++
					if !numericPhysicalReservationRefusal(err) {
						t.Fatal(alias, after, err)
					}
					// Source big_set/result setters may complete publication before failure.
					mpzPublicationAudit(t, m, &a, &b, &c, &out)
					budget.FailAfter(-1)
					m.Close()
					if budget.Used() != 0 {
						t.Fatal("failed source roots leaked", budget.Used())
					}
					continue
				} else {
					successes++
				}
				budget.FailAfter(-1)
				if integerString(t, m, dst) != expected.String() {
					t.Fatal("fused operation", sub, alias, after)
				}
				m.Close()
				if budget.Used() != 0 {
					t.Fatal("fused operation budget leak", budget.Used())
				}
			}
			if failures == 0 || successes == 0 {
				t.Fatal("fault sweep completion boundary", sub, alias, failures, successes)
			}
		}
	}
}

func TestIntegerGCDZeroPromotionFailure(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		owner, _ := rationalManager(t, false)
		m := owner.Z
		zero, minimum, out := NewInt(0), NewInt(math.MinInt32), NewInt(17)
		// The source Copy+Abs zero branch promotes INT_MIN. Exhaust cells so that
		// failure occurs after the actual caller Copy and before the promotion
		// setter assigns a new pointer/sign. The caller remains valid inline MIN.
		m.Cells.config.SlotLimit = m.Cells.Stats().Slots
		a, b := &zero, &minimum
		if reverse {
			a, b = b, a
		}
		e := m.GCD(a, b, &out)
		qopsAuditLiveOwnership(t, owner, []*Int{&zero, &minimum, &out})
		if !errors.Is(e, arena.ErrSlotLimit) || out != NewInt(math.MinInt32) || zero != NewInt(0) || minimum != NewInt(math.MinInt32) {
			t.Fatal("source caller Copy before promotion refusal", reverse, e, out)
		}
	}
}

func TestIntegerGCDAndFusedWarmAllocations(t *testing.T) {
	m, budget := integerManager(t)
	var a, b, c, g, out Int
	x, y := big.NewInt(1), big.NewInt(1)
	for i := 0; i < 400; i++ {
		x, y = y, new(big.Int).Add(x, y)
	}
	setIntegerOracle(t, m, &a, x)
	setIntegerOracle(t, m, &b, y)
	m.SetInt64(&c, 1<<40)
	if e := m.GCD(&a, &b, &g); e != nil {
		t.Fatal(e)
	}
	if e := m.AddMul(&a, &b, &c, &out); e != nil {
		t.Fatal(e)
	}
	// Prime SubMul too: the retained graph must plateau across every route.
	if e := m.SubMul(&a, &b, &c, &out); e != nil {
		t.Fatal(e)
	}
	// Prime the same complete measured cycle with out's big root now live.
	// The first GCD ran before AddMul created it; paired dead metadata can make
	// the later, genuinely larger descriptor peak require another real page.
	if e := m.GCD(&a, &b, &g); e != nil {
		t.Fatal(e)
	}
	if e := m.AddMul(&a, &b, &c, &out); e != nil {
		t.Fatal(e)
	}
	if e := m.SubMul(&a, &b, &c, &out); e != nil {
		t.Fatal(e)
	}
	beforeWarm := numericWarmProfileSnapshot(t, m, budget)
	var operationError error
	allocations := testing.AllocsPerRun(100, func() {
		operationError = m.GCD(&a, &b, &g)
		if operationError == nil {
			operationError = m.AddMul(&a, &b, &c, &out)
		}
		if operationError == nil {
			operationError = m.SubMul(&a, &b, &c, &out)
		}
	})
	if operationError != nil {
		t.Fatal("warmed gcd/fused operations allocate", allocations, operationError)
	}
	numericWarmProfileAudit(t, m, budget, beforeWarm, allocations, modularSourceDebug)
	integerPhysicalAudit(t, m, &a, &b, &c, &g, &out)
	if integerString(t, m, &g) != "1" {
		t.Fatal("gcd warm result")
	}
}

// TestIntegerGCDWordShortcutMatchesLehmer checks the Go-only one-word GCD
// shortcut against the Lehmer path (forced by an armed, never reached
// injection): same value and same inline/big representation, for word-sized
// operands and for a big operand against an inline one, with aliasing.
func TestIntegerGCDWordShortcutMatchesLehmer(t *testing.T) {
	m, b := integerManager(t)
	rng := rand.New(rand.NewSource(0x67636477))
	edges := []string{"0", "1", "-1", "3", "2147483647", "-2147483647", "-2147483648", "2147483648", "4294967295",
		"4294967296", "9223372036854775807", "-9223372036854775808", "18446744073709551615", "-18446744073709551615",
		"18446744073709551616", "340282366920938463463374607431768211455", "-170141183460469231731687303715884105728"}
	values := append([]string{}, edges...)
	for i := 0; i < 300; i++ {
		v := new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), uint(1+rng.Intn(100))))
		if rng.Intn(4) == 0 {
			v.Mul(v, big.NewInt(int64(1+rng.Intn(1<<20))))
		}
		if rng.Intn(2) == 0 {
			v.Neg(v)
		}
		values = append(values, v.String())
	}
	run := func(x, y string, lehmer bool, alias int) (string, bool) {
		var a, c, out Int
		if e := m.SetString(&a, x); e != nil {
			t.Fatal(e)
		}
		if e := m.SetString(&c, y); e != nil {
			t.Fatal(e)
		}
		dst := &out
		if alias == 1 {
			dst = &a
		} else if alias == 2 {
			dst = &c
		}
		if lehmer {
			b.FailAfter(1 << 40)
		}
		e := m.GCD(&a, &c, dst)
		b.FailAfter(-1)
		if e != nil {
			t.Fatal(x, y, e)
		}
		s, small := integerString(t, m, dst), dst.IsSmall()
		m.Del(&a)
		m.Del(&c)
		m.Del(&out)
		return s, small
	}
	for i, x := range values {
		for j := 0; j < 40; j++ {
			y := values[(i*31+j*7)%len(values)]
			if j < len(edges) {
				y = edges[j]
			}
			alias := (i + j) % 3
			fast, fastSmall := run(x, y, false, alias)
			slow, slowSmall := run(x, y, true, alias)
			bx, _ := new(big.Int).SetString(x, 10)
			by, _ := new(big.Int).SetString(y, 10)
			want := new(big.Int).GCD(nil, nil, new(big.Int).Abs(bx), new(big.Int).Abs(by)).String()
			if fast != slow || fastSmall != slowSmall || fast != want {
				t.Fatal(x, y, alias, fast, slow, want, fastSmall, slowSmall)
			}
		}
	}
}
