package order

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/backtrack"
)

// ProjectGroundEqualities builds a Boolean relaxation of positive ground
// conjuncts. Equalities forced by the premises can identify applications of
// the same declaration. Only an UNSAT result on the relaxation is conclusive.
func ProjectGroundEqualities(m *ast.ASTManager, conjuncts []ast.ASTHandle, steps uint32) (*BooleanProblem, error) {
	if len(conjuncts) == 0 || len(conjuncts) > 16_000 || m.ResourceLimit().CancelFlag() {
		return nil, nil
	}
	g := newGroundProjection(m, steps)
	problem, err := g.equalitiesProblem(conjuncts)
	if err == errBitLimit || err == inapplicable {
		return nil, nil
	}
	return problem, err
}

func (g *groundProjection) equalitiesProblem(conjuncts []ast.ASTHandle) (*BooleanProblem, error) {
	for _, h := range conjuncts {
		if err := g.compiler.charge(1); err != nil {
			return nil, err
		}
		ground, err := g.manager.IsGround(h)
		if err != nil {
			return nil, err
		}
		if !ground {
			continue
		}
		id, err := g.read(h, 0)
		if err != nil {
			return nil, err
		}
		if !g.input[id].boolean {
			return nil, inapplicable
		}
		g.roots = append(g.roots, id)
	}

	var classes backtrack.BasicUnionFind
	if err := g.mergeRequiredEqualities(&classes); err != nil {
		return nil, err
	}
	if err := g.mergeCongruentTerms(&classes); err != nil {
		return nil, err
	}
	return g.encodeGroundClasses(&classes)
}

// The input DAG is in postorder. By the time we visit a node backwards,
// all parents have contributed the polarities they force on it.
func (g *groundProjection) mergeRequiredEqualities(classes *backtrack.BasicUnionFind) error {
	const positive, negative = 1, 2
	forced := make([]uint8, len(g.input))
	for _, id := range g.roots {
		forced[id] = positive
	}
	for i := len(g.input) - 1; i > 0; i-- {
		n := &g.input[i]
		if err := g.compiler.charge(1 + int(n.count)); err != nil {
			return err
		}
		sign := forced[i]
		if sign == 0 {
			continue
		}
		a := g.children[n.start : n.start+n.count]
		switch n.operation {
		case ast.ASTOpNot:
			forced[a[0]] |= (sign&positive)<<1 | (sign&negative)>>1
		case ast.ASTOpAnd, ast.ASTOpOr:
			required := sign & positive
			if n.operation == ast.ASTOpOr {
				required = sign & negative
			}
			for _, child := range a {
				forced[child] |= required
			}
		case ast.ASTOpImplies:
			if sign&negative != 0 {
				forced[a[0]] |= positive
				forced[a[1]] |= negative
			}
		case ast.ASTOpEq:
			if sign&positive != 0 {
				classes.Merge(a[0], a[1])
			}
		}
	}
	return nil
}

type groundSignature struct {
	symbol uint32
	args   [4]uint32
}

// A later merge can enable further congruences. Two bounded passes suffice for
// common nested functions and conditionals. Missing a merge weakens the proof.
func (g *groundProjection) mergeCongruentTerms(classes *backtrack.BasicUnionFind) error {
	known := make(map[groundSignature]uint32)
	for range 2 {
		clear(known)
		for i := uint32(1); i < uint32(len(g.input)); i++ {
			n := &g.input[i]
			if err := g.compiler.charge(1 + int(n.count)); err != nil {
				return err
			}
			if n.count == 0 || n.count > 4 {
				continue
			}
			k := groundSignature{symbol: n.symbol}
			a := g.children[n.start : n.start+n.count]
			for j, child := range a {
				k.args[j] = classes.Find(child)
			}
			if n.operation == ast.ASTOpEq && k.args[0] > k.args[1] {
				k.args[0], k.args[1] = k.args[1], k.args[0]
			}
			if n.count == 3 && k.args[1] == k.args[2] {
				// Data-valued conditionals are opaque to the Boolean compiler.
				h, err := g.manager.NodeHeader(n.expr)
				if err != nil {
					return err
				}
				if h.Family == g.manager.BasicFamilyID() && h.DeclKind == ast.ASTOpIte {
					classes.Merge(i, a[1])
				}
			}
			if old := known[k]; old != 0 {
				classes.Merge(i, old)
			} else {
				known[k] = i
			}
		}
	}
	return nil
}

// Keep Boolean gates in source order. Opaque Boolean applications in one
// class share a single SAT variable. Distinct classes can still be equal.
func (g *groundProjection) encodeGroundClasses(classes *backtrack.BasicUnionFind) (*BooleanProblem, error) {
	first := make([]uint32, len(g.input))
	ids := make([]uint32, len(g.input))
	g.terms = make([]groundApplication, len(g.input))
	g.arguments = g.children
	for i := uint32(1); i < uint32(len(g.input)); i++ {
		n := &g.input[i]
		if err := g.compiler.charge(1 + int(n.count)); err != nil {
			return nil, err
		}
		a := g.children[n.start : n.start+n.count]
		op := n.operation
		if op == ast.ASTOpEq {
			switch {
			case classes.Find(a[0]) == classes.Find(a[1]):
				op = ast.ASTOpTrue
			case !g.input[a[0]].boolean:
				op = opaqueOperation
			}
		}
		ids[i] = i
		count := n.count
		if op == opaqueOperation {
			rep := classes.Find(i)
			if first[rep] != 0 {
				ids[i] = first[rep]
				continue
			}
			first[rep] = i
			count = 0
		} else if op == ast.ASTOpTrue {
			count = 0
		} else {
			for j, child := range a {
				a[j] = ids[child]
			}
		}
		g.terms[i] = groundApplication{operation: op, boolean: n.boolean, start: n.start, count: count}
	}
	for i, id := range g.roots {
		g.roots[i] = ids[id]
	}
	return g.booleanProblem()
}
