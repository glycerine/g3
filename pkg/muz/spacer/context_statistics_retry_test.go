package spacer

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

func TestSpacerStatisticsCollectionRetryDoesNotDuplicatePartialCounters(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c := f.context
	p := c.transformers[f.query]
	c.stats.queries = 7
	p.propagations = 3
	failure := errors.New("transformer clock unavailable")
	fail := false
	p.initializeWatch = stopwatch.New(func() (uint64, error) {
		if fail {
			return 0, failure
		}
		return 1e9, nil
	})
	depthMust(t, p.initializeWatch.Start())
	destination := statistics.New()
	destination.UpdateUint("caller counter", 11)
	before := *destination
	fail = true
	for i := 0; i < 2; i++ {
		if err := c.CollectStatistics(destination); !errors.Is(err, failure) {
			t.Fatal("missing late collection failure", err)
		}
		if !reflect.DeepEqual(*destination, before) {
			t.Fatal("failed collection published partial counters")
		}
	}
	fail = false
	depthMust(t, c.CollectStatistics(destination))
	expected := statistics.New()
	expected.UpdateUint("caller counter", 11)
	depthMust(t, c.CollectStatistics(expected))
	if !reflect.DeepEqual(*destination, *expected) {
		t.Fatal("retry duplicated contributions", destination, expected)
	}

	totals := map[statistics.Key]uint32{}
	for i := 0; i < destination.Size(); i++ {
		if destination.IsUint(i) {
			totals[destination.Key(i)] += destination.UintValue(i)
		}
	}
	if totals["caller counter"] != 11 || totals["SPACER num queries"] != 7 || totals["SPACER num propagations"] != 3 {
		t.Fatal("retry changed source counters", totals)
	}
	depthMust(t, p.initializeWatch.Stop())
}
