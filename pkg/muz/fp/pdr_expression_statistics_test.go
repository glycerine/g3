package fp

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestPDRExpressionStatisticsSurviveCloneDisposal(t *testing.T) {
	f := newPDRLifecycleFixture(t)
	e := f.engine
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	checks := func(engine *PropositionalEngine) uint32 {
		t.Helper()
		stats := statistics.New()
		must(engine.CollectStatistics(stats))
		var count uint32
		for index := 0; index < stats.Size(); index++ {
			if stats.IsUint(index) && stats.Key(index) == "num checks" {
				count += stats.UintValue(index)
			}
		}
		return count
	}
	var total uint32
	for round := 0; round < 3; round++ {
		prior := e.queryClone
		status, err := e.QueryExpression(f.applications["q"])
		must(err)
		if status != lbool.True {
			t.Fatal("query result", round, status)
		}
		if prior != nil {
			if _, err := prior.LastResult(); err == nil {
				t.Fatal("previous query clone remained open")
			}
		}
		current := checks(e.queryClone)
		if current == 0 {
			t.Fatal("query clone reported no solver checks")
		}
		total += current
		if got := checks(e); got != total {
			t.Fatal("parent lost or duplicated query statistics", round, got, total)
		}
		if got := checks(e); got != total {
			t.Fatal("repeated collection mutated query statistics", got, total)
		}
	}
	prior := e.queryClone
	must(e.SetQueryParameters(nil))
	if e.queryClone != nil {
		t.Fatal("parameter update left its query clone active")
	}
	if _, err := prior.LastResult(); err == nil {
		t.Fatal("parameter update left the old clone open")
	}
	if got := checks(e); got != total {
		t.Fatal("clone disposal lost or duplicated cumulative checks", got, total)
	}
	must(e.SetQueryParameters(nil))
	if got := checks(e); got != total {
		t.Fatal("repeated cleanup duplicated archived statistics", got, total)
	}
}
