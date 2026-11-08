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

// termGroups is ackr_helper::fun2terms_map: an obj_map from each
// uninterpreted function to the obj_hashtable of its applications. Both are
// iterated in the source's hash-table order, which picks the abstraction
// names and the order of the Ackermann lemmas.
type termGroups struct {
	decls *ackrObjHashtable   // func_decl -> index into sets
	sets  []*ackrObjHashtable // app_set per func_decl
}

func newTermGroups() *termGroups { return &termGroups{decls: newAckrObjHashtable()} }

func (g *termGroups) size() int { return g.decls.size }

// termGroup is one fun2terms_map entry, its applications in app_set order.
type termGroup struct {
	decl  ast.ASTHandle
	terms []ast.ASTHandle
}

func (g *termGroups) groups() []termGroup {
	entries := g.decls.entries()
	out := make([]termGroup, len(entries))
	for i, entry := range entries {
		out[i] = termGroup{decl: entry.key, terms: g.sets[entry.value].keys()}
	}
	return out
}

// bound is ackr_helper::calculate_lemma_bound, summed in fun2terms order.
func (g *termGroups) bound() float64 {
	var result float64
	for _, entry := range g.decls.entries() {
		result += NChoose2Bound(uint32(g.sets[entry.value].size))
	}
	return result
}

func (g *termGroups) add(m *ast.ASTManager, term ast.ASTHandle) error {
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
	slot, inserted, err := g.decls.insert(m, d)
	if err != nil {
		return err
	}
	if inserted {
		g.decls.table[slot].value = len(g.sets)
		g.sets = append(g.sets, newAckrObjHashtable())
	}
	_, _, err = g.sets[g.decls.table[slot].value].insert(m, term)
	return err
}
