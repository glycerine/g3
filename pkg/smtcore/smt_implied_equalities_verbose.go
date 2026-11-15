//go:build !g3trace

package smtcore

// Untagged builds omit get_implied_equalities_impl's stopwatches and its
// IF_VERBOSE(1) timing line (src/smt/smt_implied_equalities.cpp:362-366);
// see smt_implied_equalities_verbose_trace.go.

type smtImpliedEqualityStopwatch struct{}

func (w *smtImpliedEqualityStopwatch) start() {}
func (w *smtImpliedEqualityStopwatch) stop()  {}

type smtImpliedEqualityTimers struct {
	stats, valEq smtImpliedEqualityStopwatch
}

func (t *smtImpliedEqualityTimers) startCall() *smtImpliedEqualityStopwatch { return nil }
func (t *smtImpliedEqualityTimers) stopCall(*smtImpliedEqualityStopwatch)   {}
func (t *smtImpliedEqualityTimers) startStaticValEq()                       {}
func (t *smtImpliedEqualityTimers) stopStaticValEq()                        {}
func (t *smtImpliedEqualityTimers) verbose(uint32, *smtImpliedEqualityStopwatch, uint32) error {
	return nil
}
