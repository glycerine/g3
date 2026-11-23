package order

import "github.com/glycerine/g3/pkg/ast"

// A positive universal premise entails its body at every well-sorted ground
// tuple. Choose tuples from applications already present in the ground facts.
// Even a partial syntactic match is safe because we assert the entire body.
type observedRule struct {
	body  uint32
	arity uint32
	sorts [4]ast.ASTHandle // Indexed by de Bruijn index.
}

type observedInstances struct {
	source, ground *groundProjection
	groundEnd      uint32
	terms          map[groundSignature]uint32
	applications   map[uint32][]uint32
}

func (g *groundProjection) addObservedInstances(premises []ast.ASTHandle) error {
	end, edges := len(g.input), len(g.children)
	if end == 1 || len(premises) == 0 {
		return nil
	}
	var rules []observedRule
	for _, h := range premises {
		if err := g.compiler.charge(1); err != nil {
			return err
		}
		rule, err := g.readObservedRule(h)
		if err != nil {
			return err
		}
		if rule.arity != 0 {
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return nil
	}

	// Both DAGs share the immutable ground prefix. Appending an instantiated
	// term must allocate past that prefix, leaving the source bodies readable.
	source := *g
	g.input, g.children = g.input[:end:end], g.children[:edges:edges]
	v := observedInstances{source: &source, ground: g, groundEnd: uint32(end),
		terms: make(map[groundSignature]uint32), applications: make(map[uint32][]uint32)}
	for id := uint32(1); id < v.groundEnd; id++ {
		n := &g.input[id]
		if err := g.compiler.charge(1 + int(n.count)); err != nil {
			return err
		}
		if n.count > 4 {
			continue
		}
		key := groundSignature{symbol: n.symbol}
		copy(key.args[:], g.children[n.start:n.start+n.count])
		v.terms[key] = id
		if n.count != 0 {
			v.applications[n.symbol] = append(v.applications[n.symbol], id)
		}
	}

	const maxInstances = 640
	added := 0
	for _, rule := range rules {
		if added >= maxInstances {
			break
		}
		count, err := v.addRule(rule, min(8, maxInstances-added))
		if err != nil {
			return err
		}
		added += count
	}
	return nil
}

func (g *groundProjection) readObservedRule(h ast.ASTHandle) (observedRule, error) {
	var rule observedRule
	hd, err := g.manager.NodeHeader(h)
	if err != nil || hd.Kind != ast.ASTQuantifier {
		return rule, err
	}
	universal, err := g.manager.IsForall(h)
	if err != nil || !universal {
		return rule, err
	}
	arity, err := g.manager.GetNumDeclarations(h)
	if err != nil || arity == 0 || arity > uint32(len(rule.sorts)) {
		return rule, err
	}
	body, err := g.manager.GetQuantifierBody(h)
	if err != nil {
		return rule, err
	}
	nested, err := g.manager.HasQuantifiers(body)
	if err != nil || nested {
		return rule, err
	}
	id, err := g.read(body, 0)
	if err != nil {
		return rule, err
	}
	if !g.input[id].boolean {
		return rule, inapplicable
	}
	if err := g.compiler.charge(int(arity)); err != nil {
		return rule, err
	}
	for j := range arity {
		rule.sorts[arity-1-j], err = g.manager.GetDeclarationSort(h, j)
		if err != nil {
			return rule, err
		}
	}
	rule.arity, rule.body = arity, id
	return rule, nil
}

type observedPattern struct {
	terms    []uint32
	arity    uint32
	position [4]uint32
}

// Prefer the least frequent application that supplies every bound variable.
// For a defining equation, start from its left side.
func (v *observedInstances) pattern(rule observedRule) (observedPattern, error) {
	var best observedPattern
	s := v.source
	id := rule.body
	if n := &s.input[id]; n.operation == ast.ASTOpEq || n.operation == ast.ASTOpIff {
		id = s.children[n.start]
	}
	stack := []uint32{id}
	for visits := 0; len(stack) != 0 && visits < 45; visits++ {
		id = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n := &s.input[id]
		if err := v.ground.compiler.charge(1 + int(n.count)); err != nil {
			return best, err
		}
		if n.kind != ast.ASTApp || n.count == 0 || n.count > 8 {
			continue
		}
		children := s.children[n.start : n.start+n.count]
		stack = append(stack, children...)
		hits := v.applications[n.symbol]
		if len(hits) == 0 || n.count > 4 || len(best.terms) != 0 && len(hits) >= len(best.terms) {
			continue
		}
		hd, err := s.manager.NodeHeader(n.expr)
		if err != nil {
			return best, err
		}
		if hd.Family != ast.NullFamilyID {
			continue
		}
		var mask uint32
		var positions [4]uint32
		for j, ch := range children {
			x := &s.input[ch]
			if x.kind == ast.ASTVar && x.index < rule.arity && x.sort == rule.sorts[x.index] {
				positions[x.index] = uint32(j)
				mask |= 1 << x.index
			}
		}
		if mask == (1<<rule.arity)-1 {
			best = observedPattern{terms: hits, arity: n.count, position: positions}
		}
	}
	return best, nil
}

func (v *observedInstances) addRule(rule observedRule, limit int) (int, error) {
	p, err := v.pattern(rule)
	if err != nil {
		return 0, err
	}
	s, g := v.source, v.ground
	var seen [8][4]uint32
	tried, added := 0, 0
	for _, id := range p.terms {
		if tried >= limit {
			break
		}
		if err := g.compiler.charge(1 + int(rule.arity)); err != nil {
			return 0, err
		}
		n := &s.input[id]
		if n.count != p.arity {
			continue
		}
		var binding [4]uint32
		for j := range rule.arity {
			arg := s.children[n.start+p.position[j]]
			sort, err := s.manager.GetSort(s.input[arg].expr)
			if err != nil {
				return 0, err
			}
			if sort != rule.sorts[j] {
				return 0, inapplicable
			}
			binding[j] = arg
		}
		duplicate := false
		for _, prev := range seen[:tried] {
			if prev == binding {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		seen[tried] = binding
		tried++
		inst, err := v.instantiate(rule.body, &rule, &binding, 0)
		if err != nil {
			return 0, err
		}
		if inst != 0 {
			g.roots = append(g.roots, inst)
			added++
		}
	}
	return added, nil
}

func (v *observedInstances) instantiate(id uint32, rule *observedRule, binding *[4]uint32, depth int) (uint32, error) {
	g := v.ground
	if err := g.compiler.charge(1); err != nil {
		return 0, err
	}
	if id < v.groundEnd {
		return id, nil
	}
	n := v.source.input[id]
	if n.kind == ast.ASTVar {
		if n.index >= rule.arity || n.sort != rule.sorts[n.index] {
			return 0, nil
		}
		return binding[n.index], nil
	}
	if n.kind != ast.ASTApp || n.count > 4 || depth > 16 {
		return 0, nil
	}
	key := groundSignature{symbol: n.symbol}
	for j, child := range v.source.children[n.start : n.start+n.count] {
		arg, err := v.instantiate(child, rule, binding, depth+1)
		if err != nil || arg == 0 {
			return 0, err
		}
		key.args[j] = arg
	}
	if prev := v.terms[key]; prev != 0 {
		return prev, nil
	}
	if len(g.input) >= 18_000 {
		return 0, errBitLimit
	}
	id = uint32(len(g.input))
	n.start = uint32(len(g.children))
	g.children = append(g.children, key.args[:n.count]...)
	g.input = append(g.input, n)
	v.terms[key] = id
	return id, nil
}
