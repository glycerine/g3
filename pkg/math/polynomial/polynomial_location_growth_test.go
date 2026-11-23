package polynomial

import (
	"math"
	"strconv"
	"testing"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func TestPolynomialLocationReuse(t *testing.T) {
	m := polynomialCellFixture(t)
	makePower := func(power uint32) Polynomial {
		p, err := m.MkVariable(0, power)
		serviceMust(t, err)
		serviceMust(t, m.IncRef(p))
		return p
	}
	var polys [8]Polynomial
	var monomials [8]Monomial
	for i := range polys {
		polys[i] = makePower(uint32(i + 1))
		mono, err := m.monomialFieldCore(polys[i], 0)
		serviceMust(t, err)
		monomials[i] = mono
	}
	for _, i := range []int{1, 6} {
		serviceMust(t, m.DecRef(polys[i]))
	}
	for _, i := range []int{1, 6} {
		old := polys[i]
		polys[i] = makePower(uint32(i + 10))
		mono, err := m.monomialFieldCore(polys[i], 0)
		serviceMust(t, err)
		if polys[i].slot != old.slot || polys[i].generation != old.generation+1 ||
			mono.slot != monomials[i].slot || mono.generation != monomials[i].generation+1 {
			t.Fatalf("replaced polynomial %v with %v and monomial %v with %v", old, polys[i], monomials[i], mono)
		}
		if _, err := m.polynomialLocation(old); err == nil {
			t.Fatal("retired polynomial handle still valid", old)
		}
	}
	for _, p := range polys {
		serviceMust(t, m.DecRef(p))
	}
}

func TestUnivariateDescriptorAndBackingReuse(t *testing.T) {
	p, _, _ := upolySourceOwner(t)
	var polys [70]UPolynomial
	var backings [70]upolyVectorState
	for i := range polys {
		v, err := p.NewPolynomial()
		serviceMust(t, err)
		serviceMust(t, p.SetInts(v, []unumeral.Int{unumeral.NewInt(1)}))
		polys[i] = v
		d, err := p.descriptor(v)
		serviceMust(t, err)
		backings[i] = d.state
	}
	for _, i := range []int{3, 68} {
		v := polys[i]
		serviceMust(t, p.DelPolynomial(&v))
	}
	for _, i := range []int{3, 68} {
		v, err := p.NewPolynomial()
		serviceMust(t, err)
		serviceMust(t, p.SetInts(v, []unumeral.Int{unumeral.NewInt(2)}))
		d, err := p.descriptor(v)
		serviceMust(t, err)
		if v.slot != polys[i].slot || v.generation != polys[i].generation+1 ||
			d.state.backing != backings[i].backing || d.state.generation != backings[i].generation+1 {
			t.Fatalf("slot %d: old %v/%v, new %v/%v", i, polys[i], backings[i], v, d.state)
		}
		polys[i] = v
	}
	for i := range polys {
		serviceMust(t, p.DelPolynomial(&polys[i]))
	}
}

func TestAlgebraicLocationReuse(t *testing.T) {
	m, _ := algebraicFixture(t)
	var values [8]Algebraic
	for i := range values {
		n := unumeral.NewInt(int32(i + 10))
		serviceMust(t, m.SetInteger(&values[i], &n))
	}
	for _, i := range []int{2, 6} {
		v := values[i]
		serviceMust(t, m.Del(&v))
	}
	for _, i := range []int{2, 6} {
		old := values[i]
		var v Algebraic
		n := unumeral.NewInt(int32(i + 20))
		serviceMust(t, m.SetInteger(&v, &n))
		if old.slot == 0 || v.slot != old.slot || v.generation != old.generation+1 {
			t.Fatalf("old %v, new %v", old, v)
		}
		values[i] = v
	}
	for i := range values {
		serviceMust(t, m.Del(&values[i]))
	}
}

func TestPolynomialLocationExhaustedGeneration(t *testing.T) {
	m := MultivariatePolynomialCore{locations: []polynomialLocation{
		{generation: math.MaxUint32}, {live: true}, {generation: math.MaxUint32 - 1},
	}}
	for _, want := range []uint32{3, 4} {
		slot, err := m.prepareLocation()
		serviceMust(t, err)
		if slot != want {
			t.Fatalf("got slot %d, want %d", slot, want)
		}
	}
}

// Preallocate the directory so that the measured work is finding and publishing
// each slot. The shell benchmark in perf/nlsat also includes backing growth,
// arithmetic, reference management and solver teardown.
func BenchmarkPolynomialLocationGrowth(b *testing.B) {
	for _, width := range []int{128, 1024, 8192} {
		b.Run("polynomial/"+strconv.Itoa(width), func(b *testing.B) {
			rows := make([]polynomialLocation, 0, width)
			for b.Loop() {
				m := MultivariatePolynomialCore{locations: rows}
				for i := 0; i < width; i++ {
					slot, err := m.prepareLocation()
					if err != nil || slot != uint32(i+1) {
						b.Fatalf("slot %d at %d: %v", slot, i, err)
					}
					m.locations[slot-1].live = true
				}
			}
		})
		b.Run("monomial/"+strconv.Itoa(width), func(b *testing.B) {
			rows := make([]monomialLocation, 0, width)
			for b.Loop() {
				m := MonomialManager{locations: rows}
				for i := 0; i < width; i++ {
					slot, err := m.prepareLocation()
					if err != nil || slot != uint32(i+1) {
						b.Fatalf("slot %d at %d: %v", slot, i, err)
					}
					m.locations[slot-1].live = true
				}
			}
		})
	}
}

func TestUnivariateReusesBackingReleasedByBorrower(t *testing.T) {
	for _, action := range []string{"delete", "grow"} {
		t.Run(action, func(t *testing.T) {
			owner, q, _, limit, coop := upolySourceOwnerServices(t)
			borrower := upolyAdoptionOther(t, q, limit, coop)
			src := upolySourceNew(t, owner, []int32{7})
			d, err := owner.descriptor(src)
			serviceMust(t, err)
			old := d.state
			for i := 0; i < 5; i++ {
				upolySourceNew(t, owner, []int32{1})
			}
			dst, err := borrower.NewPolynomial()
			serviceMust(t, err)
			t.Cleanup(func() {
				if dst != (UPolynomial{}) {
					serviceMust(t, borrower.DelPolynomial(&dst))
				}
			})
			serviceMust(t, borrower.SwapFrom(dst, owner, src))
			if action == "delete" {
				serviceMust(t, borrower.DelPolynomial(&dst))
			} else {
				values := make([]unumeral.Int, 64)
				values[len(values)-1] = unumeral.NewInt(1)
				serviceMust(t, borrower.SetInts(dst, values))
			}
			serviceMust(t, owner.SetInts(src, []unumeral.Int{unumeral.NewInt(9)}))
			if d.state.backing != old.backing || d.state.generation != old.generation+1 {
				t.Fatalf("old physical backing %v, new %v", old, d.state)
			}
		})
	}
}
