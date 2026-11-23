package tactic

import (
	"math/rand"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// Anchoring each square gives exactly two admissible values per variable.
// Enumerate their signs and check the original integer product directly.
func TestProductEquationsAgainstFiniteAssignments(t *testing.T) {
	f := newGoalCompilerFixture(t)
	p := integerCandidate{manager: f.manager}
	rng := rand.New(rand.NewSource(183419))
	for trial := range 2000 {
		n := 2 + rng.Intn(6)
		s := productSystem{candidate: &p, edges: make([][]productEquation, n), signs: make([]int8, n)}
		magnitudes := make([]int32, n)
		for i := range n {
			magnitudes[i] = 1 + int32(rng.Intn(9))
			s.edges[i] = append(s.edges[i], productEquation{int32(i), magnitudes[i] * magnitudes[i]})
			s.signs[i] = int8(rng.Intn(3)) - 1
		}
		for j := rng.Intn(10); j > 0; j-- {
			a, b := rng.Intn(n), rng.Intn(n)
			k := magnitudes[a] * magnitudes[b]
			if rng.Intn(2) == 0 {
				k = -k
			}
			if rng.Intn(9) == 0 {
				k *= 2
			}
			s.edges[a] = append(s.edges[a], productEquation{int32(b), k})
			s.edges[b] = append(s.edges[b], productEquation{int32(a), k})
		}
		satisfies := func(mask int) bool {
			value := func(i int) int32 {
				if mask&(1<<i) != 0 {
					return -magnitudes[i]
				}
				return magnitudes[i]
			}
			for i, edges := range s.edges {
				x := value(i)
				if x*int32(s.signs[i]) < 0 {
					return false
				}
				for _, edge := range edges {
					if x*value(int(edge.other)) != edge.value {
						return false
					}
				}
			}
			return true
		}
		exists := false
		for mask := range 1 << n {
			if satisfies(mask) {
				exists = true
				break
			}
		}
		roots := make([]rationalRadical, n)
		status := s.solve(roots)
		if status == lbool.Undef || exists != (status == lbool.True) {
			t.Fatalf("trial %d has solution %v, status %v", trial, exists, status)
		}
		if exists {
			mask := 0
			for i, v := range roots {
				if v.numerator != magnitudes[i]*magnitudes[i] || v.denominator != 1 {
					t.Fatalf("trial %d returned radicand %+v at %d", trial, v, i)
				}
				if v.negative {
					mask |= 1 << i
				}
			}
			if !satisfies(mask) {
				t.Fatalf("trial %d returned invalid signs %b", trial, mask)
			}
		}
	}
}

func TestOddProductCycleFixesFractionalSquares(t *testing.T) {
	f := newGoalCompilerFixture(t)
	p := integerCandidate{manager: f.manager}
	// xy=-2, yz=3, zx=-5, y>0.
	s := productSystem{candidate: &p, edges: [][]productEquation{
		{{1, -2}, {2, -5}}, {{0, -2}, {2, 3}}, {{1, 3}, {0, -5}},
	}, signs: []int8{0, 1, 0}}
	want := [...]rationalRadical{{10, 3, true}, {6, 5, false}, {15, 2, false}}
	var got [3]rationalRadical
	if status := s.solve(got[:]); status != lbool.True || got != want {
		t.Fatalf("status %v, solution %+v", status, got)
	}
	f.manager.Limit.Cancel()
	defer f.manager.Limit.DecCancel()
	if s.solve(got[:]) != lbool.Undef {
		t.Fatal("canceled product propagation returned a decision")
	}
}

// The initial triangle fixes every absolute value without an explicit square.
// Express each magnitude as a rational multiple of sqrt(p*q). This lets the
// oracle verify every edge and returned root using only small integer products.
func TestProductEquationsWithFractionalMagnitudes(t *testing.T) {
	f := newGoalCompilerFixture(t)
	candidate := integerCandidate{manager: f.manager}
	rng := rand.New(rand.NewSource(300119))
	for trial := range 1200 {
		primes := [...]int32{2, 3, 5, 7}
		i := rng.Intn(len(primes))
		p, q := primes[i], primes[(i+1+rng.Intn(len(primes)-1))%len(primes)]
		n := 3 + rng.Intn(5)
		s := productSystem{candidate: &candidate, edges: make([][]productEquation, n), signs: make([]int8, n)}
		var coefficient, denominator [7]int32
		for i := range n {
			coefficient[i], denominator[i] = 1+int32(rng.Intn(5)), 1
			if rng.Intn(2) == 0 {
				coefficient[i] = -coefficient[i]
			}
			s.signs[i] = int8(rng.Intn(3)) - 1
		}
		denominator[0], denominator[1] = q, p
		add := func(i, j int, perturb int32) {
			num, den := coefficient[i]*coefficient[j]*p*q, denominator[i]*denominator[j]
			if num%den == 0 && num/den+perturb != 0 {
				value := num/den + perturb
				s.edges[i] = append(s.edges[i], productEquation{int32(j), value})
				s.edges[j] = append(s.edges[j], productEquation{int32(i), value})
			}
		}
		add(0, 1, 0)
		add(1, 2, 0)
		add(2, 0, 0)
		for i := 3; i < n; i++ {
			add(2, i, 0)
		}
		for range rng.Intn(10) {
			var perturb int32
			if rng.Intn(4) == 0 {
				perturb = 1
			}
			add(rng.Intn(n), rng.Intn(n), perturb)
		}
		// Connectivity and the nonzero triangle leave only two global signs.
		accepts := func(sign int32) bool {
			for i, edges := range s.edges {
				if sign*coefficient[i]*int32(s.signs[i]) < 0 {
					return false
				}
				for _, edge := range edges {
					j := edge.other
					if coefficient[i]*coefficient[j]*p*q != edge.value*denominator[i]*denominator[j] {
						return false
					}
				}
			}
			return true
		}
		roots := make([]rationalRadical, n)
		status := s.solve(roots)
		if status == lbool.Undef || (status == lbool.True) != (accepts(1) || accepts(-1)) {
			t.Fatalf("trial %d returned %v", trial, status)
		}
		if status == lbool.True {
			sign := int32(1)
			if roots[0].negative != (coefficient[0] < 0) {
				sign = -1
			}
			if !accepts(sign) {
				t.Fatalf("trial %d chose an inconsistent sign", trial)
			}
			for i, root := range roots {
				if root.numerator*denominator[i]*denominator[i] != root.denominator*coefficient[i]*coefficient[i]*p*q ||
					root.negative != (sign*coefficient[i] < 0) {
					t.Fatalf("trial %d has invalid root %d: %+v", trial, i, root)
				}
			}
		}
	}
}
