package memory

// Source: src/test/memory.cpp, the 10/20/30 MiB limit sequence.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestMemorySourceLimitSequence(t *testing.T) {
	for _, megabytes := range []uint32{10, 20, 30} {
		budget := arena.NewBudget(1 << 27)
		owner, err := NewRuntime(RuntimeConfig{Budget: budget, SizeTBits: 64})
		memoryRuntimeRequire(t, err)
		memoryRuntimeRequire(t, owner.SetMaxSizeMegabytes(megabytes))
		limit := uint64(megabytes) << 20
		block, err := owner.Allocate(limit - 8)
		memoryRuntimeRequire(t, err)
		if state := owner.State(); state.OutOfMemory || state.GlobalSize != int64(limit) {
			t.Fatal("below source limit", megabytes, state)
		}
		_, err = owner.Allocate(0)
		memoryRuntimeCode(t, err, 101)
		if !owner.State().OutOfMemory {
			t.Fatal("missing out-of-memory state", megabytes)
		}
		memoryRuntimeRequire(t, owner.Deallocate(&block))
		memoryRuntimeRequire(t, owner.Close())
		if budget.Used() != 0 {
			t.Fatal("memory owner did not release blocks", megabytes, budget.Used())
		}
	}
}
