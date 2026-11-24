package order

// Applications in a goal suggest substitutions for universal premises.
// A new instance can in turn supply applications for another rule. Only
// instances of positive universal premises and permanent ground facts are
// asserted. A sibling's goal never becomes a premise.
type observedPool struct {
	bySymbol map[uint32][]applicationRule
	scanned  []bool
}

type applicationRule struct {
	rule    int
	body    uint32
	pattern observedPattern
}

// MinConstantPoolRules selects the Cartesian product of goal constants and
// bounded residual SAT search for batches with many universal premises.
const MinConstantPoolRules = 192

const minApplicationRules = 3

func (p *GroundInstancePool) indexApplications() error {
	g := &p.g
	p.observed.bySymbol = make(map[uint32][]applicationRule)
	reader := observedInstances{source: g, ground: g}
	for i, r := range p.rules {
		body, err := p.source(r.body)
		if err != nil {
			return err
		}
		rule := observedRule{arity: uint32(r.n), body: body}
		for j := 0; j < r.n; j++ {
			rule.sorts[r.n-1-j] = r.sorts[j]
		}
		patterns, err := reader.patterns(&rule)
		if err != nil {
			return err
		}
		if len(patterns) > 0 {
			first := patterns[0]
			p.observed.bySymbol[first.symbol] = append(p.observed.bySymbol[first.symbol], applicationRule{rule: i, body: body, pattern: first})
		}
	}
	return nil
}

func (p *GroundInstancePool) matchApplications(id uint32) error {
	const perGoal = 32
	const perPool = 4000
	g, obs := &p.g, p.observed
	pending := []uint32{id}
	count := 0
	for len(pending) > 0 && count < perGoal && len(p.instances) < perPool {
		last := len(pending) - 1
		termID := pending[last]
		pending = pending[:last]
		if n := int(termID) + 1 - len(obs.scanned); n > 0 {
			obs.scanned = append(obs.scanned, make([]bool, n)...)
		}
		if obs.scanned[termID] {
			continue
		}
		obs.scanned[termID] = true
		term := g.terms[termID]
		args := g.arguments[term.start : term.start+term.count]
		if err := g.compiler.charge(1 + len(args)); err != nil {
			return err
		}
		pending = append(pending, args...)
		for _, candidate := range obs.bySymbol[term.symbol] {
			if count == perGoal || len(p.instances) == perPool {
				break
			}
			if err := g.compiler.charge(1); err != nil {
				return err
			}
			if term.count != candidate.pattern.arity {
				continue
			}
			r := &p.rules[candidate.rule]
			key := groundRuleInstance{id: candidate.rule}
			for j := 0; j < r.n; j++ {
				arg := args[candidate.pattern.position[r.n-1-j]]
				if p.sorts[g.terms[arg].symbol] != r.sorts[j] {
					return inapplicable
				}
				key.args[j] = arg
			}
			if p.instances[key] {
				continue
			}
			g.epoch++
			g.bindings, g.bindingSorts = key.args[:r.n], r.sorts[:r.n]
			root, err := g.ground(candidate.body, 0)
			g.bindings, g.bindingSorts = nil, nil
			if err != nil {
				return err
			}
			p.instances[key] = true
			g.roots = append(g.roots, root)
			pending = append(pending, root)
			count++
		}
	}
	return nil
}
