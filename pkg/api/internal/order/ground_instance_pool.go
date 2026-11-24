package order

import (
	"slices"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/control"
)

// GroundInstancePool retains a Boolean relaxation of ground consequences of
// one ordered premise list. Each worker owns its pool and its source arena.
// Source handles are weak identities. Source premises stay alive in the
// caller's solver, and goals only contribute assumptions and definitions.
type GroundInstancePool struct {
	manager                   *ast.ASTManager
	base                      []ast.ASTHandle
	g                         groundProjection
	observed                  *observedPool
	rules                     []flatUniversalRule
	facts                     []ast.ASTHandle
	indexed, loaded, disabled bool
	installed                 map[[8]ast.ASTHandle]bool
	instances                 map[groundRuleInstance]bool
	groundBoolEncoding
	units instanceUnits
}

type flatUniversalRule struct {
	body  ast.ASTHandle
	sorts [4]ast.ASTHandle
	n     int
}

type groundRuleInstance struct {
	id   int
	args [4]uint32
}

// GroundInstanceQuery borrows the pool's append-only clause stream until its
// next Project call. Only a contradiction under Goal establishes a refutation.
type GroundInstanceQuery struct {
	Variables int
	Clauses   []BooleanClause
	Goal      int32
	Work      uint32
	units     *instanceUnits
}

func NewGroundInstancePool(m *ast.ASTManager, base []ast.ASTHandle) *GroundInstancePool {
	return &GroundInstancePool{manager: m, base: slices.Clone(base)}
}

func (p *GroundInstancePool) Matches(m *ast.ASTManager, base []ast.ASTHandle) bool {
	return p != nil && p.manager == m && slices.Equal(p.base, base)
}

func (p *GroundInstancePool) start(steps uint32) {
	p.g = newGroundProjection(p.manager, steps)
	k := &p.g.compiler
	k.Vars = 1
	k.firstGate = 2
	k.gates = make(map[uint64]int32)
	k.Clauses = []BooleanClause{{1}}
	if p.observed == nil {
		p.installed = make(map[[8]ast.ASTHandle]bool)
	}
	p.instances = make(map[groundRuleInstance]bool)
	p.groundBoolEncoding = groundBoolEncoding{
		result: []int32{0}, codes: []groundValueCode{{}},
		labels: make(map[ast.ASTHandle]int), group: make(map[uint32][]uint32),
		sorts: make(map[uint32]ast.ASTHandle), dataITE: make(map[uint32]bool),
		equal: make(map[[2]uint32]int32), nextInput: 1,
	}
}

// disable drops an exhausted or inapplicable encoding. Retaining the premise
// identities avoids charging every subsequent sibling for the same attempt.
func (p *GroundInstancePool) disable() {
	*p = GroundInstancePool{manager: p.manager, base: p.base, disabled: true}
}

func (p *GroundInstancePool) Disabled() bool { return p.disabled }

// Only positive top-level conjuncts supply permanent facts or universal rules.
func (p *GroundInstancePool) index(limit *control.Reslimit) error {
	m := p.manager
	todo := slices.Clone(p.base)
	seen := map[ast.ASTHandle]bool{}
	for len(todo) > 0 {
		if len(todo) > 8192 || len(seen) > 8192 || m.ResourceLimit().CancelFlag() {
			return errBitLimit
		}
		i := len(todo) - 1
		h := todo[i]
		todo = todo[:i]
		if seen[h] {
			continue
		}
		seen[h] = true
		hd, e := m.NodeHeader(h)
		if e != nil {
			return e
		}
		if !limit.Inc() {
			return errBitLimit
		}
		if hd.Kind == ast.ASTApp && hd.Family == m.BasicFamilyID() && hd.DeclKind == ast.ASTOpAnd {
			if uint64(hd.NumArgs)+uint64(len(todo)) > 8192 || !limit.IncOffset(hd.NumArgs) {
				return errBitLimit
			}
			for j := hd.NumArgs; j > 0; j-- {
				v, e := m.GetArg(h, j-1)
				if e != nil {
					return e
				}
				todo = append(todo, v)
			}
			continue
		}
		if hd.Kind != ast.ASTQuantifier {
			ok, e := m.IsGround(h)
			if e != nil {
				return e
			}
			if ok {
				p.facts = append(p.facts, h)
			}
			continue
		}
		all, e := m.IsForall(h)
		if e != nil {
			return e
		}
		if !all {
			continue
		}
		n, e := m.GetNumDeclarations(h)
		if e != nil {
			return e
		}
		if n == 0 || n > 4 {
			continue
		}
		body, e := m.GetQuantifierBody(h)
		if e != nil {
			return e
		}
		nested, e := m.HasQuantifiers(body)
		if e != nil {
			return e
		}
		if nested {
			continue
		}
		rule := flatUniversalRule{n: int(n), body: body}
		for j := uint32(0); j < n; j++ {
			rule.sorts[j], e = m.GetDeclarationSort(h, j)
			if e != nil {
				return e
			}
		}
		p.rules = append(p.rules, rule)
	}
	p.indexed = true
	return nil
}

