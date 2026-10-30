package fp_test

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
)

func TestHornTacticProgressCallbackRunsAfterApply(t *testing.T) {
	f := newHornLifecycleFixture(t, false)
	f.populate(t)
	var progress []uint32
	fpMust(t, f.tactic.SetProgressCallback(func(value uint32) { progress = append(progress, value) }))
	result := &tactic.TacticResult{}
	t.Cleanup(func() { fpMust(t, result.Close()) })
	fpMust(t, f.tactic.Apply(f.goal, result))
	if len(progress) != 1 || progress[0] != 1 {
		t.Fatal("progress", progress)
	}
}
