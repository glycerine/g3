package numeral

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

// smallFastState is everything an inline fast path may touch besides the
// destination: the raw local scopes and the scratch members.
type smallFastState struct {
	Raw                                 rationalRawArithmeticScope
	Generation                          uint64
	NTmp, AddTmp1, AddTmp2              Int
	AddMulTmp, LtTmp1, LtTmp2, FusedTmp Rational
	FusedPending, FusedExecuting        bool
	FusedCursor                         uint8
}

func snapshotSmallFast(m *RationalManager) smallFastState {
	return smallFastState{m.RawArithmetic, m.RawGeneration, m.NTmp, m.AddTmp1, m.AddTmp2, m.AddMulTmp, m.LtTmp1, m.LtTmp2, m.fusedTemporary, m.fusedPending, m.fusedExecuting, m.fusedCursor}
}

// The public entry points take the inline paths; the internal ones they
// fall back to are the full source paths. Both must give the same value,
// the same error and the same manager state, including at the int32
// boundary where z3 would allocate cells and the fast path must decline.
func TestRationalSmallFastPathMatchesFullPath(t *testing.T) {
	edges := []int64{0, 1, -1, 2, -2, 3, 6, 7, -7, 46340, 46341, -46341, 65536, math.MaxInt32, math.MaxInt32 - 1, math.MinInt32, math.MinInt32 + 1}
	for _, synch := range []bool{false, true} {
		fast, _ := rationalManager(t, synch)
		full, _ := rationalManager(t, synch)
		r := rand.New(rand.NewSource(29))
		pick := func() int64 {
			switch r.Intn(3) {
			case 0:
				return edges[r.Intn(len(edges))]
			case 1:
				return int64(r.Intn(41) - 20)
			}
			return int64(int32(r.Uint32()))
		}
		set := func(m *RationalManager, q *Rational, n, d int64) {
			if d == 0 {
				d = 1
			}
			if d < 0 {
				n, d = -n, -d
			}
			if !fitsInt32(n) {
				n = math.MaxInt32
			}
			// SetFractionSigned keeps every int32 component inline (SetFraction's
			// uint64 denominator would make INT_MAX a cell).
			if e := m.SetFractionSigned(q, int32(n), int32(d)); e != nil {
				t.Fatal(e)
			}
		}
		type op struct {
			name       string
			fast, full func(m *RationalManager, a, b, c, out *Rational) (int, error)
		}
		ops := []op{
			{"add", func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.Add(a, b, out) },
				func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.addSub(a, b, out, false) }},
			{"sub", func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.Sub(a, b, out) },
				func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.addSub(a, b, out, true) }},
			{"mul", func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.Mul(a, b, out) },
				func(m *RationalManager, a, b, c, out *Rational) (int, error) {
					return 0, m.mulWithAllocationContext(a, b, out, numeralContextRef(integerAllocationInvocationContext{}))
				}},
			{"div", func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.Div(a, b, out) },
				func(m *RationalManager, a, b, c, out *Rational) (int, error) {
					return 0, m.divWithAllocationContext(a, b, out, integerAllocationInvocationContext{})
				}},
			{"cmp", func(m *RationalManager, a, b, c, out *Rational) (int, error) { return m.Compare(a, b) },
				func(m *RationalManager, a, b, c, out *Rational) (int, error) {
					return m.compareWithAllocationContext(a, b, numeralContextRef(integerAllocationInvocationContext{}))
				}},
			{"eq", func(m *RationalManager, a, b, c, out *Rational) (int, error) {
				v, e := m.Equal(a, b)
				if v {
					return 1, e
				}
				return 0, e
			}, func(m *RationalManager, a, b, c, out *Rational) (int, error) {
				if e := m.checkScalarValue(a); e != nil {
					return 0, e
				}
				n, e := m.Z.CompareFromOwners(m.Z, &a.Num, m.Z, &b.Num)
				if e != nil || n != 0 {
					return 0, e
				}
				d, e := m.Z.CompareFromOwners(m.Z, &a.Den, m.Z, &b.Den)
				if d == 0 {
					return 1, e
				}
				return 0, e
			}},
			{"copy", func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.Copy(out, a) },
				func(m *RationalManager, a, b, c, out *Rational) (int, error) {
					return 0, m.copyWithAllocationContext(out, a, numeralContextRef(integerAllocationInvocationContext{}))
				}},
			{"neg", func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.Neg(a) },
				func(m *RationalManager, a, b, c, out *Rational) (int, error) {
					return 0, m.negWithSourceContext(a, numeralContextRef(integerAllocationInvocationContext{}))
				}},
			{"addmul", func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.AddMul(a, b, c, out) },
				func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.fused(a, b, c, out, false) }},
			{"submul", func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.SubMul(a, b, c, out) },
				func(m *RationalManager, a, b, c, out *Rational) (int, error) { return 0, m.fused(a, b, c, out, true) }},
		}
		for round := 0; round < 30000; round++ {
			o := ops[r.Intn(len(ops))]
			vals := [8]int64{}
			for i := range vals {
				vals[i] = pick()
			}
			if r.Intn(3) == 0 {
				vals[1], vals[3], vals[5] = 1, 1, 1
			}
			if r.Intn(4) == 0 {
				vals[2], vals[3] = 1, 1
			}
			if o.name == "div" && vals[2] == 0 {
				vals[2] = 1
			}
			alias := r.Intn(4)
			var got [2][2]string
			var res [2]int
			var errs [2]error
			var states [2]smallFastState
			for side, m := range []*RationalManager{fast, full} {
				a, b, c, out := NewRational(), NewRational(), NewRational(), NewRational()
				set(m, &a, vals[0], vals[1])
				set(m, &b, vals[2], vals[3])
				set(m, &c, vals[4], vals[5])
				set(m, &out, vals[6], vals[7])
				dst := &out
				switch alias {
				case 1:
					dst = &a
				case 2:
					dst = &b
				}
				f := o.fast
				if side == 1 {
					f = o.full
				}
				res[side], errs[side] = f(m, &a, &b, &c, dst)
				states[side] = snapshotSmallFast(m)
				if errs[side] == nil {
					got[side] = [2]string{rationalString(t, m, dst), rationalString(t, m, &a)}
				}
				for _, q := range []*Rational{&a, &b, &c, &out} {
					if e := m.Del(q); e != nil {
						t.Fatal(e)
					}
				}
			}
			if res[0] != res[1] || (errs[0] == nil) != (errs[1] == nil) || got[0] != got[1] {
				t.Fatalf("synch=%v %s %v alias %d: fast %v %v %v, full %v %v %v", synch, o.name, vals, alias, res[0], errs[0], got[0], res[1], errs[1], got[1])
			}
			if !reflect.DeepEqual(states[0], states[1]) {
				t.Fatalf("synch=%v %s %v alias %d: state\nfast %+v\nfull %+v", synch, o.name, vals, alias, states[0], states[1])
			}
		}
	}
}

