package smtcore

// Source: src/smt/smt_model_finder.cpp f_var macro-based population and
// quantifier_info::populate_macro_based_inst_sets.

import "github.com/glycerine/g3/pkg/ast"

// populateMacroInstantiationSets is
// quantifier_info::populate_macro_based_inst_sets: each f_var qinfo of the
// chosen head, in m_qinfo_vect order, fills uvar_inst_sets[m_var_j]
// (f_var_plus_offset is ignored in a macro), and each set then makes its
// inverse with the auf_solver as the evaluator.
func (f *SMTModelFinder) populateMacroInstantiationSets(quantifier ast.ASTHandle) error {
	chosen := f.chosen[quantifier]
	if chosen.IsNull() {
		return nil
	}
	if _, found := f.sets[quantifier]; found {
		return nil
	}
	info := f.qinfos[quantifier]
	if info == nil {
		return ast.ErrASTDomain
	}
	var sets []*smtModelFinderInstantiationSet
	f.sets[quantifier] = sets
	for _, ref := range info.order {
		if ref.kind != qinfoFVar {
			continue
		}
		fvar := info.fvars[ref.index]
		if fvar.function != chosen || !fvar.offset.IsNull() {
			continue
		}
		for uint32(len(sets)) <= fvar.variable {
			sets = append(sets, nil)
		}
		if sets[fvar.variable] == nil {
			set, err := newSMTModelFinderInstantiationSet(f.manager)
			if err != nil {
				return err
			}
			sets[fvar.variable] = set
			f.sets[quantifier] = sets
		}
		set := sets[fvar.variable]
		for _, application := range f.state.context.byDeclaration[chosen] {
			if application == nil || !f.state.isRelevant(application.owner) {
				continue
			}
			if fvar.position >= uint32(len(application.arguments)) {
				return ast.ErrASTDomain
			}
			argument := application.arguments[fvar.position]
			if _, err := set.Insert(argument.owner, argument.Generation()); err != nil {
				return err
			}
		}
	}
	f.sets[quantifier] = sets
	for _, set := range sets {
		if set != nil {
			if err := set.MakeInverse(f.aufSolver.eval); err != nil {
				return err
			}
		}
	}
	return nil
}
