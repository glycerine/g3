package nlsatsmt

// Source: src/ast/rewriter/rewriter.h rewriter_core::must_cache and the
// act_cache references kept by rewriter_tpl, as used by nl_purify's rw.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// mustCache is rewriter_core::must_cache for an application with count
// arguments: t->get_ref_count() > 1 && t != m_root && num_args > 0.
func mustCache(m *ast.ASTManager, term, root ast.ASTHandle, count uint32) (bool, error) {
	if term == root || count == 0 {
		return false, nil
	}
	rc, err := m.GetRefCount(term)
	return rc > 1, err
}

// holdCacheEntry takes the references the rewriter cache keeps on a cached
// key and its value.
func holdCacheEntry(m *ast.ASTManager, refs *[]ast.ASTRef, key, value ast.ASTHandle) error {
	for _, h := range []ast.ASTHandle{key, value} {
		ref, err := m.NewRef(h)
		if err != nil {
			return err
		}
		*refs = append(*refs, ref)
	}
	return nil
}

// releaseCache is the rewriter's reset(): it empties the cache and drops its
// references.
func releaseCache(m *ast.ASTManager, refs *[]ast.ASTRef, memo map[ast.ASTHandle]ast.ASTHandle) (result error) {
	for i := len(*refs) - 1; i >= 0; i-- {
		result = errors.Join(result, m.DelRef(&(*refs)[i]))
	}
	*refs = nil
	for k := range memo {
		delete(memo, k)
	}
	return result
}
