package sat

// Source: src/sat/sat_prob.h; sat_prob.cpp:24-47,191-245.

import (
	"math"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

type satProbClauseInfo struct{ truths, sum uint32 }

type satProb struct {
	limit                    *control.Reslimit
	random                   *z3rand.Generator
	clauses                  [][]Literal
	info                     []satProbClauseInfo
	values, best             []bool
	use                      [][]uint32
	breaks                   []uint32
	unsat                    []uint32
	unsatPos                 []int
	probBreak, probabilities []float64
	model                    Model
	bestUnsat                uint32
	flips, nextRestart       uint64
	restarts                 uint32
	restartOffset            uint64
	randomInit               uint32
}

func newSATProb(limit *control.Reslimit, seed uint32) *satProb {
	return &satProb{limit: limit, random: z3rand.New(seed), restartOffset: 1000000, randomInit: 4}
}

func (p *satProb) check() lbool.LBool {
	p.initialize()
	for p.bestUnsat != 0 && p.limit.Inc() {
		if p.flips >= p.nextRestart {
			p.restart()
		} else {
			p.flipOne()
		}
	}
	if p.bestUnsat == 0 {
		return lbool.True
	}
	return lbool.Undef
}

func (p *satProb) initialize() {
	for i := range p.values {
		p.values[i] = p.random.Next()%2 == 0
	}
	p.initializeClauses()
	p.autoConfigure()
	p.saveBest()
	p.restarts, p.flips = 1, 0
	p.nextRestart = p.restartOffset
}

func (p *satProb) autoConfigure() {
	maxLen, maxOcc := 0, 0
	for _, c := range p.clauses {
		if len(c) > maxLen {
			maxLen = len(c)
		}
	}
	for _, u := range p.use {
		if len(u) > maxOcc {
			maxOcc = len(u)
		}
	}
	cb := 5.4
	switch maxLen {
	case 0, 1, 2, 3:
		cb = 2.5
	case 4:
		cb = 2.85
	case 5:
		cb = 3.7
	case 6:
		cb = 5.1
	}
	p.probBreak = make([]float64, maxOcc+1)
	for i := range p.probBreak {
		p.probBreak[i] = math.Pow(cb, -float64(i))
	}
}
