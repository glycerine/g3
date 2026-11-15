package smtcore

// Source: src/smt/smt_model_finder.cpp f_var_plus_offset.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// collectOffset is process_u_app's is_var_plus_ground branch: it reports
// whether the argument matched (x + k, not inverted), whether or not
// insert_qinfo kept the f_var_plus_offset qinfo.
func (g *smtModelFinderAUFGraph) collectOffset(quantifier, function ast.ASTHandle, position uint32, argument ast.ASTHandle) (bool, error) {
	m := g.finder.manager
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	inverse, variable, matched, err := g.finder.macroUtil.IsVarPlusGround(argument, ref)
	if err != nil || !matched || inverse {
		return false, errors.Join(err, m.DelRef(&ref))
	}
	offset, err := m.RefGet(ref)
	if err != nil {
		return false, errors.Join(err, m.DelRef(&ref))
	}
	index, err := m.GetVarIndex(variable)
	if err != nil {
		return false, errors.Join(err, m.DelRef(&ref))
	}
	inserted, err := g.insertFVar(smtModelFinderFVar{
		quantifier: quantifier, function: function, position: position, variable: index, offset: offset,
	})
	if err != nil {
		return false, errors.Join(err, m.DelRef(&ref))
	}
	if !inserted {
		// insert_qinfo deleted the duplicate qinfo and its offset.
		return true, m.DelRef(&ref)
	}
	g.refs = append(g.refs, ref)
	return true, nil
}

// shiftedRef is n_k of copy_instances: arith_rw/bv_rw.mk_add or mk_sub(n,
// k), one poly_rewriter step, not rewritten further. The caller owns it.
func (g *smtModelFinderAUFGraph) shiftedRef(arith *rewriter.ArithRewriter, bv *rewriter.BVRewriter, term, offset ast.ASTHandle, plus bool) (ast.ASTRef, error) {
	m := g.finder.manager
	sort, err := m.GetSort(term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	isBV, err := m.IsBitVectorSort(sort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if isBV {
		if bv == nil {
			return ast.ASTRef{}, ast.ErrASTHandle
		}
		if plus {
			return bv.MkAdd2(term, offset)
		}
		return bv.MkSub2(term, offset)
	}
	if arith == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	if plus {
		return arith.MkAdd2(term, offset)
	}
	return arith.MkSub2(term, offset)
}

// insertShifted is one copy_instances step: to->insert(n_k, generation).
func (g *smtModelFinderAUFGraph) insertShifted(arith *rewriter.ArithRewriter, bv *rewriter.BVRewriter, to *smtModelFinderAUFNode, term, offset ast.ASTHandle, generation uint32, plus bool) (result error) {
	m := g.finder.manager
	ref, err := g.shiftedRef(arith, bv, term, offset, plus)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	shifted, err := m.RefGet(ref)
	if err != nil {
		return err
	}
	_, err = to.insert(shifted, generation)
	return err
}

// localArithRewriter is a local `arith_rewriter arith_rw(m)`.
func (g *smtModelFinderAUFGraph) localArithRewriter() (*rewriter.ArithRewriter, error) {
	c, gl := g.localRewriterServices()
	return rewriter.NewArithRewriter(g.finder.manager, c, gl, nil)
}

func (g *smtModelFinderAUFGraph) localRewriterServices() (*uparams.ParamsContext, *uparams.GlobalParamsService) {
	st := g.finder.state
	c, gl := st.paramsContext, st.globals
	if c == nil || gl == nil {
		// Standalone states borrow the quantifier rewriter's services.
		if q := st.quantifierManager; q != nil && q.rewriter != nil {
			c, gl = q.rewriter.ParamsServices()
		}
	}
	return c, gl
}

// localBVRewriter is a local `bv_rewriter bv_rw(m)`.
func (g *smtModelFinderAUFGraph) localBVRewriter() (*rewriter.BVRewriter, error) {
	c, gl := g.localRewriterServices()
	if gl == nil {
		return nil, uparams.ErrParamsOwner
	}
	legacy, err := gl.Manager()
	if err != nil {
		return nil, err
	}
	return rewriter.NewBVRewriter(g.finder.manager, c, legacy, nil)
}

// localRewriters is copy_instances' and populate_inst_sets' pair of local
// rewriters (arith_rewriter arith_rw(m); bv_rewriter bv_rw(m);).
func (g *smtModelFinderAUFGraph) localRewriters() (*rewriter.ArithRewriter, *rewriter.BVRewriter, func() error, error) {
	arith, err := g.localArithRewriter()
	if err != nil {
		return nil, nil, nil, err
	}
	bv, err := g.localBVRewriter()
	if err != nil {
		return nil, nil, nil, errors.Join(err, arith.Close())
	}
	return arith, bv, func() error { return errors.Join(bv.Close(), arith.Close()) }, nil
}

// copyInstances is copy_instances<PLUS>(from, to, s), with its local
// arith_rewriter.
func (g *smtModelFinderAUFGraph) copyInstances(from, to *smtModelFinderAUFNode, offset ast.ASTHandle, plus bool) (result error) {
	arith, bv, release, err := g.localRewriters()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, release()) }()
	for _, entry := range from.set.entries() {
		if err := g.insertShifted(arith, bv, to, entry.Key, offset, uint32(entry.Num), plus); err != nil {
			return err
		}
	}
	return nil
}

// populateOffset is f_var_plus_offset::populate_inst_sets2.
func (g *smtModelFinderAUFGraph) populateOffset(info smtModelFinderFVar) error {
	if info.offset.IsNull() {
		return nil
	}
	argument := g.arguments[smtModelFinderAUFKey{info.function, info.position}].root()
	variable := g.uvars[smtModelFinderAUFKey{info.quantifier, info.variable}].root()
	if argument == variable {
		// Source uses only ground-context arguments for this case: no
		// finite fixed point exists for a nonzero offset within a class.
		return nil
	}
	// The second population pass enforces A_f_i-k subset S_j followed
	// by S_j+k subset A_f_i, preserving each source generation.
	if err := g.copyInstances(argument, variable, info.offset, false); err != nil {
		return err
	}
	if err := g.copyInstances(variable, argument, info.offset, true); err != nil {
		return err
	}
	return nil
}
