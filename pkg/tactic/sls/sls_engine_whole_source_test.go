package sls

// Whole-source behavioral audit for src/tactic/sls/sls_engine.cpp/.h.

import (
	"math"
	"testing"
)

func TestSLSEngineWholeSourceParamsRestartAndStatistics(t *testing.T) {
	engine, _, _, _, _, _ := slsEngineFixture(t)
	local := engine.context.EmptyValue()
	setBool := func(name string, value bool) {
		if err := engine.context.SetBoolName(&local, &name, value); err != nil {
			t.Fatal(err)
		}
	}
	setUInt := func(name string, value uint32) {
		if err := engine.context.SetUIntName(&local, &name, value); err != nil {
			t.Fatal(err)
		}
	}
	setUInt("max_restarts", 9)
	setBool("walksat", true)
	setBool("walksat_repick", true)
	setUInt("paws_sp", 1024)
	setUInt("wp", 77)
	setUInt("vns_mc", 3)
	setBool("vns_repick", true)
	setUInt("restart_base", 4)
	setBool("restart_init", true)
	setBool("early_prune", false)
	setBool("random_offset", false)
	setBool("rescore", false)
	setUInt("random_seed", 23)
	if err := engine.UpdateParams(&local); err != nil {
		t.Fatal(err)
	}
	if engine.maxRestarts != 9 || !engine.walksat || !engine.walksatRepick || engine.paws || engine.wp != 77 || engine.vnsMC != 3 || !engine.vnsRepick || engine.restartBase != 4 || engine.restartNext != 4 || !engine.restartInit || engine.earlyPrune || engine.randomOffset || engine.rescore {
		t.Fatal("source parameter publication", *engine)
	}

	engine.restartNext = 5
	engine.stats.Restarts = 0
	if engine.CheckRestart(6) || engine.restartNext != 13 {
		t.Fatal("even-restart source schedule", engine.restartNext)
	}
	engine.stats.Restarts = 1
	if engine.CheckRestart(14) || engine.restartNext != 17 {
		t.Fatal("odd-restart source schedule", engine.restartNext)
	}
	if !engine.CheckRestart(17) {
		t.Fatal("restart boundary must be strict greater-than")
	}

	engine.stats = EngineStats{Restarts: 2, FullEvaluations: 3, IncrementalEvaluations: 4, Moves: 5, Flips: 6, Increments: 7, Decrements: 8, Inversions: 9, Started: engine.stats.Started}
	statistics, err := engine.CollectStatistics()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{"sls restarts": 2, "sls full evals": 3, "sls incr evals": 4, "sls FLIP moves": 6, "sls INC moves": 7, "sls DEC moves": 8, "sls INV moves": 9, "sls moves": 5}
	for name, value := range want {
		if statistics[name] != value {
			t.Fatal(name, statistics[name], value)
		}
	}
	if math.IsNaN(statistics["sls moves/sec"]) || math.IsInf(statistics["sls moves/sec"], 0) {
		t.Fatal("invalid source rate", statistics)
	}
	priorRestartNext := engine.restartNext
	if err = engine.ResetStatistics(); err != nil {
		t.Fatal(err)
	}
	if engine.stats.Moves != 5 || engine.stats.Restarts != 2 || engine.stats.FullEvaluations != 0 ||
		engine.stats.IncrementalEvaluations != 0 || engine.stats.Flips != 0 ||
		engine.stats.Increments != 7 || engine.stats.Decrements != 8 || engine.stats.Inversions != 9 ||
		engine.restartNext != priorRestartNext {
		t.Fatal("source statistics reset", engine.stats, engine.restartNext)
	}
}
