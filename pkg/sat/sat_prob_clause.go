package sat

// Source: src/sat/sat_prob.cpp:92-151,167-189.

func (p *satProb) reserveVariables(n uint32) {
	for uint32(len(p.values)) < n {
		p.values = append(p.values, false)
		p.best = append(p.best, false)
		p.breaks = append(p.breaks, 0)
	}
}

func (p *satProb) addClause(input []Literal) {
	c := append([]Literal(nil), input...)
	idx := uint32(len(p.clauses))
	p.clauses = append(p.clauses, c)
	p.info = append(p.info, satProbClauseInfo{})
	for _, l := range c {
		p.reserveVariables(uint32(l.variable()) + 1)
		for len(p.use) <= int(l.index()) {
			p.use = append(p.use, nil)
		}
		p.use[l.index()] = append(p.use[l.index()], idx)
	}
}

func (p *satProb) literalTrue(l Literal) bool { return p.values[l.variable()] != l.sign() }
func (p *satProb) initializeClauses() {
	for i := range p.breaks {
		p.breaks[i] = 0
	}
	p.unsat = nil
	p.unsatPos = make([]int, len(p.clauses))
	for i := range p.unsatPos {
		p.unsatPos[i] = -1
	}
	for i, c := range p.clauses {
		ci := satProbClauseInfo{}
		for _, l := range c {
			if p.literalTrue(l) {
				ci.truths++
				ci.sum += l.index()
			}
		}
		p.info[i] = ci
		if ci.truths == 0 {
			p.insertUnsat(uint32(i))
		} else if ci.truths == 1 {
			p.breaks[Literal(ci.sum).variable()]++
		}
	}
}
func (p *satProb) insertUnsat(i uint32) {
	if p.unsatPos[i] >= 0 {
		return
	}
	p.unsatPos[i] = len(p.unsat)
	p.unsat = append(p.unsat, i)
}
func (p *satProb) removeUnsat(i uint32) {
	pos := p.unsatPos[i]
	if pos < 0 {
		return
	}
	last := p.unsat[len(p.unsat)-1]
	p.unsat[pos] = last
	p.unsatPos[last] = pos
	p.unsat = p.unsat[:len(p.unsat)-1]
	p.unsatPos[i] = -1
}
func (p *satProb) saveBest() {
	p.bestUnsat = uint32(len(p.unsat))
	copy(p.best, p.values)
	p.model = make(Model, len(p.values))
	for i, v := range p.values {
		if v {
			p.model[i] = 1
		} else {
			p.model[i] = -1
		}
	}
}
