package smtcore

// hintSet is the hint solver's obj_hashtable (func_decl_set, quantifier_set,
// expr_set): membership plus the source's table iteration order.

import "github.com/glycerine/g3/pkg/ast"

type hintSet struct{ table *ast.ObjMap }

func newHintSet(m *ast.ASTManager) *hintSet { return &hintSet{table: ast.NewObjMap(m)} }

func (h *hintSet) add(x ast.ASTHandle) error { return h.table.Insert(x, [2]ast.ASTHandle{}) }

func (h *hintSet) has(x ast.ASTHandle) bool {
	if h == nil {
		return false
	}
	found, err := h.table.Contains(x)
	return err == nil && found
}

func (h *hintSet) erase(x ast.ASTHandle) error { return h.table.Erase(x) }

func (h *hintSet) size() int {
	if h == nil {
		return 0
	}
	return h.table.Len()
}

// keys lists the members in iteration order.
func (h *hintSet) keys() []ast.ASTHandle {
	if h == nil {
		return nil
	}
	entries := h.table.Entries()
	result := make([]ast.ASTHandle, len(entries))
	for i, entry := range entries {
		result[i] = entry.Key
	}
	return result
}

func (h *hintSet) reset() { h.table.Reset() }
