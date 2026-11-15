package arith

// Source: src/tactic/arith/recover_01_tactic.cpp, model converter setup.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func (d *recover01Definitions) publishModel(services model.ModelEvaluatorConfig, out *tactic.TacticResult) (err error) {
	if d == nil || d.manager == nil || out == nil {
		return ast.ErrASTHandle
	}
	m := d.manager
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
	insert := func(variable ast.ASTHandle) error {
		var definition, proof ast.ASTHandle
		found, e := m.ExprSubstitutionFindProof(d.sub, variable, &definition, &proof)
		if e != nil {
			return e
		}
		if !found {
			return ast.ErrASTDomain
		}
		decl, e := m.GetDecl(variable)
		if e != nil {
			return e
		}
		return extension.Insert(decl, definition)
	}
	for _, entry := range d.booleans {
		fresh, e := m.RefGet(entry.fresh)
		if e != nil {
			return e
		}
		decl, e := m.GetDecl(fresh)
		if e != nil {
			return e
		}
		if e = filter.Insert(decl); e != nil {
			return e
		}
	}
	// mc1 interleaves each new atom's definition with the recovered
	// variable whose process() created it.
	for _, h := range d.extensionOrder {
		if e := insert(h); e != nil {
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
