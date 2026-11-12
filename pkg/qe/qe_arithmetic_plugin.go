package qe

// Source: src/qe/qe_arith_plugin.cpp arith_plugin.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

type arithmeticPluginKey struct{ variable, formula ast.ASTHandle }

// ArithmeticPlugin is the retained linear-arithmetic QE plugin.  It owns all
// cached roots, bounds processors, branch results and emitted constraints.
type ArithmeticPlugin struct {
	self     *ArithmeticPlugin
	context  *SolverContext
	manager  *ast.ASTManager
	util     *ArithmeticQETermUtil
	replacer *astrewriter.ExprSafeReplace
	family   ast.FamilyID
	bounds   map[arithmeticPluginKey]*arithmeticPluginBounds
	branches map[ArithmeticBranchKey]*ArithmeticPluginBranch
	// boundsTable is m_bounds_cache's obj_pair_map layout (x, fml):
	// ~arith_plugin deallocates the bounds_procs in its table order.
	boundsTable *ast.ObjMap
	// branchOrder lists the cached branches in add_cache order.
	branchOrder []*ArithmeticPluginBranch
	// trail is m_trail: add_cache pushes x, fml, result and term; the
	// bounds cache pushes x and fml.
	trail           []ast.ASTRef
	constraints     []ArithmeticConstraint
	pendingFresh    []ast.ASTRef
	normalizedAtoms []ast.ASTRef
	// arith_qe_util's m_zero_i, m_one_i, m_minus_one_i, m_zero_r, m_one_r.
	numerals []ast.ASTRef
	closed   bool
}

func NewArithmeticPlugin(context *SolverContext, config Config) (*ArithmeticPlugin, error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	m, err := context.Manager()
	if err != nil {
		return nil, err
	}
	u, err := NewArithmeticQETermUtil(m, config)
	if err != nil {
		return nil, err
	}
	r, err := astrewriter.NewExprSafeReplace(m)
	if err != nil {
		_ = u.Close()
		return nil, err
	}
	p := &ArithmeticPlugin{context: context, manager: m, util: u, replacer: r,
		family: u.family, bounds: make(map[arithmeticPluginKey]*arithmeticPluginBounds),
		branches: make(map[ArithmeticBranchKey]*ArithmeticPluginBranch), boundsTable: ast.NewObjMap(m)}
	p.self = p
	for _, n := range []struct {
		value   int64
		integer bool
	}{{0, true}, {1, true}, {-1, true}, {0, false}, {1, false}} {
		h, e := m.ArithmeticNumeralRat(big.NewRat(n.value, 1), n.integer)
		var ref ast.ASTRef
		if e == nil {
			ref, e = m.NewRef(h)
		}
		if e != nil {
			return nil, errors.Join(e, p.Close())
		}
		p.numerals = append(p.numerals, ref)
	}
	return p, nil
}

func (p *ArithmeticPlugin) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	if p.context == nil || p.manager == nil || p.util == nil || p.replacer == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (p *ArithmeticPlugin) FamilyID() ast.FamilyID {
	if p == nil {
		return ast.NullFamilyID
	}
	return p.family
}

func (p *ArithmeticPlugin) Close() (result error) {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	// ~arith_plugin: dealloc each bounds_proc in m_bounds_cache order.
	for _, row := range p.boundsTable.Entries() {
		key := arithmeticPluginKey{row.Key, row.Value[0]}
		if entry := p.bounds[key]; entry != nil {
			result = errors.Join(result, entry.close(p.manager))
			delete(p.bounds, key)
		}
	}
	for key, entry := range p.bounds {
		// Unreachable: every entry is in boundsTable.
		result = errors.Join(result, entry.close(p.manager))
		delete(p.bounds, key)
	}
	p.boundsTable = ast.NewObjMap(p.manager)
	// The cached branches' fresh-variable copies (branch_formula keeps
	// them as a ptr_vector), in add_cache order.
	for _, branch := range p.branchOrder {
		result = errors.Join(result, branch.close(p.manager))
	}
	p.branchOrder = nil
	clear(p.branches)
	// m_trail is destroyed forward.
	for i := range p.trail {
		result = errors.Join(result, p.manager.DelRef(&p.trail[i]))
	}
	p.trail = nil
	for i := len(p.normalizedAtoms) - 1; i >= 0; i-- {
		result = errors.Join(result, p.manager.DelRef(&p.normalizedAtoms[i]))
	}
	p.normalizedAtoms = nil
	for i := len(p.numerals) - 1; i >= 0; i-- {
		result = errors.Join(result, p.manager.DelRef(&p.numerals[i]))
	}
	p.numerals = nil
	result = errors.Join(result, p.clearConstraints(), p.clearPendingFresh(), p.replacer.Close(), p.util.Close())
	if result == nil {
		p.closed = true
		p.context = nil
		p.manager = nil
		p.replacer = nil
		p.util = nil
	}
	return result
}
