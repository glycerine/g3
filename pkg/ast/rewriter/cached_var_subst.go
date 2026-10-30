package rewriter

// Source: src/ast/rewriter/cached_var_subst.h:24-53;
// src/ast/rewriter/cached_var_subst.cpp:33-37.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type cachedVarSubstitutionKey struct {
	quantifier ast.ASTHandle
	bindings   []ast.ASTHandle
}

type cachedVarSubstitutionEntry struct {
	key    cachedVarSubstitutionKey
	owned  []ast.ASTRef
	result ast.ASTRef
}

// CachedVarSubstitution is the retained concrete cached_var_subst instance used
// by quantifier queues. Begin/SetBinding model the source's first operator(),
// and Instantiate models its nullary operator().
type CachedVarSubstitution struct {
	self         *CachedVarSubstitution
	manager      *ast.ASTManager
	substitution *VarSubstitution
	entries      map[ast.ASTHandle]map[uint32][]*cachedVarSubstitutionEntry
	ordered      []*cachedVarSubstitutionEntry
	current      *cachedVarSubstitutionKey
	header       uint64
	closed       bool
}

func NewCachedVarSubstitution(m *ast.ASTManager) (*CachedVarSubstitution, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(CachedVarSubstitution{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	c := &CachedVarSubstitution{manager: m, header: n, entries: make(map[ast.ASTHandle]map[uint32][]*cachedVarSubstitutionEntry)}
	c.self = c
	var err error
	c.substitution, err = NewVarSubstitution(m, true)
	return c, err
}

func (c *CachedVarSubstitution) open() error {
	if c == nil || c.self != c || c.closed || c.manager == nil || c.substitution == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (c *CachedVarSubstitution) Size() uint32 {
	if c == nil {
		return 0
	}
	return uint32(len(c.ordered))
}
