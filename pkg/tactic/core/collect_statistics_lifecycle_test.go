package core

import "testing"

func TestCollectStatisticsAccumulatesAcrossApplyAndReset(t *testing.T) {
	f := newCollectStatisticsFixture(t)
	r := f.runner(t)
	term := f.app(t, f.f, f.a)
	applyStatistics(t, r, f.goal(t, term))
	simplifyMust(t, r.ResetStatistics())
	applyStatistics(t, r, f.goal(t, term))
	s, e := r.Snapshot()
	simplifyMust(t, e)
	if s["function-applications"] != 8 || s["uninterpreted-function-occurrences"] != 2 {
		t.Fatal(s)
	}
}

func TestCollectStatisticsTranslationHasFreshCountersAndSameOutput(t *testing.T) {
	f := newCollectStatisticsFixture(t)
	r := f.runner(t)
	applyStatistics(t, r, f.goal(t, f.a))
	translated, e := r.Translate(f.m)
	simplifyMust(t, e)
	copy := translated.(*CollectStatisticsTactic)
	t.Cleanup(func() { simplifyMust(t, copy.Close()) })
	s, e := copy.Snapshot()
	simplifyMust(t, e)
	if len(s) != 0 {
		t.Fatal("translated source owner copied counters", s)
	}
	if copy.output != r.output {
		t.Fatal("translation did not preserve selected stream")
	}
}
