package numeral

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestIntegerDigitSetterSourceAssertionBeforeOldCellRelease(t *testing.T) {
	if !modularSourceDebug {
		t.Skip("source Z3DEBUG assertions only")
	}
	words := []uint32{1, 2, 3, 4, 5, 6, 7}
	t.Run("closed-true-condition", func(t *testing.T) {
		d, _ := bindMPNDebugActions(t, "")
		m, _ := integerManager(t)
		var a Int
		serviceMust(t, m.SetInt64(&a, -(1<<40)))
		old := a.Cell
		serviceMust(t, d.Close())
		err := m.SetLimbs(&a, words)
		if !errors.Is(err, arena.ErrClosed) || a.Cell != old || a.Val != 1 || m.Cell(old).Size != 2 {
			t.Fatal("source assertion must precede old-cell release", err, a)
		}
	})
	t.Run("false-condition-actions", func(t *testing.T) {
		d, log := bindMPNDebugActions(t, "TC")
		m, _ := integerManager(t)
		var a Int
		serviceMust(t, m.SetInt64(&a, -(1<<40)))
		old := a.Cell
		cell := m.Cell(old)
		cell.Capacity = 5 // actual six-word backing remains available
		m.Store(old, cell)
		short := words[:6]
		if err := m.SetLimbs(&a, short); !isMPNVerificationThrow(err) || a.Cell != old || a.Val != 1 || m.Cell(old).Size != 2 {
			t.Fatal("Throw must retain the old cell after sign publication", err, a)
		}
		if err := m.SetLimbs(&a, short); err != nil || a.Cell == old || m.Cell(a.Cell).Capacity != 6 {
			t.Fatal("Continue must release and replace the old cell", err, a)
		}
		contents, err := os.ReadFile(log.Name())
		serviceMust(t, err)
		if count := strings.Count(string(contents), "File: src/util/mpz.cpp\nLine: 326\nsz > m_init_cell_capacity\n"); count != 2 {
			t.Fatal("source assertion location and action count", count, string(contents))
		}
		serviceMust(t, d.EnableAssertions(false))
		replacement := m.Cell(a.Cell)
		replacement.Capacity = 5
		m.Store(a.Cell, replacement)
		if err := m.SetLimbs(&a, short); err != nil || m.Cell(a.Cell).Capacity != 6 {
			t.Fatal("disabled source assertion must reach replacement", err, a)
		}
	})
}
