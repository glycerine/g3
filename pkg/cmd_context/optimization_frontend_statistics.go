package cmd_context

// Source: src/shell/opt_frontend.cpp display_statistics.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/statistics"
)

// WriteOptimizationFrontendStatistics is opt_frontend's display_statistics:
// g_opt->collect_statistics(stats); stats.display(std::cout); then the
// "time:" line. stats.display leaves std::cout in fixed notation with two
// digits, so the time prints as %.2f. AUDIT.md FUZZ_J-F6.
func (c *Context) WriteOptimizationFrontendStatistics(out io.Writer, seconds float64) error {
	if err := c.valid(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	st := statistics.New()
	if c.optimizationStatsOptimizer != nil {
		if err := c.collectOptimizationStatistics(st); err != nil {
			return err
		}
	} else {
		// collect_statistics without a run: memory and rlimit only.
		statistics.AddProcessMemory(st)
		m, err := c.Manager()
		if err != nil {
			return err
		}
		st.UpdateUint64("rlimit count", m.Limit.Count())
	}
	if err := st.Display(out); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "time:                %.2f secs\n", seconds)
	return err
}
