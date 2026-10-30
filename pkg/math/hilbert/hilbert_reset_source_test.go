package hilbert

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestSourceResetKeepsStatisticsUntilExplicitReset(t *testing.T) {
	b := New(nil)
	if err := b.AddGE(rationals(1), rational(0)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Saturate(); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	if b.BasisSize() != 0 || len(b.inequalities) != 0 || b.IsInt(0) {
		t.Fatal("reset left problem state")
	}
	st := statistics.New()
	b.CollectStatistics(st)
	if sourceStatistic(st, "hb.num_saturations") != 1 {
		t.Fatal("source reset unexpectedly cleared statistics")
	}
}