// Distinct nullary data terms in the goal provide typed substitutions.
func (p *GroundInstancePool) goalConstants(q ast.ASTHandle, limit *control.Reslimit) (k [8]ast.ASTHandle, n int, err error) {
	m := p.manager
	ground, e := m.IsGround(q)
	if e != nil || !ground {
		return k, 0, e
	}
	todo := []ast.ASTHandle{q}
	seen := map[ast.ASTHandle]bool{}
	for len(todo) != 0 {
		if len(seen) > 256 || len(todo) > 512 || m.ResourceLimit().CancelFlag() {
			return k, 0, nil
		}
		i := len(todo) - 1
		h := todo[i]
		todo = todo[:i]
		if seen[h] {
			continue
		}
		seen[h] = true
		hd, e := m.NodeHeader(h)
		if e != nil || hd.Kind != ast.ASTApp || hd.NumArgs > 256 {
			return k, 0, e
		}
		if !limit.IncOffset(1 + hd.NumArgs) {
			return k, 0, nil
		}
		if hd.NumArgs == 0 && hd.Family == ast.NullFamilyID {
			st, e := m.GetSort(h)
			if e != nil {
				return k, 0, e
			}
			ok, e := m.IsUninterpretedSort(st)
			if e != nil {
				return k, 0, e
			}
			if ok {
				if n == len(k) {
					return k, 0, nil
				}
				k[n] = h
				n++
			}
		}
		for j := uint32(0); j < hd.NumArgs; j++ {
			v, e := m.GetArg(h, j)
			if e != nil {
				return k, 0, e
			}
			todo = append(todo, v)
		}
	}
	ast.SortHandles(k[:n])
	return k, n, nil
}

// Record a declaration's result sort and term-ITE semantics while its source
// node is live. Later goals can reuse compact ground terms after an AST pop.
func (p *GroundInstancePool) source(h ast.ASTHandle) (uint32, error) {
	g := &p.g
	v, e := g.read(h, 0)
	if e != nil {
		return 0, e
	}
	if d := len(g.input) - len(g.global); d > 0 {
		g.global = append(g.global, make([]uint32, d)...)
		g.local = append(g.local, make([]uint32, d)...)
		g.version = append(g.version, make([]uint32, d)...)
	}
	for ; p.nextInput < len(g.input); p.nextInput++ {
		n := &g.input[p.nextInput]
		if n.kind != ast.ASTApp || n.boolean && p.observed == nil {
			continue
		}
		if _, ok := p.sorts[n.symbol]; ok {
			continue
		}
		st, e := p.manager.GetSort(n.expr)
		if e != nil {
			return 0, e
		}
		p.sorts[n.symbol] = st
		if p.observed != nil {
			continue
		}
		hd, e := p.manager.NodeHeader(n.expr)
		if e != nil {
			return 0, e
		}
		p.dataITE[n.symbol] = hd.Family == p.manager.BasicFamilyID() && hd.DeclKind == ast.ASTOpIte && hd.NumArgs == 3
	}
	return v, nil
}

func (p *GroundInstancePool) root(h ast.ASTHandle) (uint32, error) {
	id, e := p.source(h)
	if e != nil {
		return 0, e
	}
	return p.g.ground(id, 0)
}

// Use up to three observed constants of each sort and 32 tuples per rule.
// Each installed tuple is a source consequence independently of how many
// other tuples or domain elements the source model contains.
func (p *GroundInstancePool) join(k [8]ast.ASTHandle, n int) error {
	g := &p.g
	if !p.installed[k] {
		if len(p.installed) >= 1024 {
			return errBitLimit
		}
		by := map[ast.ASTHandle][]uint32{}
		for _, h := range k[:n] {
			id, e := p.source(h)
			if e != nil {
				return e
			}
			st := g.input[id].sort
			if st.IsNull() {
				return inapplicable
			}
			if len(by[st]) < 3 {
				by[st] = append(by[st], id)
			}
		}
		for id, r := range p.rules {
			var sets [4][]uint32
			comb := 1
			for j := 0; j < r.n; j++ {
				sets[j] = by[r.sorts[j]]
				comb *= len(sets[j])
			}
			if comb == 0 {
				continue
			}
			if e := g.compiler.charge(1); e != nil {
				return e
			}
			var root uint32
			for j := 0; j < min(comb, 32); j++ {
				key := groundRuleInstance{id: id}
				off := j
				for x := r.n - 1; x >= 0; x-- {
					key.args[x] = sets[x][off%len(sets[x])]
					off /= len(sets[x])
				}
				if p.instances[key] {
					continue
				}
				if len(p.instances) >= 9000 {
					return errBitLimit
				}
				if root == 0 {
					var e error
					root, e = p.source(r.body)
					if e != nil {
						return e
					}
				}
				g.bindings = key.args[:r.n]
				g.epoch++
				v, e := g.ground(root, 0)
				g.bindings = nil
				if e != nil {
					return e
				}
				g.roots = append(g.roots, v)
				p.instances[key] = true
			}
		}
		p.installed[k] = true
	}
	return nil
}

