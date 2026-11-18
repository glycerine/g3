package arith

// Source: src/tactic/arith/degree_shift_tactic.cpp, prepare_substitution.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func degreeShiftPublishModel(plan *degreeShiftPlan, services model.ModelEvaluatorConfig, out *tactic.TacticResult) (err error) {
	if plan == nil || plan.manager == nil || out == nil {
		return ast.ErrASTHandle
	}
	if len(plan.entries) == 0 {
		return nil
	}
	m := plan.manager
	filter, err := tactic.NewFilterModel(m)
	if err != nil {
		return err
	}
	if err = filter.IncRef(); err != nil {
		return errors.Join(err, filter.Close())
	}
	defer func() { err = errors.Join(err, filter.DecRef()) }()
	extension, err := tactic.NewExtensionModel(m, services)
	if err != nil {
		return err
	}
	if err = extension.IncRef(); err != nil {
		return errors.Join(err, extension.Close())
	}
	defer func() { err = errors.Join(err, extension.DecRef()) }()
	for _, entry := range plan.entries {
		fresh, e := m.RefGet(entry.fresh)
		if e != nil {
			return e
		}
		freshDecl, e := m.GetDecl(fresh)
		if e != nil {
			return e
		}
		if e = filter.Insert(freshDecl); e != nil {
			return e
		}
		original, e := m.RefGet(entry.original)
		if e != nil {
			return e
		}
		originalDecl, e := m.GetDecl(original)
		if e != nil {
			return e
		}
		definition := entry.definition
		if definition == (ast.ASTRef{}) {
			definition, e = degreeShiftModelDefinition(m, fresh, &entry.degree)
			if e != nil {
				return e
			}
			defer func(r ast.ASTRef) { err = errors.Join(err, m.DelRef(&r)) }(definition)
		}
		value, e := m.RefGet(definition)
		if e != nil {
			return e
		}
		if e = extension.Insert(originalDecl, value); e != nil {
			return e
		}
	}
	combined, err := tactic.ConcatModels(m.Budget(), filter, extension)
	if err != nil {
		return err
	}
	if err = out.SetModelConverter(combined); err != nil {
		return errors.Join(err, combined.Close())
	}
	return nil
}
