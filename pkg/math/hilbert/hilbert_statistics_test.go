package hilbert

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func sourceStatistic(st *statistics.Statistics, name string) uint32 {
	for index := 0; index < st.Size(); index++ {
		if string(st.Key(index)) == name && st.IsUint(index) {
			return st.UintValue(index)
		}
	}
	return 0
}

func TestSourceSaturationStatistics(t *testing.T) {
	b := New(nil)
	if err := b.AddEQ(rationals(1, -1), rational(0)); err != nil {
		t.Fatal(err)
	}
	if result, err := b.Saturate(); err != nil || result != lbool.True {
		t.Fatal(result, err)
	}
	st := statistics.New()
	b.CollectStatistics(st)
	if sourceStatistic(st, "hb.num_saturations") != 1 || sourceStatistic(st, "hb.index.num_insert") == 0 {
		t.Fatal("missing source Hilbert statistics")
	}
	b.ResetStatistics()
	st.Reset()
	b.CollectStatistics(st)
	if sourceStatistic(st, "hb.num_saturations") != 0 || sourceStatistic(st, "hb.basis_size") == 0 {
		t.Fatal("statistics reset removed basis size or kept counters")
	}
}
