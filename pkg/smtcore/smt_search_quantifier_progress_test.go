package smtcore

import "testing"

func TestSMTSearchProgressCountsConsumedLazyQuantifierInstances(t *testing.T) {
	schedule := NewSMTQIQueueSchedule(nil, 0, 2, false)
	schedule.delayedEntries = append(schedule.delayedEntries, SMTQIQueueEntry{Cost: 1})
	state := &SMTInternalizerState{
		quantifierManager: &SMTQuantifierManager{
			registry: &SMTQuantifierRegistry{schedule: schedule},
		},
	}
	before := state.searchProgress()
	done, ready, err := schedule.FinalCheck()
	if err != nil {
		t.Fatal(err)
	}
	if done || len(ready) != 1 {
		t.Fatal("delayed quantifier instance was not consumed", done, len(ready))
	}
	after := state.searchProgress()
	if after == before || after.quantifierLazyInstances != before.quantifierLazyInstances+1 {
		t.Fatal("lazy quantifier queue transition was invisible to search", before, after)
	}
	drained := state.searchProgress()
	done, ready, err = schedule.FinalCheck()
	if err != nil {
		t.Fatal(err)
	}
	if !done || len(ready) != 0 || state.searchProgress() != drained {
		t.Fatal("drained quantifier queue reported spurious progress", done, len(ready))
	}
}
