package spacer

// Source: src/muz/spacer/spacer_context.cpp solve_core, expand_node,
// collect_statistics and reset_statistics.

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func spacerContextStatistic(t *testing.T, c *Context, key string) uint32 {
	t.Helper()
	collected := statistics.New()
	depthMust(t, c.CollectStatistics(collected))
	var result uint32
	for i := 0; i < collected.Size(); i++ {
		if collected.IsUint(i) && string(collected.Key(i)) == key {
			result += collected.UintValue(i)
		}
	}
	return result
}

func TestSpacerContextCountsGeneratedLemmasAcrossRuleUpdates(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	c := f.context
	truth, err := c.manager.MkTrue()
	depthMust(t, err)
	_, err = c.transformers[f.query].AddLemma(truth, 4)
	depthMust(t, err)
	status, err := c.Solve(0, 1)
	depthMust(t, err)
	if status != lbool.Undef {
		t.Fatal("expected bounded recursive search", status)
	}
	for key, expected := range map[string]uint32{"SPACER num lemmas": 1, "SPACER max depth": 1} {
		if got := spacerContextStatistic(t, c, key); got != expected {
			t.Fatalf("%s = %d, want %d", key, got, expected)
		}
	}
	count, err := c.lemmaAdditions()
	depthMust(t, err)
	if count != 1 {
		t.Fatal("restart schedule counted externally supplied lemmas", count)
	}
	depthMust(t, c.UpdateRules(f.rules))
	if got := spacerContextStatistic(t, c, "SPACER num lemmas"); got != 1 {
		t.Fatal("rule replacement lost context count", got)
	}
	depthMust(t, c.ResetStatistics())
	for _, key := range []string{"SPACER num lemmas", "SPACER max depth", "SPACER restarts"} {
		if got := spacerContextStatistic(t, c, key); got != 0 {
			t.Fatal("statistics reset left context counter", key, got)
		}
	}
	count, err = c.lemmaAdditions()
	depthMust(t, err)
	if count != 0 {
		t.Fatal("statistics reset did not reset restart lemma count", count)
	}
}

func TestSpacerContextRecordsAttemptedQueryLevel(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c := f.context
	status, err := c.Solve(4, 1)
	depthMust(t, err)
	if status != lbool.True {
		t.Fatal("expected initial fact", status)
	}
	if got := spacerContextStatistic(t, c, "SPACER max query lvl"); got != 4 {
		t.Fatal("attempted query level", got)
	}
	if got := spacerContextStatistic(t, c, "SPACER max depth"); got != 0 {
		t.Fatal("successful attempt advanced depth", got)
	}
	depthMust(t, c.Reset())
	if got := spacerContextStatistic(t, c, "SPACER max query lvl"); got != 4 {
		t.Fatal("reset lost last attempted query level", got)
	}
	depthMust(t, c.ResetStatistics())
	if got := spacerContextStatistic(t, c, "SPACER max query lvl"); got != 0 {
		t.Fatal("statistics reset left last query level", got)
	}
}
