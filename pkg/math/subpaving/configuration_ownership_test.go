package subpaving

// Source: src/math/subpaving/subpaving_hwf.h, subpaving_mpf.h,
// subpaving_mpff.h and subpaving_mpfx.h: borrowed config manager references.

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestSubpavingConfigurationBorrowedManagers(t *testing.T) {
	t.Run("hwf", func(t *testing.T) {
		m, err := numeral.NewHardwareFloatNumeralManager(numeral.NewHardwareFloatManager(), 11, 53)
		if err != nil {
			t.Fatal(err)
		}
		limit := &control.Reslimit{Limit: 100}
		first, err := NewHardwareContext(m, limit, 0, 2, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := first.Close(); err != nil {
			t.Fatal(err)
		}
		second, err := NewHardwareContext(m, limit, 0, 2, 1)
		if err != nil {
			t.Fatal("context close invalidated borrowed configuration", err)
		}
		t.Cleanup(func() {
			if err := second.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := second.NewVariable(true); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("mpf", func(t *testing.T) {
		f := newArbitraryClauseFixture(t)
		if err := f.context.Close(); err != nil {
			t.Fatal(err)
		}
		second, err := NewArbitraryContext(f.numbers, &control.Reslimit{Limit: 100}, 0, 2, 1)
		if err != nil {
			t.Fatal("context close invalidated borrowed configuration", err)
		}
		t.Cleanup(func() {
			if err := second.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := second.NewVariable(true); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("mpff", func(t *testing.T) {
		m, first, _, _ := fastMonomialFixture(t, nil)
		if err := first.Close(); err != nil {
			t.Fatal(err)
		}
		second, err := NewFastContext(m, &control.Reslimit{Limit: 100}, nil, 0, 2, 1)
		if err != nil {
			t.Fatal("context close invalidated borrowed configuration", err)
		}
		t.Cleanup(func() {
			if err := second.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := second.NewVariable(true); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("mpfx", func(t *testing.T) {
		m, first, _, _ := fixedClauseFixture(t, nil)
		if err := first.Close(); err != nil {
			t.Fatal(err)
		}
		second, err := NewFixedContext(m, &control.Reslimit{Limit: 100}, nil, 0, 2, 1)
		if err != nil {
			t.Fatal("context close invalidated borrowed configuration", err)
		}
		t.Cleanup(func() {
			if err := second.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := second.NewVariable(true); err != nil {
			t.Fatal(err)
		}
	})
}
