package numeral

import (
	"math/big"
	"math/rand"
	"testing"
)

// TestIntegerCRACoefficientMatchesSourceOperations checks CRACoefficient
// against the source mul, mul, add, mod, gt, sub sequence on the same
// manager: same value and same inline/big representation.
func TestIntegerCRACoefficientMatchesSourceOperations(t *testing.T) {
	m, _ := integerManager(t)
	rng := rand.New(rand.NewSource(0x637261))
	random := func(bitsMax int) string {
		v := new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), uint(1+rng.Intn(bitsMax))))
		if rng.Intn(2) == 0 {
			v.Neg(v)
		}
		if rng.Intn(5) == 0 {
			v.SetInt64(0)
		}
		return v.String()
	}
	set := func(a *Int, s string) {
		if e := m.SetString(a, s); e != nil {
			t.Fatal(e)
		}
	}
	var scratch CRAScratch
	for i := 0; i < 400; i++ {
		var x, y, a1, a2, bound, upper, two, fast, tmp1, tmp2, tmp3, slow Int
		b := new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), uint(2+rng.Intn(900))))
		b.SetBit(b, 0, 1)
		if b.Cmp(big.NewInt(3)) < 0 {
			b.SetInt64(3)
		}
		set(&bound, b.String())
		two = NewInt(2)
		if e := m.Div(&bound, &two, &upper); e != nil {
			t.Fatal(e)
		}
		set(&x, random(40))
		set(&y, random(900))
		set(&a1, random(900))
		set(&a2, random(64))
		if i%7 == 0 {
			set(&x, "-2147483648")
		}
		done, e := m.CRACoefficient(&x, &y, &a1, &a2, &bound, &upper, &fast, &scratch)
		if e != nil || !done {
			t.Fatal(done, e)
		}
		for _, step := range []error{m.Mul(&x, &a1, &tmp1), m.Mul(&y, &a2, &tmp2), m.Add(&tmp1, &tmp2, &tmp3), m.Mod(&tmp3, &bound, &slow)} {
			if step != nil {
				t.Fatal(step)
			}
		}
		if gt, e := m.Greater(&slow, &upper); e != nil {
			t.Fatal(e)
		} else if gt {
			if e := m.Sub(&slow, &bound, &slow); e != nil {
				t.Fatal(e)
			}
		}
		if integerString(t, m, &fast) != integerString(t, m, &slow) || fast.IsSmall() != slow.IsSmall() {
			t.Fatal(i, integerString(t, m, &fast), integerString(t, m, &slow))
		}
		for _, v := range []*Int{&x, &y, &a1, &a2, &bound, &upper, &fast, &tmp1, &tmp2, &tmp3, &slow} {
			m.Del(v)
		}
	}
}

// TestModularDirectMulAddMatchesSourceOperations checks DirectMulAdd (and
// its word path) against mpzzp addmul/submul in Z mode and for inline,
// word and big moduli: same value and same inline/big representation.
func TestModularDirectMulAddMatchesSourceOperations(t *testing.T) {
	m, _ := integerManager(t)
	z, e := NewIntegerModularManager(m)
	if e != nil {
		t.Fatal(e)
	}
	rng := rand.New(rand.NewSource(0x6d6164))
	random := func(bitsMax int) string {
		v := new(big.Int).Rand(rng, new(big.Int).Lsh(big.NewInt(1), uint(1+rng.Intn(bitsMax))))
		if rng.Intn(2) == 0 {
			v.Neg(v)
		}
		if rng.Intn(6) == 0 {
			v.SetInt64(0)
		}
		return v.String()
	}
	set := func(a *Int, s string) {
		if e := m.SetString(a, s); e != nil {
			t.Fatal(e)
		}
	}
	var scratch CRAScratch
	for _, modulus := range []string{"", "40009", "2305843009213693951", "340282366920938463463374607431768211507", "1606938044258990275541962092341162602522202993782792835301611"} {
		var p Int
		if modulus == "" {
			if e := z.SetZ(); e != nil {
				t.Fatal(e)
			}
		} else {
			set(&p, modulus)
			if e := z.SetZP(&p); e != nil {
				t.Fatal(e)
			}
		}
		for i := 0; i < 300; i++ {
			var a, b, c, fast, slow Int
			bitsMax := 300
			if modulus != "" {
				bitsMax = len(modulus) * 3
			}
			for _, v := range []*Int{&a, &b, &c} {
				set(v, random(bitsMax))
				if modulus != "" {
					if e := z.PNormalize(v); e != nil {
						t.Fatal(e)
					}
				}
			}
			subtract := i%2 == 1
			if done, e := z.DirectMulAdd(&a, &b, &c, &fast, subtract, &scratch); e != nil || !done {
				t.Fatal(done, e)
			}
			if subtract {
				e = z.SubMul(&a, &b, &c, &slow)
			} else {
				e = z.AddMul(&a, &b, &c, &slow)
			}
			if e != nil {
				t.Fatal(e)
			}
			if integerString(t, m, &fast) != integerString(t, m, &slow) || fast.IsSmall() != slow.IsSmall() {
				t.Fatal(modulus, i, integerString(t, m, &fast), integerString(t, m, &slow))
			}
			for _, v := range []*Int{&a, &b, &c, &fast, &slow} {
				m.Del(v)
			}
		}
		m.Del(&p)
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
}
