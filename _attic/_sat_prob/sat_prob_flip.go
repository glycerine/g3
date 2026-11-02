package sat

// Source: src/sat/sat_prob.cpp:49-91.

func (p *satProb) pickVar() BoolVar {
	ci := p.unsat[p.random.Next()%uint32(len(p.unsat))]
	c := p.clauses[ci]
	if cap(p.probabilities) < len(c) {
		p.probabilities = make([]float64, len(c))
	} else {
		p.probabilities = p.probabilities[:len(c)]
	}
	sum := 0.0
	for i, l := range c {
		q := p.probBreak[p.breaks[l.variable()]]
		p.probabilities[i] = q
		sum += q
	}
	lim := sum * float64(p.random.Next()) / float64(0x7fff)
	i := len(c)
	for {
		i--
		lim -= p.probabilities[i]
		if lim < 0 || i == 0 {
			break
		}
	}
	return c[i].variable()
}
func (p *satProb) flipOne() {
	p.flip(p.pickVar())
	if uint32(len(p.unsat)) < p.bestUnsat {
		p.saveBest()
	}
}
func (p *satProb) flip(v BoolVar) {
	p.flips++
	lit := MakeLiteral(v, !p.values[v])
	nlit := lit.Negated()
	if int(lit.index()) < len(p.use) {
		for _, idx := range p.use[lit.index()] {
			ci := &p.info[idx]
			ci.truths--
			ci.sum -= lit.index()
			switch ci.truths {
			case 0:
				p.insertUnsat(idx)
				p.breaks[v]--
			case 1:
				p.breaks[Literal(ci.sum).variable()]++
			}
		}
	}
	if int(nlit.index()) < len(p.use) {
		for _, idx := range p.use[nlit.index()] {
			ci := &p.info[idx]
			switch ci.truths {
			case 0:
				p.removeUnsat(idx)
				p.breaks[v]++
			case 1:
				p.breaks[Literal(ci.sum).variable()]--
			}
			ci.truths++
			ci.sum += nlit.index()
		}
	}
	p.values[v] = !p.values[v]
}
