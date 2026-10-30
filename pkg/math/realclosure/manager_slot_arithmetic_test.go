package realclosure

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestManagerNumeralSlotArithmeticAliasing(t *testing.T) {
	for _, usePrem := range []bool{false, true} {
		t.Run(map[bool]string{false: "remainder", true: "pseudo_remainder"}[usePrem], func(t *testing.T) {
			f := newRankedValueFixture(t, usePrem)
			store := f.base.store
			slot := f.gamma
			if err := store.retain(slot); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := store.resetNumeralSlot(&slot); err != nil {
					t.Fatal(err)
				}
			})

			if err := store.addNumeralSlots(&slot, slot, f.gamma, usePrem); err != nil {
				t.Fatal("aliased add", err)
			}
			twoGamma, err := store.multiplyRankedRetainedValues(f.two, f.gamma, usePrem)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = store.releaseSturmScratch(twoGamma) }()
			if equal, err := store.equalAnyDeterminedValues(slot, twoGamma, usePrem); err != nil || !equal {
				t.Fatal("add result", equal, err)
			}

			if err := store.subtractNumeralSlots(&slot, f.gamma, f.gamma, usePrem); err != nil || slot != nil {
				t.Fatal("zero subtraction", slot, err)
			}
			if err := store.multiplyNumeralSlots(&slot, f.gamma, f.gamma, usePrem); err != nil {
				t.Fatal("multiply", err)
			}
			if equal, err := store.equalAnyDeterminedValues(slot, f.alpha, usePrem); err != nil || !equal {
				t.Fatal("gamma squared", equal, err)
			}
			if err := store.negateNumeralSlot(&slot, slot, usePrem); err != nil {
				t.Fatal("aliased negate", err)
			}
			if sign, err := store.determineRankedRetainedValueSign(slot, usePrem); err != nil || sign != -1 {
				t.Fatal("negative alpha", sign, err)
			}
			if err := store.powerNumeralSlot(&slot, f.gamma, 4, usePrem); err != nil {
				t.Fatal("power", err)
			}
			if equal, err := store.equalAnyDeterminedValues(slot, f.two, usePrem); err != nil || !equal {
				t.Fatal("gamma fourth power", equal, err)
			}
			if err := store.inverseNumeralSlot(&slot, f.gamma, usePrem); err != nil {
				t.Fatal("inverse", err)
			}
			if err := store.multiplyNumeralSlots(&slot, slot, f.gamma, usePrem); err != nil {
				t.Fatal("inverse product", err)
			}
			if equal, err := store.equalAnyDeterminedValues(slot, f.one, usePrem); err != nil || !equal {
				t.Fatal("inverse product result", equal, err)
			}
			if err := store.divideNumeralSlots(&slot, f.alpha, f.gamma, usePrem); err != nil {
				t.Fatal("division", err)
			}
			if equal, err := store.equalAnyDeterminedValues(slot, f.gamma, usePrem); err != nil || !equal {
				t.Fatal("alpha/gamma", equal, err)
			}
		})
	}
}

func TestManagerNumeralSlotArithmeticFailuresPreserveDestination(t *testing.T) {
	f := newRankedValueFixture(t, true)
	foreign := newRankedValueFixture(t, true)
	store := f.base.store
	slot := f.alpha
	if err := store.retain(slot); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.resetNumeralSlot(&slot) }()
	original, refs := slot, slot.refs
	checks := []struct {
		name string
		run  func() error
	}{
		{"add foreign", func() error { return store.addNumeralSlots(&slot, f.alpha, foreign.alpha, true) }},
		{"negate foreign", func() error { return store.negateNumeralSlot(&slot, foreign.alpha, true) }},
		{"inverse zero", func() error { return store.inverseNumeralSlot(&slot, nil, true) }},
		{"divide zero", func() error { return store.divideNumeralSlots(&slot, f.alpha, nil, true) }},
		{"power foreign zero", func() error { return store.powerNumeralSlot(&slot, foreign.alpha, 0, true) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); !errors.Is(err, arena.ErrInvalidSlot) {
				t.Fatal("expected invalid slot", err)
			}
			if slot != original || slot.refs != refs || slot.deleted {
				t.Fatal("destination changed after failure", slot, refs, slot.refs)
			}
		})
	}
}

func TestManagerNumeralSlotPowerZeroAndNilArithmetic(t *testing.T) {
	f := newRankedValueFixture(t, true)
	store := f.base.store
	var slot *realValue
	if err := store.powerNumeralSlot(&slot, nil, 0, true); err != nil {
		t.Fatal(err)
	}
	if equal, err := store.equalAnyDeterminedValues(slot, f.one, true); err != nil || !equal {
		t.Fatal("zero power", equal, err)
	}
	if err := store.addNumeralSlots(&slot, nil, nil, true); err != nil || slot != nil {
		t.Fatal("zero plus zero", slot, err)
	}
}
