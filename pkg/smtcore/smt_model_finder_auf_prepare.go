package smtcore

// Source: src/smt/smt_model_finder.cpp cleanup_instantiation_sets,
// add_elem_to_empty_inst_sets, mk_sorts_finite and add_mono_exceptions.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// eval is auf_solver::eval(n, true).
func (g *smtModelFinderAUFGraph) eval(term ast.ASTHandle) (ast.ASTHandle, error) {
	return g.evalCached(term, true)
}

// evalCached is auf_solver::eval(n, model_completion): results (a null
// handle when evaluation fails) are memoized in m_eval_cache and kept
// alive by m_eval_cache_range until reset_eval_cache.
func (g *smtModelFinderAUFGraph) evalCached(term ast.ASTHandle, completion bool) (ast.ASTHandle, error) {
	if g.model == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	index := 0
	if completion {
		index = 1
	}
	if g.evalCache[index] == nil {
		g.evalCache[index] = make(map[ast.ASTHandle]ast.ASTHandle)
	}
	if value, found := g.evalCache[index][term]; found {
		return value, nil
	}
	m := g.finder.manager
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ok, err := g.model.Eval(term, ref, completion)
	if err != nil {
		return ast.ASTHandle{}, errors.Join(err, m.DelRef(&ref))
	}
	var value ast.ASTHandle
	if ok {
		if value, err = m.RefGet(ref); err != nil {
			return ast.ASTHandle{}, errors.Join(err, m.DelRef(&ref))
		}
	} else if err = m.DelRef(&ref); err != nil {
		return ast.ASTHandle{}, err
	}
	g.evalCache[index][term] = value
	g.evalRange = append(g.evalRange, ref)
	return value, nil
}

// resetEvalCache is auf_solver::reset_eval_cache: m_eval_cache_range is
// released front to back.
func (g *smtModelFinderAUFGraph) resetEvalCache() (result error) {
	g.evalCache[0], g.evalCache[1] = nil, nil
	for i := range g.evalRange {
		result = errors.Join(result, g.finder.manager.DelRef(&g.evalRange[i]))
	}
	g.evalRange = nil
	return result
}

func (g *smtModelFinderAUFGraph) cleanInstantiationSets() error {
	for _, node := range g.nodes {
		if node.parent != nil {
			continue
		}
		for _, entry := range node.set.entries() {
			term := entry.Key
			value, err := g.eval(term)
			if err != nil {
				return err
			}
			isValue, err := g.finder.manager.IsValue(value)
			if err != nil {
				return err
			}
			if !isValue {
				if err := node.set.Remove(term); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) fillEmptyInstantiationSets() error {
	m := g.finder.manager
	bySort := make(map[ast.ASTHandle]ast.ASTHandle)
	var fresh []*smtModelFinderAUFNode
	for _, node := range g.nodes {
		if node.parent != nil {
			continue
		}
		if node.set.size() != 0 {
			bySort[node.sort] = node.set.entries()[0].Key
			continue
		}
		interpreted, err := m.IsFullyInterpreted(node.sort)
		if err != nil {
			return err
		}
		if !interpreted {
			fresh = append(fresh, node)
			continue
		}
		value, err := g.model.SomeValue(node.sort)
		if err != nil {
			return err
		}
		if _, err := node.insert(value, 0); err != nil {
			return err
		}
	}
	for _, node := range fresh {
		term := bySort[node.sort]
		if term.IsNull() {
			var err error
			term, err = m.MkFreshConst("elem", node.sort)
			if err != nil {
				return err
			}
			bySort[node.sort] = term
		}
		// Insert pins the term. Private model values must never escape
		// into instantiations for sorts with an arbitrary interpretation.
		if _, err := node.insert(term, 0); err != nil {
			return err
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) freezeUniverses() error {
	for _, node := range g.nodes {
		if node.parent != nil {
			continue
		}
		uninterpreted, err := g.finder.manager.IsUninterpretedSort(node.sort)
		if err != nil {
			return err
		}
		if !uninterpreted {
			continue
		}
		finite, err := g.model.IsFinite(node.sort)
		if err != nil {
			return err
		}
		if !finite {
			if err := g.model.FreezeUniverse(node.sort); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) addMonotoneExceptions() error {
	for _, node := range g.nodes {
		if node.parent != nil || !node.mono {
			continue
		}
		if err := g.addNodeMonotoneExceptions(node); err != nil {
			return err
		}
	}
	return nil
}

// addNodeMonotoneExceptions is auf_solver::add_mono_exceptions(n), run by
// mk_mono_proj on each mono root as the projections are built.
func (g *smtModelFinderAUFGraph) addNodeMonotoneExceptions(node *smtModelFinderAUFNode) error {
	m := g.finder.manager
	bv, err := m.IsBitVectorSort(node.sort)
	if err != nil {
		return err
	}
	if !bv {
		family, err := m.GetFamilyID(node.sort)
		if err != nil {
			return err
		}
		arithmetic, err := m.ArithmeticFamilyID()
		if err != nil {
			return err
		}
		op, err := m.GetDeclKind(node.sort)
		if err != nil {
			return err
		}
		// Integer sort kind is one. Reals deliberately receive no
		// neighboring exceptions in the source algorithm.
		if family != arithmetic || op != 1 {
			return nil
		}
	}
	one, err := m.MkDatalogNumeral(1, node.sort)
	if err != nil {
		return err
	}
	if !bv {
		// arith_rw.mk_sub(e, one, e_minus_1); arith_rw.mk_add(e, one,
		// e_plus_1); insert e_plus_1, then e_minus_1.
		return g.addIntegerMonotoneExceptions(node, one)
	}
	for _, exception := range node.exceptions {
		if err := g.insertShifted(nil, node, exception, one, 0, true); err != nil {
			return err
		}
		if err := g.insertShifted(nil, node, exception, one, 0, false); err != nil {
			return err
		}
	}
	return nil
}

func (g *smtModelFinderAUFGraph) prepareInstantiationSets() error {
	if err := g.cleanInstantiationSets(); err != nil {
		return err
	}
	if err := g.fillEmptyInstantiationSets(); err != nil {
		return err
	}
	if err := g.freezeUniverses(); err != nil {
		return err
	}
	return g.addMonotoneExceptions()
}

func (g *smtModelFinderAUFGraph) addIntegerMonotoneExceptions(node *smtModelFinderAUFNode, one ast.ASTHandle) (result error) {
	m := g.finder.manager
	arith, err := g.localArithRewriter()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, arith.Close()) }()
	// expr_ref e_minus_1, e_plus_1 live across iterations.
	var minus, plus ast.ASTRef
	release := func(r *ast.ASTRef) error {
		if *r == (ast.ASTRef{}) {
			return nil
		}
		return m.DelRef(r)
	}
	defer func() { result = errors.Join(result, release(&plus), release(&minus)) }()
	for _, e := range node.exceptions {
		next, err := arith.MkSub2(e, one)
		if err != nil {
			return err
		}
		if err = release(&minus); err != nil {
			return errors.Join(err, m.DelRef(&next))
		}
		minus = next
		if next, err = arith.MkAdd2(e, one); err != nil {
			return err
		}
		if err = release(&plus); err != nil {
			return errors.Join(err, m.DelRef(&next))
		}
		plus = next
		for _, ref := range []ast.ASTRef{plus, minus} {
			h, err := m.RefGet(ref)
			if err != nil {
				return err
			}
			if _, err = node.insert(h, 0); err != nil {
				return err
			}
		}
	}
	return nil
}
