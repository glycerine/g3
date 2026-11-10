package smtcore

// Source: src/smt/smt_model_finder.cpp f_var_plus_offset.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (g *smtModelFinderAUFGraph) collectOffset(quantifier, function ast.ASTHandle, position uint32, argument ast.ASTHandle) error {
	m := g.finder.manager
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	inverse, variable, matched, err := g.finder.macroUtil.IsVarPlusGround(argument, ref)
	if err != nil || !matched || inverse {
		return errors.Join(err, m.DelRef(&ref))
	}
	offset, err := m.RefGet(ref)
	if err != nil {
		return errors.Join(err, m.DelRef(&ref))
	}
	index, err := m.GetVarIndex(variable)
	if err != nil {
		return errors.Join(err, m.DelRef(&ref))
	}
	g.refs = append(g.refs, ref)
	g.appendFVar(smtModelFinderFVar{
		quantifier: quantifier, function: function, position: position, variable: index, offset: offset,
	})
	return nil
}

// insertShifted is one copy_instances step: n_k is arith_rw.mk_add/mk_sub(n,
// m_offset) (not rewritten further) for arithmetic sorts; bit-vectors keep
// the bv rewriter path.
func (g *smtModelFinderAUFGraph) insertShifted(arith *rewriter.ArithRewriter, to *smtModelFinderAUFNode, term, offset ast.ASTHandle, generation uint32, plus bool) (result error) {
	m := g.finder.manager
	sort, err := m.GetSort(term)
	if err != nil {
		return err
	}
	isBV, err := m.IsBitVectorSort(sort)
	if err != nil {
		return err
	}
	if !isBV {
		var ref ast.ASTRef
		if plus {
			ref, err = arith.MkAdd2(term, offset)
		} else {
			ref, err = arith.MkSub2(term, offset)
		}
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
	var shifted ast.ASTHandle
	if plus {
		shifted, err = g.finder.macroUtil.MkAdd(term, offset)
	} else {
		shifted, err = g.finder.macroUtil.MkSub(term, offset)
	}
	if err != nil {
		return err
	}
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	manager := g.finder.state.quantifierManager
	if manager == nil || manager.rewriter == nil {
		return ast.ErrASTHandle
	}
	if err = manager.rewriter.Rewrite(shifted, ref); err != nil {
		return err
	}
	shifted, err = m.RefGet(ref)
	if err != nil {
		return err
	}
	_, err = to.insert(shifted, generation)
	return err
}

// localArithRewriter is a local `arith_rewriter arith_rw(m)`.
func (g *smtModelFinderAUFGraph) localArithRewriter() (*rewriter.ArithRewriter, error) {
	st := g.finder.state
	c, gl := st.paramsContext, st.globals
	if c == nil || gl == nil {
		// Standalone states borrow the quantifier rewriter's services.
		if q := st.quantifierManager; q != nil && q.rewriter != nil {
			c, gl = q.rewriter.ParamsServices()
		}
	}
	return rewriter.NewArithRewriter(g.finder.manager, c, gl, nil)
}

// copyInstances is copy_instances<PLUS>(from, to, s), with its local
// arith_rewriter.
func (g *smtModelFinderAUFGraph) copyInstances(from, to *smtModelFinderAUFNode, offset ast.ASTHandle, plus bool) (result error) {
	arith, err := g.localArithRewriter()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, arith.Close()) }()
	for _, entry := range from.set.entries() {
		if err := g.insertShifted(arith, to, entry.Key, offset, uint32(entry.Num), plus); err != nil {
			return err
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) populateOffsets(quantifier ast.ASTHandle) error {
	for _, info := range g.fvars {
		if info.quantifier != quantifier || info.offset.IsNull() {
			continue
		}
		argument := g.arguments[smtModelFinderAUFKey{info.function, info.position}].root()
		variable := g.uvars[smtModelFinderAUFKey{info.quantifier, info.variable}].root()
		if argument == variable {
			// Source uses only ground-context arguments for this case: no
			// finite fixed point exists for a nonzero offset within a class.
			continue
		}
		// The second population pass enforces A_f_i-k subset S_j followed
		// by S_j+k subset A_f_i, preserving each source generation.
		if err := g.copyInstances(argument, variable, info.offset, false); err != nil {
			return err
		}
		if err := g.copyInstances(variable, argument, info.offset, true); err != nil {
			return err
		}
	}
	return nil
}
