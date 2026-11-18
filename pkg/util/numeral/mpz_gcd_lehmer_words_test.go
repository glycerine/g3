package numeral

import (
	"math/big"
	"math/rand"
	"testing"
)

// TestIntegerGCDLehmerWordStepMatchesSource checks GCDDirect's cofactor
// update of the Lehmer loop against the source mul/addmul sequence (forced by an
// armed, never reached injection) and math/big: same value and same
// inline/big representation, with aliasing, for operands of many digits
// and common factors.
func TestIntegerGCDLehmerWordStepMatchesSource(t *testing.T) {
	m, b := integerManager(t)
	rng := rand.New(rand.NewSource(0x6c65686d))
	var values []string
	common := new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), 700))
	for i := 0; i < 60; i++ {
		v := new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), uint(64+rng.Intn(2500))))
		if i%3 == 0 {
			v.Mul(v, common)
		}
		if rng.Intn(2) == 0 {
			v.Neg(v)
		}
		values = append(values, v.String())
	}
	run := func(x, y string, armed bool, alias int) (string, bool) {
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
		if armed {
			b.FailAfter(1 << 40)
		}
		e := m.GCDDirect(&a, &c, dst)
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
		for j := 0; j < 12; j++ {
			y := values[(i*7+j*13+1)%len(values)]
			alias := (i + j) % 3
			fast, fastSmall := run(x, y, false, alias)
			slow, slowSmall := run(x, y, true, alias)
			bx, _ := new(big.Int).SetString(x, 10)
			by, _ := new(big.Int).SetString(y, 10)
			want := new(big.Int).GCD(nil, nil, new(big.Int).Abs(bx), new(big.Int).Abs(by)).String()
			if fast != slow || fastSmall != slowSmall || fast != want {
				t.Fatal(i, j, alias, "fast", fast, "slow", slow, "want", want, fastSmall, slowSmall)
			}
		}
	}
}