// Integer Div, GCD, Copy and Reset: the inline paths against the full ones,
// including aliased destinations and INT_MIN.
func TestIntegerSmallFastPathMatchesFullPath(t *testing.T) {
	fast, full := smallPathManager(t), smallPathManager(t)
	edges := []int64{0, 1, -1, 2, -2, 7, -7, 46341, math.MaxInt32, math.MinInt32, math.MinInt32 + 1}
	r := rand.New(rand.NewSource(31))
	pick := func() int64 {
		if r.Intn(2) == 0 {
			return edges[r.Intn(len(edges))]
		}
		return int64(r.Intn(201) - 100)
	}
	for round := 0; round < 20000; round++ {
		x, y, z := pick(), pick(), pick()
		op, alias := r.Intn(4), r.Intn(3)
		if op == 0 && y == 0 {
			y = 3
		}
		var got [2]string
		var errs [2]error
		for side, m := range []*IntegerManager{fast, full} {
			a, b, c := NewInt(0), NewInt(0), NewInt(0)
			for _, p := range []struct {
				q *Int
				v int64
			}{{&a, x}, {&b, y}, {&c, z}} {
				if e := m.SetInt64(p.q, p.v); e != nil {
					t.Fatal(e)
				}
			}
			dst := &c
			if alias == 1 {
				dst = &a
			} else if alias == 2 {
				dst = &b
			}
			switch op {
			case 0:
				if side == 0 {
					errs[side] = m.Div(&a, &b, dst)
				} else {
					errs[side] = m.divWithAllocationContext(&a, &b, dst, integerAllocationInvocationContext{})
				}
			case 1:
				if side == 0 {
					errs[side] = m.GCD(&a, &b, dst)
				} else {
					errs[side] = m.gcdWithAllocationContext(&a, &b, dst, integerAllocationInvocationContext{})
				}
			case 2:
				if side == 0 {
					errs[side] = m.Copy(dst, &a)
				} else {
					errs[side] = m.CopyFromOwner(dst, m, &a)
				}
			case 3:
				if side == 0 {
					errs[side] = m.Reset(dst)
				} else {
					if errs[side] = m.Del(dst); errs[side] == nil {
						dst.Val = 0
					}
				}
			}
			if errs[side] == nil {
				s, e := m.String(dst)
				if e != nil {
					t.Fatal(e)
				}
				got[side] = s
			}
			for _, q := range []*Int{&a, &b, &c} {
				if e := m.Del(q); e != nil {
					t.Fatal(e)
				}
			}
		}
		if got[0] != got[1] || (errs[0] == nil) != (errs[1] == nil) {
			t.Fatalf("op %d %d %d %d alias %d: fast %q %v full %q %v", op, x, y, z, alias, got[0], errs[0], got[1], errs[1])
		}
	}
}
