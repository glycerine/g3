package tactic

import (
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

// Copy the entire tuple of a satisfying component when its constraints
// match another component's after a consistent one-to-one variable renaming.
// Parameters and variable-free subterms keep their original identities.
func (p *polynomialComponents) groupCoupledSystems(forms []ast.ASTHandle) (bool, error) {
	if len(p.variables) < 4 {
		return false, nil
	}
	mixed := 0
	for _, h := range forms {
		if p.terms[h].variable == mixedComponent {
			mixed++
			if mixed == 2 {
				break
			}
		}
	}
	if mixed < 2 {
		return false, nil
	}

	g := coupledPolynomials{p: p, indices: make(map[ast.ASTHandle]int), parents: make([]uint32, len(p.variables)+1)}
	for i := range g.parents {
		g.parents[i] = uint32(i)
	}
	owners := make([]uint32, len(forms))
	for i, form := range forms {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return false, nil
		}
		v, ok, err := g.collect(form, 0)
		if err != nil || !ok {
			return false, err
		}
		owners[i] = v
	}

	systems := make([]polynomialSystem, len(g.parents))
	for i := range p.variables {
		v := uint32(i + 1)
		s := &systems[g.root(v)]
		s.variables = append(s.variables, v)
	}
	for i, v := range owners {
		if v != 0 {
			s := &systems[g.root(v)]
			s.forms = append(s.forms, forms[i])
		}
	}
	// Source encounter order supplies one bijection. Missing a different
	// permutation costs only a missed reduction. Bound work on large systems.
	slots := make([]uint32, len(g.parents))
	eligible := 0
	for i := range systems {
		s := &systems[i]
		if len(s.variables) < 2 || len(s.variables) > 8 || len(s.forms) > 192 {
			continue
		}
		eligible++
		for j, v := range s.variables {
			slots[v] = uint32(j + 1)
		}
	}
	if eligible < 2 {
		return false, nil
	}

	// A unary subterm already has an exact shape with one variable hole.
	// Fill the hole with the local slot. Only mixed nodes need new child lists.
	shapes := make(map[coupledShape]uint32)
	intern := func(k coupledShape) uint32 {
		if id := shapes[k]; id != 0 {
			return id
		}
		id := uint32(len(shapes) + 1)
		shapes[k] = id
		return id
	}
	shape := func(h ast.ASTHandle) uint32 {
		t := p.terms[h]
		if t.variable == mixedComponent {
			return g.nodes[g.indices[h]].shape
		}
		return intern(coupledShape{input: t.shape, local: slots[t.variable]})
	}
	sequences := make(map[componentLink]uint32)
	for i := range g.nodes {
		if i%64 == 0 && p.manager.Limit.CancelFlag() {
			return false, nil
		}
		n := &g.nodes[i]
		if slots[n.variable] == 0 {
			continue
		}
		var seq uint32
		for _, child := range g.children[n.first:n.end] {
			seq = componentAppend(sequences, seq, shape(child))
		}
		n.shape = intern(coupledShape{decl: n.decl, input: seq})
	}

	representatives := make(map[componentLink]int)
	aliases := 0
	for i := range systems {
		s := &systems[i]
		if len(s.variables) == 0 || slots[s.variables[0]] == 0 {
			continue
		}
		if p.manager.Limit.CancelFlag() {
			return false, nil
		}
		rows := make([]int, len(s.forms))
		for j, form := range s.forms {
			rows[j] = int(shape(form))
		}
		sort.Ints(rows)
		var seq, prev uint32
		for _, row := range rows {
			if id := uint32(row); id != prev {
				seq = componentAppend(sequences, seq, id)
				prev = id
			}
		}
		key := componentLink{uint32(len(s.variables)), seq}
		j, found := representatives[key]
		if !found {
			representatives[key] = i
			continue
		}
		if p.omitted == nil {
			p.omitted = make(map[ast.ASTHandle]bool)
		}
		for _, form := range s.forms {
			p.omitted[form] = true
		}
		for slot, v := range s.variables {
			p.variables[v-1].representative = systems[j].variables[slot]
		}
		aliases += len(s.variables)
	}
	return aliases >= 4, nil
}

type polynomialSystem struct {
	variables []uint32
	forms     []ast.ASTHandle
}

type coupledShape struct {
	decl         ast.ASTHandle // empty for closed or unary shapes
	input, local uint32
}

type coupledNode struct {
	decl            ast.ASTHandle
	first, end      int
	variable, shape uint32
}

type coupledPolynomials struct {
	p        *polynomialComponents
	parents  []uint32
	indices  map[ast.ASTHandle]int
	nodes    []coupledNode // children precede their parents
	children []ast.ASTHandle
}

func (g *coupledPolynomials) root(v uint32) uint32 {
	for g.parents[v] != v {
		g.parents[v] = g.parents[g.parents[v]]
		v = g.parents[v]
	}
	return v
}

func (g *coupledPolynomials) collect(h ast.ASTHandle, depth int) (uint32, bool, error) {
	p := g.p
	if v := p.terms[h].variable; v != mixedComponent {
		return v, true, nil
	}
	if i, found := g.indices[h]; found {
		return g.nodes[i].variable, true, nil
	}
	hd, err := p.manager.NodeHeader(h)
	if err != nil || depth > maxCandidateDepth || int(hd.NumArgs)+1 > maxCandidateEdges-p.work || p.manager.Limit.CancelFlag() {
		return 0, false, err
	}
	p.work += int(hd.NumArgs) + 1
	n := coupledNode{decl: hd.Decl, first: len(g.children), end: len(g.children) + int(hd.NumArgs)}
	g.children = append(g.children, make([]ast.ASTHandle, hd.NumArgs)...)
	for i := range hd.NumArgs {
		child, err := p.manager.GetArg(h, i)
		if err != nil {
			return 0, false, err
		}
		v, ok, err := g.collect(child, depth+1)
		if err != nil || !ok {
			return 0, false, err
		}
		g.children[n.first+int(i)] = child
		if v != 0 {
			if n.variable == 0 {
				n.variable = v
			} else {
				first, other := g.root(n.variable), g.root(v)
				g.parents[max(first, other)] = min(first, other)
			}
		}
	}
	g.indices[h] = len(g.nodes)
	g.nodes = append(g.nodes, n)
	return n.variable, true, nil
}
