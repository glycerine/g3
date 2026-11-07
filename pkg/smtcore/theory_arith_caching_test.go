package smtcore

import (
	"fmt"
	"math/big"
	"math/rand"
	"sort"
	"testing"
)

func smtCachingAtom(r *rand.Rand) *smtLinearArithmeticAtom {
	form := newSMTLinearForm()
	for i := 0; i < 1+r.Intn(3); i++ {
		form.terms[uint32(1+r.Intn(5))] = big.NewRat(int64(1+r.Intn(5)), int64(1+r.Intn(3)))
	}
	form.constant.SetFrac64(int64(r.Intn(21)-10), int64(1+r.Intn(4)))
	ops := []int32{smtArithOpLE, smtArithOpGE, smtArithOpLT, smtArithOpGT}
	return &smtLinearArithmeticAtom{form: form, op: ops[r.Intn(len(ops))], supported: true}
}

// A memoized bound must equal a fresh computation for every polarity and mode,
// and must survive callers working on their own clones of it.
func TestSharedAtomBoundMatchesFreshComputation(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for round := 0; round < 300; round++ {
		atom := smtCachingAtom(r)
		for _, truth := range []bool{true, false} {
			for _, integer := range []bool{true, false} {
				for repeat := 0; repeat < 2; repeat++ {
					wantLower, want := smtLinearAtomBound(atom, truth, integer)
					gotLower, got := smtLinearAtomBoundShared(atom, truth, integer)
					if wantLower != gotLower || want.Cmp(&got) != 0 {
						t.Fatalf("truth %v integer %v: %v %v, want %v %v", truth, integer, gotLower, got, wantLower, want)
					}
					clone := got.Clone()
					clone.Quo(big.NewRat(7, 3))
					clone.Neg()
				}
			}
		}
	}
}

func TestSharedAtomBoundEntriesAreIndependent(t *testing.T) {
	seen := map[int]bool{}
	for _, truth := range []bool{true, false} {
		for _, integer := range []bool{true, false} {
			seen[smtAtomBoundIndex(truth, integer)] = true
		}
	}
	if len(seen) != 4 {
		t.Fatal("polarity and mode share a cache entry", seen)
	}
}

func TestCachedCoefficientKeyMatchesComputedKey(t *testing.T) {
	r := rand.New(rand.NewSource(5))
	for round := 0; round < 200; round++ {
		atom := smtCachingAtom(r)
		want := smtArithmeticCoefficientKey(&atom.form)
		if got := atom.cachedCoefficientKey(); got != want || atom.cachedCoefficientKey() != want {
			t.Fatal(got, want)
		}
		if !atom.coefficientKeySet {
			t.Fatal("key not cached")
		}
	}
}

// smtScanAssertionsForColumn is the full scan the index replaced.
func smtScanAssertionsForColumn(th *SMTLinearArithmeticTheory, column uint32, bases []uint32) []int {
	var result []int
	for index, assertion := range th.assertions {
		atom := &th.atoms[assertion.atom]
		if !atom.supported {
			continue
		}
		matched := bases[assertion.atom] == column
		if len(atom.form.terms) == 1 {
			for variable := range atom.form.terms {
				matched = matched || variable == column
			}
		}
		if matched {
			result = append(result, index)
		}
	}
	return result
}

// The column index must match a full scan, in assertion order, through growth,
// pops, and regrowth of the assertion stack to the same length.
func TestAssertionColumnIndexMatchesFullScan(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for seed := 0; seed < 20; seed++ {
		th := &SMTLinearArithmeticTheory{}
		for i := 0; i < 12; i++ {
			atom := smtCachingAtom(r)
			atom.supported = r.Intn(5) != 0
			th.atoms = append(th.atoms, *atom)
		}
		bases := make([]uint32, len(th.atoms))
		for i := range bases {
			bases[i] = uint32(100 + r.Intn(4))
		}
		check := func(step string) {
			t.Helper()
			index := th.arithmeticAssertionsByColumn(bases)
			for column := uint32(0); column < 110; column++ {
				got, want := index[column], smtScanAssertionsForColumn(th, column, bases)
				if fmt.Sprint(got) != fmt.Sprint(want) && len(got)+len(want) != 0 {
					t.Fatalf("%s column %d: index %v, scan %v", step, column, got, want)
				}
				if !sort.IntsAreSorted(got) {
					t.Fatalf("%s column %d: not in assertion order: %v", step, column, got)
				}
			}
		}
		for step := 0; step < 60; step++ {
			if r.Intn(4) == 0 && len(th.assertions) != 0 {
				th.assertions = th.assertions[:r.Intn(len(th.assertions))]
				th.assertionPopEpoch++
			} else {
				th.assertions = append(th.assertions, smtIntegerDiffAssertion{atom: r.Intn(len(th.atoms)), truth: r.Intn(2) == 0})
			}
			check(fmt.Sprintf("seed %d step %d", seed, step))
		}
		// New bases for the same atoms invalidate the index.
		fresh := make([]uint32, len(bases))
		for i := range fresh {
			fresh[i] = uint32(104 + r.Intn(4))
		}
		bases = fresh
		check("rebased")
	}
}
