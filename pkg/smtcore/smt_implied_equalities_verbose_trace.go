//go:build g3trace

package smtcore

// Source: the stopwatches of get_implied_equalities_impl (m_stats_timer,
// m_stats_val_eq_timer and the static s_timer, s_stats_val_eq_timer) and
// its IF_VERBOSE(1, ...) line at the end of operator(),
// src/smt/smt_implied_equalities.cpp:362-366. The line prints wall-clock
// times, so it is built only with the g3trace tag.

import (
	"strconv"
	"time"
)

// smtImpliedEqualityStopwatch is util/stopwatch.h: start/stop accumulate.
type smtImpliedEqualityStopwatch struct {
	total   time.Duration
	started time.Time
	running bool
}

func (w *smtImpliedEqualityStopwatch) start() {
	if !w.running {
		w.started = time.Now()
		w.running = true
	}
}

func (w *smtImpliedEqualityStopwatch) stop() {
	if w.running {
		w.total += time.Since(w.started)
		w.running = false
	}
}

func (w *smtImpliedEqualityStopwatch) seconds() string {
	return strconv.FormatFloat(w.total.Seconds(), 'g', 6, 64)
}

// The static stopwatches of get_implied_equalities_impl.
var (
	smtImpliedEqualitiesSTimer           smtImpliedEqualityStopwatch
	smtImpliedEqualitiesSStatsValEqTimer smtImpliedEqualityStopwatch
)

type smtImpliedEqualityTimers struct {
	stats, valEq smtImpliedEqualityStopwatch
}

// startCall starts operator()'s local timer and s_timer.
func (t *smtImpliedEqualityTimers) startCall() *smtImpliedEqualityStopwatch {
	timer := &smtImpliedEqualityStopwatch{}
	timer.start()
	smtImpliedEqualitiesSTimer.start()
	return timer
}

func (t *smtImpliedEqualityTimers) stopCall(timer *smtImpliedEqualityStopwatch) {
	timer.stop()
	smtImpliedEqualitiesSTimer.stop()
}

func (t *smtImpliedEqualityTimers) startStaticValEq() { smtImpliedEqualitiesSStatsValEqTimer.start() }
func (t *smtImpliedEqualityTimers) stopStaticValEq()  { smtImpliedEqualitiesSStatsValEqTimer.stop() }

// verbose is IF_VERBOSE(1, verbose_stream() << s_timer << "\t" << num_terms
// << "\t" << timer << "\t" << m_stats_calls << "\t" << m_stats_timer << "\t"
// << m_stats_val_eq_timer << "\t" << s_stats_val_eq_timer << "\n").
func (t *smtImpliedEqualityTimers) verbose(numTerms uint32, timer *smtImpliedEqualityStopwatch, calls uint32) error {
	return assertedVerbose(1, smtImpliedEqualitiesSTimer.seconds()+"\t"+strconv.FormatUint(uint64(numTerms), 10)+"\t"+
		timer.seconds()+"\t"+strconv.FormatUint(uint64(calls), 10)+"\t"+
		t.stats.seconds()+"\t"+t.valEq.seconds()+"\t"+smtImpliedEqualitiesSStatsValEqTimer.seconds()+"\n")
}
