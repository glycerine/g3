package subpaving

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/memory"
)

// AUDIT.md math_subpaving_02-F3: the MPFX context's checkpoint tests
// max_memory against the bound process counter, as context_t::checkpoint.
func TestFixedCheckpointMaxMemory(t *testing.T) {
	m, c, v, root := fixedClauseFixture(t, []bool{false})
	b := arena.NewBudget(1 << 20)
	p := memory.NewProcessAccounting()
	r, err := memory.NewRuntime(memory.RuntimeConfig{Budget: b, Process: p, SizeTBits: 64})
	if err != nil {
		t.Fatal(err)
	}
	extra, err := r.Allocate(64)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := r.Deallocate(&extra); err != nil {
			t.Error(err)
		}
		if err := r.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := c.BindProcessAccounting(p); err != nil {
		t.Fatal(err)
	}
	c.maxMemory = 0
	atom := fixedClauseAtom(t, m, c, v, 0, 1, true, false)
	if err := c.AddUnitClause(atom, false); err != nil {
		t.Fatal(err)
	}
	if err := c.AssertUnitClauses(root); !errors.Is(err, ErrSubpavingMaxMemory) {
		t.Fatal("max_memory not enforced", err)
	}
}
