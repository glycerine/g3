package sat

// Source: src/sat/sat_prob.cpp:247-264; src/util/luby.h.

import "github.com/glycerine/g3/pkg/util/luby"

func (p *satProb) restart() {
	copy(p.values, p.best)
	for v := range p.values {
		if p.random.Next()%100 < p.randomInit {
			p.values[v] = !p.values[v]
		}
	}
	p.initializeClauses()
	n, _ := luby.Value(p.restarts)
	p.restarts++
	p.nextRestart += p.restartOffset * uint64(n)
}
