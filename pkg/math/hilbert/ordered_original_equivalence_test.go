package hilbert

import (
	"sort"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func saturateOriginalForTest(b *Basis) (lbool.LBool, error) {
	b.initBasis()
	b.current = 0
	for b.current < len(b.inequalities) {
		if err := b.selectInequality(); err != nil {
			return lbool.Undef, err
		}
		result, err := b.saturateOne(b.inequalities[b.current].values, b.inequalities[b.current].equal)
		if err != nil || result != lbool.True {
			return result, err
		}
		b.current++
	}
	return lbool.True, nil
}

func basisKeys(t *testing.T, b *Basis) []string {
	t.Helper()
	keys := make([]string, 0, b.BasisSize())
	for index := uint32(0); index < b.BasisSize(); index++ {
		values, initial, err := b.BasisSolution(index)
		if err != nil {
			t.Fatal(err)
		}
		parts := []string{"homogeneous"}
		if initial {
			parts[0] = "initial"
		}
		for i := range values {
			parts = append(parts, values[i].String())
		}
		keys = append(keys, strings.Join(parts, ":"))
	}
	sort.Strings(keys)
	return keys
}

func TestOrderedSaturationMatchesOriginalWorkList(t *testing.T) {
	build := func() *Basis {
		b := New(nil)
		b.SetIsInt(0)
		if err := b.AddGE(rationals(1, -1), rational(0)); err != nil {
			t.Fatal(err)
		}
		if err := b.AddEQ(rationals(1, 1), rational(2)); err != nil {
			t.Fatal(err)
		}
		return b
	}
	ordered, original := build(), build()
	left, err := ordered.Saturate()
	if err != nil {
		t.Fatal(err)
	}
	right, err := saturateOriginalForTest(original)
	if err != nil {
		t.Fatal(err)
	}
	if left != right {
		t.Fatalf("results = %v and %v", left, right)
	}
	leftKeys, rightKeys := basisKeys(t, ordered), basisKeys(t, original)
	if strings.Join(leftKeys, "|") != strings.Join(rightKeys, "|") {
		t.Fatalf("ordered basis %v, original basis %v", leftKeys, rightKeys)
	}
}
