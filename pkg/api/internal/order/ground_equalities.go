package order

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/backtrack"
)

// ProjectGroundEqualities builds a Boolean relaxation of asserted conjuncts.
// Ground facts and selected ground instances of positive universal premises
// can identify applications of the same declaration. Only UNSAT is conclusive.
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
	var nonGround []ast.ASTHandle
	for _, h := range conjuncts {
		if err := g.compiler.charge(1); err != nil {
			return nil, err
		}
		ground, err := g.manager.IsGround(h)
		if err != nil {
			return nil, err
		}
		if !ground {
			nonGround = append(nonGround, h)
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

	if err := g.addObservedInstances(nonGround); err != nil {
		return nil, err
	}
	classes := groundClasses{trueID: uint32(len(g.input)), falseID: uint32(len(g.input)) + 1}
	if err := g.mergeRequiredEqualities(&classes); err != nil {
		return nil, err
	}
	if err := g.mergeCongruentTerms(&classes); err != nil {
		return nil, err
	}
	if classes.Find(classes.trueID) == classes.Find(classes.falseID) {
		return &BooleanProblem{Clauses: []BooleanClause{{}}, Work: uint32(g.compiler.work)}, nil
	}
	return g.encodeGroundClasses(&classes)
}

// Distinct synthetic IDs represent the Boolean constants even when neither
// occurs in the input. A merge of these two classes establishes a conflict.
type groundClasses struct {
	backtrack.BasicUnionFind
	trueID, falseID uint32
	different       [][2]uint32
	merges          int
}

func (c *groundClasses) merge(a, b uint32) {
	if c.Find(a) != c.Find(b) {
		c.Merge(a, b)
		c.merges++
	}
}

// The input DAG is in postorder. By the time we visit a node backwards,
// all parents have contributed the polarities they force on it.
func (g *groundProjection) mergeRequiredEqualities(classes *groundClasses) error {
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
		if sign&positive != 0 || n.operation == ast.ASTOpTrue {
			classes.merge(uint32(i), classes.trueID)
		}
		if sign&negative != 0 || n.operation == ast.ASTOpFalse {
			classes.merge(uint32(i), classes.falseID)
		}
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
				classes.merge(a[0], a[1])
			} else if sign&negative != 0 {
				classes.different = append(classes.different, [2]uint32{a[0], a[1]})
			}
		}
	}
	return nil
}

type groundSignature struct {
	symbol uint32
	args   [4]uint32
}

// Merging arguments can determine a Boolean condition, select an ITE branch
// and enable another congruence. Stop when no class changes or after six passes.
func (g *groundProjection) mergeCongruentTerms(classes *groundClasses) error {
	known := make(map[groundSignature]uint32)
	different := make(map[[2]uint32]bool, len(classes.different))
	for range 6 {
		before := classes.merges
		clear(known)
		clear(different)
		for _, pair := range classes.different {
			if err := g.compiler.charge(1); err != nil {
				return err
			}
			a, b := classes.Find(pair[0]), classes.Find(pair[1])
			different[[2]uint32{min(a, b), max(a, b)}] = true
		}
		for i := uint32(1); i < uint32(len(g.input)); i++ {
			n := &g.input[i]
			if err := g.compiler.charge(1 + int(n.count)); err != nil {
				return err
			}
			if n.count == 0 || n.count > 4 {
				continue
			}
			k := groundSignature{symbol: n.symbol}
			for j, child := range g.children[n.start : n.start+n.count] {
				k.args[j] = classes.Find(child)
			}
			if n.operation == ast.ASTOpEq && k.args[0] > k.args[1] {
				k.args[0], k.args[1] = k.args[1], k.args[0]
			}
			if err := g.reduceGroundClass(classes, i, k.args[:n.count], different); err != nil {
				return err
			}
			if old := known[k]; old != 0 {
				classes.merge(i, old)
			} else {
				known[k] = i
			}
		}
		if before == classes.merges || classes.Find(classes.trueID) == classes.Find(classes.falseID) {
			break
		}
	}
	return nil
}

func (g *groundProjection) reduceGroundClass(c *groundClasses, id uint32, args []uint32, different map[[2]uint32]bool) error {
	n := &g.input[id]
	t, f := c.Find(c.trueID), c.Find(c.falseID)
	switch n.operation {
	case ast.ASTOpEq:
		a, b := args[0], args[1]
		if a == b {
			c.merge(id, t)
		} else if different[[2]uint32{a, b}] || a == min(t, f) && b == max(t, f) {
			c.merge(id, f)
		}
	case ast.ASTOpNot:
		if args[0] == t {
			c.merge(id, f)
		} else if args[0] == f {
			c.merge(id, t)
		}
	case ast.ASTOpAnd, ast.ASTOpOr:
		neutral, absorbing := t, f
		if n.operation == ast.ASTOpOr {
			neutral, absorbing = f, t
		}
		allNeutral := true
		for _, a := range args {
			if a == absorbing {
				c.merge(id, absorbing)
				return nil
			}
			allNeutral = allNeutral && a == neutral
		}
		if allNeutral {
			c.merge(id, neutral)
		}
	}
	if len(args) == 3 && (args[0] == t || args[0] == f || args[1] == args[2]) {
		// Data-valued conditionals are opaque to the Boolean compiler.
		h, err := g.manager.NodeHeader(n.expr)
		if err != nil {
			return err
		}
		if h.Family == g.manager.BasicFamilyID() && h.DeclKind == ast.ASTOpIte {
			branch := args[1]
			if args[0] == f {
				branch = args[2]
			}
			c.merge(id, branch)
		}
	}
	return nil
}

// Keep Boolean gates in source order. Opaque Boolean applications in one
// class share a single SAT variable. Distinct classes can still be equal.
func (g *groundProjection) encodeGroundClasses(classes *groundClasses) (*BooleanProblem, error) {
	t, f := classes.Find(classes.trueID), classes.Find(classes.falseID)
	first := make([]uint32, len(g.input)+2)
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
		if op == ast.ASTOpEq && !g.input[a[0]].boolean {
			op = opaqueOperation
			if classes.Find(a[0]) == classes.Find(a[1]) {
				op = ast.ASTOpTrue
			}
		}
		if op == opaqueOperation && n.boolean {
			switch classes.Find(i) {
			case t:
				op = ast.ASTOpTrue
			case f:
				op = ast.ASTOpFalse
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
		} else if op == ast.ASTOpTrue || op == ast.ASTOpFalse {
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
