package fp

// Source: src/muz/fp/horn_tactic.cpp:394-410, collect_statistics,
// reset_statistics and the cleanup that accumulates m_stats.

import "github.com/glycerine/g3/pkg/util/statistics"

// recordEngineStatistics is cleanup's m_imp->collect_statistics(m_stats).
func (t *HornTactic) recordEngineStatistics(engine *PropositionalEngine) {
	if engine == nil || t.closed {
		return
	}
	_ = engine.CollectStatistics(&t.statistics)
}

func (t *HornTactic) CollectStatistics(st *statistics.Statistics) {
	if t == nil || t.closed {
		return
	}
	st.Copy(t.statistics)
}

func (t *HornTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.statistics.Reset()
	return nil
}
