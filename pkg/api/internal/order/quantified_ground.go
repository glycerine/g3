package order

import "github.com/glycerine/g3/pkg/ast"

type groundApplication struct {
	symbol, next uint32
	start, count uint32
	operation    int32
	boolean      bool
}

func groundTermHash(symbol uint32, args []uint32) uint64 {
	hash := uint64(symbol) + 1
	for _, id := range args {
		hash = (hash ^ uint64(id)) * 0x9e3779b185ebca87
	}
	return hash
}

// Ground terms are private integer IDs. A hash collision must match the
// complete declaration and argument sequence before it can reuse an ID.
func (g *groundProjection) intern(n *projectionNode, args []uint32) (uint32, error) {
	hash := groundTermHash(n.symbol, args)
	head := g.applications[hash]
	for id := head; id != 0; id = g.terms[id].next {
		t := &g.terms[id]
		if err := g.compiler.charge(1 + len(args)); err != nil {
			return 0, err
		}
		if t.symbol != n.symbol || int(t.count) != len(args) {
			continue
		}
		prev := g.arguments[t.start : t.start+t.count]
		match := true
		for j, arg := range args {
			if prev[j] != arg {
				match = false
				break
			}
		}
		if match {
			return id, nil
		}
	}
	id := uint32(len(g.terms))
	g.terms = append(g.terms, groundApplication{
		symbol: n.symbol, next: head, start: uint32(len(g.arguments)), count: uint32(len(args)),
		operation: n.operation, boolean: n.boolean,
	})
	g.arguments = append(g.arguments, args...)
	g.applications[hash] = id
	return id, nil
}

// Global entries denote closed syntax. Subexpressions with variables can
// reuse a local result only within the current binder's substitution.
func (g *groundProjection) ground(i uint32, depth int) (uint32, error) {
	if id := g.global[i]; id != 0 {
		return id, nil
	}
	if g.epoch != 0 && g.version[i] == g.epoch {
		return g.local[i], nil
	}
	if depth > 192 || len(g.terms) > 65_000 {
		return 0, errBitLimit
	}
	n := &g.input[i]
	if n.kind == ast.ASTVar {
		if uint64(n.index) >= uint64(len(g.bindings)) {
			return 0, inapplicable
		}
		w := g.bindings[len(g.bindings)-1-int(n.index)]
		// Application matches bind directly to interned ground terms. Constant
		// tuples in the ordinary projection bind to source input nodes.
		if g.bindingSorts != nil {
			if n.sort != g.bindingSorts[len(g.bindingSorts)-1-int(n.index)] {
				return 0, inapplicable
			}
			return w, nil
		}
		if n.sort != g.input[w].sort {
			return 0, inapplicable
		}
		return g.ground(w, depth+1)
	}
	count := n.count
	if n.kind == ast.ASTQuantifier {
		// A quantifier visited from an assertion becomes a propositional
		// atom. Its body is visited separately with its own bindings.
		if len(g.bindings) != 0 {
			return 0, inapplicable
		}
		count = 0
	}
	if err := g.compiler.charge(1 + int(count)); err != nil {
		return 0, err
	}
	var small [8]uint32
	args := small[:0]
	if count > uint32(len(small)) {
		args = make([]uint32, 0, count)
	}
	for _, ch := range g.children[n.start : n.start+count] {
		a, err := g.ground(ch, depth+1)
		if err != nil {
			return 0, err
		}
		args = append(args, a)
	}
	if n.operation == ast.ASTOpEq && args[1] < args[0] {
		args[0], args[1] = args[1], args[0]
	}
	id, err := g.intern(n, args)
	if err != nil {
		return 0, err
	}
	if n.ground || len(g.bindings) == 0 {
		g.global[i] = id
		if n.kind == ast.ASTQuantifier {
			g.quantifiers = append(g.quantifiers, i)
		}
	} else {
		g.local[i], g.version[i] = id, g.epoch
	}
	return id, nil
}

func (g *groundProjection) addInstance(i uint32) error {
	q := &g.input[i]
	body := g.children[q.start]
	nested, err := g.manager.HasQuantifiers(g.input[body].expr)
	if err != nil || nested {
		return err
	}
	n, err := g.manager.GetNumDeclarations(q.expr)
	if err != nil || n == 0 || n > 12 {
		return err
	}
	g.bindings = g.bindings[:0]
	for j := uint32(0); j < n; j++ {
		sort, err := g.manager.GetDeclarationSort(q.expr, j)
		if err != nil {
			return err
		}
		term := g.constants[sort]
		if term == 0 {
			return nil
		}
		g.bindings = append(g.bindings, term)
	}
	g.epoch++
	instance, err := g.ground(body, 0)
	if err != nil {
		return err
	}
	forall, err := g.manager.IsForall(q.expr)
	if err != nil {
		return err
	}
	pair := groundImplication{from: g.global[i], to: instance}
	if !forall {
		pair.from, pair.to = pair.to, pair.from
	}
	g.implications = append(g.implications, pair)
	return nil
}
