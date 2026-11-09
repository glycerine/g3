package smtcore

// Go randomizes map iteration, while the source's containers iterate the same
// way on every run. These helpers give search-affecting loops a stable order.

import (
	"slices"

	"github.com/glycerine/g3/pkg/ast"
)

func smtCompareHandles(a, b ast.ASTHandle) int {
	if a.Slot() != b.Slot() {
		if a.Slot() < b.Slot() {
			return -1
		}
		return 1
	}
	if a.Generation() != b.Generation() {
		if a.Generation() < b.Generation() {
			return -1
		}
		return 1
	}
	return 0
}

func smtSortedHandleSet(set map[ast.ASTHandle]struct{}) []ast.ASTHandle {
	keys := make([]ast.ASTHandle, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, smtCompareHandles)
	return keys
}

func smtSortedAUFKeys(nodes map[smtModelFinderAUFKey]*smtModelFinderAUFNode) []smtModelFinderAUFKey {
	keys := make([]smtModelFinderAUFKey, 0, len(nodes))
	for key := range nodes {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b smtModelFinderAUFKey) int {
		if order := smtCompareHandles(a.owner, b.owner); order != 0 {
			return order
		}
		if a.index != b.index {
			if a.index < b.index {
				return -1
			}
			return 1
		}
		return 0
	})
	return keys
}

func smtSortedEnodeKeys(values map[*SMTEnode]ast.ASTHandle) []*SMTEnode {
	keys := make([]*SMTEnode, 0, len(values))
	for key := range values {
		if key != nil {
			keys = append(keys, key)
		}
	}
	slices.SortFunc(keys, func(a, b *SMTEnode) int { return smtCompareHandles(a.owner, b.owner) })
	return keys
}
