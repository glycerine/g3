package ackermannization

// Source: src/ackermannization/ackr_helper.cpp/.h.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

// NChoose2Bound preserves the source's bit-16 guard and unsigned arithmetic,
// including its nonmonotone behavior outside the normally reached range.
func NChoose2Bound(n uint32) float64 {
	if n&(1<<16) != 0 {
		return math.Inf(1)
	}
	if n&1 != 0 {
		return float64(n * (n >> 1))
	}
	return float64((n >> 1) * (n - 1))
}
func ShouldAckermannize(m *ast.ASTManager, term ast.ASTHandle) (bool, error) {
	family, err := m.GetFamilyID(term)
	if err != nil {
		return false, err
	}
	if family == ast.NullFamilyID {
		return true, nil
	}
	d, err := m.GetDecl(term)
	if err != nil {
		return false, err
	}
	return m.IsConsideredUninterpreted(d)
}

type termGroups map[ast.ASTHandle]map[ast.ASTHandle]bool

// declarations and sortedTerms give the groups a stable order, since the order
// here picks abstraction names and the order of Ackermann lemmas.
func (g termGroups) declarations() []ast.ASTHandle {
	keys := make([]ast.ASTHandle, 0, len(g))
	for key := range g {
		keys = append(keys, key)
	}
	ast.SortHandles(keys)
	return keys
}

func sortedTerms(terms map[ast.ASTHandle]bool) []ast.ASTHandle {
	keys := make([]ast.ASTHandle, 0, len(terms))
	for key := range terms {
		keys = append(keys, key)
	}
	ast.SortHandles(keys)
	return keys
}

func (g termGroups) bound() float64 {
	var result float64
	for _, terms := range g {
		result += NChoose2Bound(uint32(len(terms)))
	}
	return result
}
func (g termGroups) add(m *ast.ASTManager, term ast.ASTHandle) error {
	n, err := m.GetNumArgs(term)
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	yes, err := ShouldAckermannize(m, term)
	if err != nil || !yes {
		return err
	}
	d, err := m.GetDecl(term)
	if err != nil {
		return err
	}
	if g[d] == nil {
		g[d] = make(map[ast.ASTHandle]bool)
	}
	g[d][term] = true
	return nil
}
