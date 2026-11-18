package shell

// Source: src/shell/dimacs_frontend.cpp:27-37.
import (
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/statistics"
	"time"
)

func (f *DIMACSFrontend) writeStatistics(s *sat.Solver) error {
	st := statistics.New()
	if err := s.CollectStatistics(st); err != nil {
		return err
	}
	// st.update("total time", seconds): statistics::update drops only an
	// exact 0.0, and display_smt2 does the two-decimal formatting.
	st.UpdateDouble("total time", time.Since(f.start).Seconds())
	return st.DisplaySMT2(f.output)
}
