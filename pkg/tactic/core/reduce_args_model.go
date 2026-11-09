package core

// Source: src/tactic/core/reduce_args_tactic.cpp mk_mc.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (f *reduceArgsFrame) makeModelConverter(result *tactic.TacticResult) (err error) {
	m := f.owner.manager
	f.extension, err = tactic.NewExtensionModel(m, f.owner.services)
	if err != nil {
		return err
	}
	if err = f.extension.IncRef(); err != nil {
		return err
	}
	f.filter, err = tactic.NewFilterModel(m)
	if err != nil {
		return err
	}
	if err = f.filter.IncRef(); err != nil {
		return err
	}
	if err = f.filter.BindTrace(f.owner.trace, f.owner.globals); err != nil {
		return err
	}
	// var_ref_vector new_vars: reset per function (forward release) and
	// destroyed when mk_mc returns. new_args, new_eqs, new_t, cond and def
	// are raw pointers.
	var newVars []ast.ASTRef
	resetVars := func() (e error) {
		for i := range newVars {
			e = errors.Join(e, m.DelRef(&newVars[i]))
		}
		newVars = newVars[:0]
		return e
	}
	defer func() { err = errors.Join(err, resetVars()) }()
	for _, group := range f.groups.Entries() {
		d := group.Key
		candidate := f.candidates[d]
		if err = resetVars(); err != nil {
			return err
		}
		var variables, arguments []ast.ASTHandle
		for i, v := range candidate.mask {
			sort, err := m.GetDomain(d, uint32(i))
			if err != nil {
				return err
			}
			variable, err := m.MkVar(uint32(i), sort)
			if err != nil {
				return err
			}
			held, err := m.NewRef(variable)
			if err != nil {
				return err
			}
			newVars = append(newVars, held)
			variables = append(variables, variable)
			if !v {
				arguments = append(arguments, variable)
			}
		}
		var definition ast.ASTHandle
		for _, entry := range f.arg2funcs[group.Num-1].Entries() {
			if err = f.filter.Insert(entry.Value[0]); err != nil {
				return err
			}
			term, err := m.MkApp(entry.Value[0], arguments)
			if err != nil {
				return err
			}
			if definition.IsNull() {
				definition = term
				continue
			}
			var equalities []ast.ASTHandle
			for i, v := range candidate.mask {
				if v {
					arg, err := m.GetArg(entry.Key, uint32(i))
					if err != nil {
						return err
					}
					eq, err := m.MkEq(variables[i], arg)
					if err != nil {
						return err
					}
					equalities = append(equalities, eq)
				}
			}
			condition := equalities[0]
			if len(equalities) != 1 {
				if condition, err = m.MkAndArgs(equalities); err != nil {
					return err
				}
			}
			definition, err = m.MkIte(condition, term, definition)
			if err != nil {
				return err
			}
		}
		if err = f.extension.Insert(d, definition); err != nil {
			return err
		}
	}
	converter, err := tactic.ConcatModels(m.Budget(), f.filter, f.extension)
	if err != nil {
		return err
	}
	if err = result.SetModelConverter(converter); err != nil {
		return errors.Join(err, converter.Close())
	}
	return nil
}
