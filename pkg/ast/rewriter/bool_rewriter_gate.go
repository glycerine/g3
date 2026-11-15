package rewriter

// Source: src/ast/rewriter/bool_rewriter.cpp:23-32 (bool_rewriter::updt_params);
// src/ast/rewriter/bit_blaster/bit_blaster.cpp:124-128 (bit_blaster::m_rw);
// src/ast/rewriter/bit_blaster/bit_blaster_rewriter.cpp:69-79
// (blaster::m_rewriter).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// BoolGateRewriter is a persistent bool_rewriter member owned by a gate layer
// such as bit_blaster_cfg's m_rw or blaster's m_rewriter. It carries the full
// bool_rewriter configuration and keeps m_local_ctx_cost across calls.
type BoolGateRewriter struct {
	rw boolRewriter
}

// NewBoolGateRewriter is bool_rewriter(m) with the bool_rewriter_params
// defaults. LoadGlobalParams then reads the rewriter module as
// updt_params(params_ref()) does.
func NewBoolGateRewriter(m *ast.ASTManager) (*BoolGateRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	return &BoolGateRewriter{rw: boolRewriter{manager: m, flat: true, localContextLimit: ^uint32(0), blastDistinctThreshold: ^uint32(0)}}, nil
}

// LoadGlobalParams is bool_rewriter::updt_params(params_ref()): every
// bool_rewriter parameter comes from the global rewriter module.
func (g *BoolGateRewriter) LoadGlobalParams(c *uparams.ParamsContext, globals *uparams.GlobalParamsService) (err error) {
	if g == nil {
		return ast.ErrASTHandle
	}
	if c == nil || globals == nil {
		return nil
	}
	source, err := newBoolRewriter(g.rw.manager, c, globals, nil)
	if source != nil {
		defer func() { err = errors.Join(err, source.Close()) }()
	}
	if err != nil {
		return err
	}
	g.rw.flat = source.flat
	g.rw.elimAnd = source.elimAnd
	g.rw.localContext = source.localContext
	g.rw.localContextLimit = source.localContextLimit
	g.rw.blastDistinct = source.blastDistinct
	g.rw.blastDistinctThreshold = source.blastDistinctThreshold
	g.rw.iteExtraRules = source.iteExtraRules
	return nil
}

// SetFlat is bool_rewriter::set_flat.
func (g *BoolGateRewriter) SetFlat(flat bool) { g.rw.flat = flat }

// SetElimAnd is bool_rewriter::set_elim_and.
func (g *BoolGateRewriter) SetElimAnd(elimAnd bool) { g.rw.elimAnd = elimAnd }

// Flat reports m_flat.
func (g *BoolGateRewriter) Flat() bool { return g.rw.flat }

// ElimAnd reports m_elim_and.
func (g *BoolGateRewriter) ElimAnd() bool { return g.rw.elimAnd }

// IteExtraRules reports m_ite_extra_rules.
func (g *BoolGateRewriter) IteExtraRules() bool { return g.rw.iteExtraRules }

// Simplify applies the bool_rewriter mk_* operation for operation.
func (g *BoolGateRewriter) Simplify(operation int32, args []ast.ASTHandle, result ast.ASTRef) error {
	if g == nil || g.rw.manager == nil {
		return ast.ErrASTHandle
	}
	return g.rw.simplifyGate(operation, args, result)
}
