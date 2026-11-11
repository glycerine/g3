package smtcore

// Source: src/smt/smt_model_finder.cpp collect_partial_funcs,
// complete_partial_funcs and auf_solver::fix_model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func (g *smtModelFinderAUFGraph) collectPartialFunctions() ([]ast.ASTHandle, error) {
	// collect_partial_funcs walks m_A_f_is in table order into r, a
	// func_decl_set; complete_partial_funcs then walks r in its own order.
	partial := ast.NewObjMap(g.finder.manager)
	var layout []ast.ObjMapEntry
	if g.argumentLayout != nil {
		layout = g.argumentLayout.Entries()
	}
	for _, slot := range layout {
		function := slot.Key
		if found, err := partial.Contains(function); err != nil {
			return nil, err
		} else if found {
			continue
		}
		interpretation, err := g.model.Core().FunctionInterpretation(function)
		if err != nil {
			return nil, err
		}
		if interpretation == nil {
			arity, err := g.finder.manager.GetArity(function)
			if err != nil {
				return nil, err
			}
			interpretation, err = model.NewFunctionInterpretation(g.finder.manager, arity)
			if err != nil {
				return nil, err
			}
			if err = g.model.Core().RegisterFunction(function, interpretation); err != nil {
				return nil, errors.Join(err, interpretation.Close())
			}
		}
		isPartial, err := interpretation.IsPartial()
		if err != nil {
			return nil, err
		}
		if isPartial {
			if err = partial.Insert(function, [2]ast.ASTHandle{}); err != nil {
				return nil, err
			}
		}
	}
	entries := partial.Entries()
	functions := make([]ast.ASTHandle, len(entries))
	for i, entry := range entries {
		functions[i] = entry.Key
	}
	return functions, nil
}

func (g *smtModelFinderAUFGraph) completeProjectedFunctions(functions []ast.ASTHandle) error {
	for _, function := range functions {
		if err := g.completeProjectedFunction(function); err != nil {
			return err
		}
	}
	return nil
}

// completeProjectedFunction is one complete_partial_funcs iteration; args is
// its expr_ref_vector, released front to back when the iteration ends.
func (g *smtModelFinderAUFGraph) completeProjectedFunction(function ast.ASTHandle) (result error) {
	m := g.finder.manager
	if err := g.model.CompletePartialFunction(function); err != nil {
		return err
	}
	interpretation, err := g.model.Core().FunctionInterpretation(function)
	if err != nil {
		return err
	}
	constant, err := interpretation.IsConstant()
	if err != nil || constant {
		return err
	}
	arity, err := m.GetArity(function)
	if err != nil {
		return err
	}
	var args []ast.ASTRef
	defer func() {
		for i := range args {
			result = errors.Join(result, m.DelRef(&args[i]))
		}
	}()
	domain := make([]ast.ASTHandle, arity)
	arguments := make([]ast.ASTHandle, arity)
	hasProjection := false
	for i := uint32(0); i < arity; i++ {
		domain[i], err = m.GetDomain(function, i)
		if err != nil {
			return err
		}
		variable, err := m.MkVar(i, domain[i])
		if err != nil {
			return err
		}
		arguments[i] = variable
		if node := g.arguments[smtModelFinderAUFKey{function, i}]; node != nil {
			projection := node.root().projection
			if !projection.IsNull() {
				arguments[i], err = m.MkApp(projection, []ast.ASTHandle{variable})
				if err != nil {
					// The variable has no owner yet.
					if held, e := m.NewRef(variable); e == nil {
						err = errors.Join(err, m.DelRef(&held))
					}
					return err
				}
				hasProjection = true
			}
		}
		ref, err := m.NewRef(arguments[i])
		if err != nil {
			return err
		}
		args = append(args, ref)
	}
	if !hasProjection {
		return nil
	}
	name, err := m.GetASTName(function)
	if err != nil {
		return err
	}
	rangeSort, err := m.GetRange(function)
	if err != nil {
		return err
	}
	auxiliary, err := m.MkFreshFuncDecl(name, symbol.Symbol{}, domain, rangeSort)
	if err != nil {
		return err
	}
	definition, err := m.MkApp(auxiliary, arguments)
	if err != nil {
		// The fresh declaration has no owner yet; a failed application
		// must not strand it.
		if held, e := m.NewRef(auxiliary); e == nil {
			err = errors.Join(err, m.DelRef(&held))
		}
		return err
	}
	replacement, err := model.NewFunctionInterpretation(m, arity)
	if err != nil {
		return err
	}
	if err = replacement.SetElse(definition); err != nil {
		return errors.Join(err, replacement.Close())
	}
	if err = g.model.ReregisterFunction(function, replacement, auxiliary); err != nil {
		return errors.Join(err, replacement.Close())
	}
	return nil
}

func (g *smtModelFinderAUFGraph) extendModel() error {
	if err := g.cleanInstantiationSets(); err != nil {
		return err
	}
	partial, err := g.collectPartialFunctions()
	if err != nil {
		return err
	}
	// reset_eval_cache: from here on evaluation uses model completion.
	if err = g.resetEvalCache(); err != nil {
		return err
	}
	if err = g.fillEmptyInstantiationSets(); err != nil {
		return err
	}
	if err = g.freezeUniverses(); err != nil {
		return err
	}
	if err = g.addMonotoneExceptions(); err != nil {
		return err
	}
	for _, node := range g.nodes {
		if err := g.finder.checkpoint(); err != nil {
			return err
		}
		if node.parent != nil {
			continue
		}
		if node.mono {
			err = g.makeMonotoneProjection(node)
		} else {
			err = g.makeSimpleProjection(node)
		}
		if err != nil {
			return err
		}
	}
	for _, node := range g.nodes {
		if node.parent == nil {
			if err = node.set.MakeInverse(g.eval); err != nil {
				return err
			}
		}
	}
	return g.completeProjectedFunctions(partial)
}