func (p *GroundInstancePool) loadFacts() error {
	g := &p.g
	if !p.loaded {
		for _, h := range p.facts {
			v, e := p.root(h)
			if e != nil {
				return e
			}
			g.roots = append(g.roots, v)
		}
		p.loaded = true
	}
	return nil
}

// Project builds the current goal as an assumption. minRules selects the
// minimum number of eligible universal rules that justifies starting a pool.
func (p *GroundInstancePool) Project(goal ast.ASTHandle, steps uint32, minRules int, limit *control.Reslimit) (*GroundInstanceQuery, error) {
	return p.project(goal, steps, minRules, false, limit)
}

// ProjectObserved uses matches on uninterpreted applications when the
// number of universal premises is small enough to index their patterns.
func (p *GroundInstancePool) ProjectObserved(goal ast.ASTHandle, steps uint32, limit *control.Reslimit) (*GroundInstanceQuery, error) {
	return p.project(goal, steps, minApplicationRules, true, limit)
}

func (p *GroundInstancePool) project(goal ast.ASTHandle, steps uint32, minRules int, observed bool, limit *control.Reslimit) (query *GroundInstanceQuery, err error) {
	if p.disabled || steps == 0 {
		return nil, nil
	}
	if len(p.base) > 8191 {
		p.disable()
		return nil, nil
	}
	eligible := func() bool { return len(p.rules) >= minRules && (!observed || len(p.rules) < MinConstantPoolRules) }
	if p.indexed && (!eligible() || p.g.manager != nil && observed != (p.observed != nil)) {
		return nil, nil
	}
	var key [8]ast.ASTHandle
	var n int
	if observed {
		ground, err := p.manager.IsGround(goal)
		if err != nil || !ground {
			return nil, err
		}
	} else {
		key, n, err = p.goalConstants(goal, limit)
		if err != nil || n == 0 {
			return nil, err
		}
	}
	defer func() {
		if err == nil {
			return
		}
		p.disable()
		if err == inapplicable || err == errBitLimit {
			query, err = nil, nil
		}
	}()
	if !p.indexed {
		if err := p.index(limit); err != nil {
			return nil, err
		}
	}
	if !eligible() {
		return nil, nil
	}
	if p.g.manager == nil {
		if observed {
			p.observed = new(observedPool)
		}
		p.start(steps)
	}
	k := &p.g.compiler
	k.work, k.nextPoll, k.ceiling = 0, 0, int(min(steps, 500_000))
	if observed && p.observed.bySymbol == nil {
		if err := p.indexApplications(); err != nil {
			return nil, err
		}
	}
	if err := p.loadFacts(); err != nil {
		return nil, err
	}
	if !observed {
		if err := p.join(key, n); err != nil {
			return nil, err
		}
		// Preserve the constant pool's clause order when adding a new sibling.
		if err := p.compileFacts(); err != nil {
			return nil, err
		}
	}
	id, err := p.root(goal)
	if err != nil {
		return nil, err
	}
	if observed {
		if err := p.matchApplications(id); err != nil {
			return nil, err
		}
	}
	if err := p.compileFacts(); err != nil {
		return nil, err
	}
	lit := p.result[id]
	if lit == 0 {
		return nil, inapplicable
	}
	p.need(lit)
	if len(k.Clauses) > 120_000 {
		return nil, errBitLimit
	}
	if err := k.charge(0); err != nil {
		return nil, err
	}
	return &GroundInstanceQuery{Variables: k.Vars, Clauses: k.Clauses, Goal: lit, Work: uint32(k.work), units: &p.units}, nil
}

func (p *GroundInstancePool) compileFacts() error {
	g := &p.g
	if e := p.compileTerms(); e != nil {
		return e
	}
	for _, v := range g.roots {
		p.assertLiteral(p.result[v])
	}
	g.roots = g.roots[:0]
	return nil
}
