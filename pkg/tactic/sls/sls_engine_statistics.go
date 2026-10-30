package sls

// Source: src/tactic/sls/sls_engine.h:40-62,101-102 and
// src/tactic/sls/sls_engine.cpp:78-100.

import "time"

// CollectStatistics publishes the exact source statistics names. The engine
// method is useful independently of the tactic wrapper (notably to BVSLS).
func (e *Engine) CollectStatistics() (map[string]float64, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	s := e.stats
	seconds := time.Since(s.Started).Seconds()
	if seconds <= 0 {
		seconds = 1
	}
	return map[string]float64{
		"sls restarts":       float64(s.Restarts),
		"sls full evals":     float64(s.FullEvaluations),
		"sls incr evals":     float64(s.IncrementalEvaluations),
		"sls incr evals/sec": float64(s.IncrementalEvaluations) / seconds,
		"sls FLIP moves":     float64(s.Flips),
		"sls INC moves":      float64(s.Increments),
		"sls DEC moves":      float64(s.Decrements),
		"sls INV moves":      float64(s.Inversions),
		"sls moves":          float64(s.Moves),
		"sls moves/sec":      float64(s.Moves) / seconds,
	}, nil
}

// ResetStatistics preserves the source separation between statistical state
// and the restart schedule/parameters.
func (e *Engine) ResetStatistics() error {
	if err := e.open(); err != nil {
		return err
	}
	e.stats = EngineStats{Started: time.Now()}
	return nil
}
