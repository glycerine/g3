package order

import "github.com/glycerine/g3/pkg/ast"

// RefutesComparisonCycle checks a small conjunction of unsigned bit-vector
// comparisons for a strict cycle. A strict comparison is a weak ordering
// plus disequality. Terms can be arbitrary expressions: only identical
// handles denote the same vertex. minWidth selects which conflicts are worth a private check.
func RefutesComparisonCycle(m *ast.ASTManager, goal ast.ASTHandle, minWidth uint32) bool {
	g := comparisonGraph{remaining: 32}
	if !g.collect(m, goal, true) || !g.hasOrder {
		return false
	}
	// Close <= transitively. Mutual reachability forces equality, which
	// contradicts any explicit or strict-comparison disequality.
	for k := 0; k < g.count; k++ {
		for i := 0; i < g.count; i++ {
			if g.le[i]&(1<<k) != 0 {
				g.le[i] |= g.le[k]
			}
		}
	}
	for i := 0; i < g.count; i++ {
		for j := 0; j < g.count; j++ {
			if g.unequal[i]&g.le[i]&(1<<j) != 0 && g.le[j]&(1<<i) != 0 {
				if minWidth == 0 {
					return true
				}
				// Equality edges can also occur in a disconnected component
				// of another sort. Only a wide bit-vector conflict qualifies.
				if bv, err := m.IsBitVector(g.terms[i]); err == nil && bv {
					width, err := m.BitVectorSize(g.terms[i])
					if err == nil && width >= minWidth {
						return true
					}
				}
			}
		}
	}
	return false
}

type comparisonGraph struct {
	terms       [8]ast.ASTHandle
	le, unequal [8]uint8
	count       int
	remaining   int
	hasOrder    bool
}

func (g *comparisonGraph) vertex(term ast.ASTHandle) int {
	for i := 0; i < g.count; i++ {
		if g.terms[i] == term {
			return i
		}
	}
	if g.count == len(g.terms) {
		return -1
	}
	i := g.count
	g.count++
	g.terms[i] = term
	g.le[i] = 1 << i
	return i
}

func (g *comparisonGraph) collect(m *ast.ASTManager, term ast.ASTHandle, positive bool) bool {
	if g.remaining == 0 {
		return false
	}
	g.remaining--
	h, err := m.NodeHeader(term)
	if err != nil || h.Kind != ast.ASTApp {
		return false
	}
	basic := h.Family == m.BasicFamilyID()
	if basic {
		switch {
		case h.DeclKind == ast.ASTOpNot && h.NumArgs == 1:
			child, err := m.GetArg(term, 0)
			return err == nil && g.collect(m, child, !positive)
		case positive && h.DeclKind == ast.ASTOpAnd, !positive && h.DeclKind == ast.ASTOpOr:
			if h.NumArgs > uint32(g.remaining) {
				return false
			}
			for i := uint32(0); i < h.NumArgs; i++ {
				child, err := m.GetArg(term, i)
				if err != nil || !g.collect(m, child, positive) {
					return false
				}
			}
			return true
		case !positive && h.DeclKind == ast.ASTOpImplies && h.NumArgs == 2:
			left, err := m.GetArg(term, 0)
			if err != nil || !g.collect(m, left, true) {
				return false
			}
			right, err := m.GetArg(term, 1)
			return err == nil && g.collect(m, right, false)
		case h.DeclKind != ast.ASTOpEq:
			return false
		}
	}
	return g.addComparison(m, term, h, positive)
}

func (g *comparisonGraph) addComparison(m *ast.ASTManager, term ast.ASTHandle, h ast.ASTNodeHeader, positive bool) bool {
	if h.NumArgs != 2 {
		return false
	}
	equality := h.Family == m.BasicFamilyID()
	if !equality {
		switch h.DeclKind {
		case bvULT, bvUGT, bvULE, bvUGE:
			ok, err := m.IsBitVectorOperation(term, h.DeclKind)
			if err != nil || !ok {
				return false
			}
		default:
			return false
		}
	}
	left, err := m.GetArg(term, 0)
	if err != nil {
		return false
	}
	right, err := m.GetArg(term, 1)
	if err != nil {
		return false
	}
	i, j := g.vertex(left), g.vertex(right)
	if i < 0 || j < 0 {
		return false
	}
	if equality {
		if positive {
			g.le[i] |= 1 << j
			g.le[j] |= 1 << i
		} else {
			g.unequal[i] |= 1 << j
		}
		return true
	}
	g.hasOrder = true
	reverse := h.DeclKind == bvUGT || h.DeclKind == bvUGE
	strict := h.DeclKind == bvULT || h.DeclKind == bvUGT
	// Negation reverses the order and exchanges strict with non-strict.
	if !positive {
		reverse = !reverse
		strict = !strict
	}
	if reverse {
		i, j = j, i
	}
	g.le[i] |= 1 << j
	if strict {
		g.unequal[i] |= 1 << j
	}
	return true
}
