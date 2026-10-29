package fp

// Source: src/tactic/horn_subsume_model_converter.cpp Horn-clause
// normalization and reverse model reconstruction.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/transforms"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (e *PropositionalEngine) normalizeInlineModelRule(rule transforms.InlineModelRule) (declaration ast.ASTHandle, result ast.ASTRef, resultErr error) {
	head, err := e.manager.RefGet(rule.Head)
	if err != nil {
		return declaration, result, err
	}
	body, err := e.manager.RefGet(rule.Body)
	if err != nil {
		return declaration, result, err
	}
	declaration, err = e.manager.GetDecl(head)
	if err != nil {
		return declaration, result, err
	}
	if family, getErr := e.manager.GetFamilyID(declaration); getErr != nil || family != ast.NullFamilyID {
		if getErr == nil {
			getErr = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, result, getErr
	}
	free, err := astrewriter.NewExprFreeVars(e.manager)
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if err = free.Collect(head); err == nil {
		err = free.Accumulate(body)
	}
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if err = free.SetDefaultSort(boolean); err != nil {
		return ast.ASTHandle{}, result, err
	}
	freeCount := free.Size()
	arity, err := e.manager.GetNumArgs(head)
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if arity == 0 && freeCount == 0 {
		result, err = e.manager.NewRef(body)
		return declaration, result, err
	}

	conjuncts := []ast.ASTHandle{body}
	bindings := make([]ast.ASTHandle, freeCount)
	for index := uint32(0); index < arity; index++ {
		argument, getErr := e.manager.GetArg(head, index)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		sort, getErr := e.manager.GetSort(argument)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		formal, getErr := e.manager.MkVar(freeCount+index, sort)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		kind, getErr := e.manager.GetKind(argument)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		if kind == ast.ASTVar {
			variable, getErr := e.manager.GetVarIndex(argument)
			if getErr != nil {
				return ast.ASTHandle{}, result, getErr
			}
			if uint64(variable) >= uint64(len(bindings)) {
				return ast.ASTHandle{}, result, ast.ErrASTDomain
			}
			if bindings[variable].IsNull() {
				bindings[variable] = formal
				continue
			}
			argument = bindings[variable]
		}
		equality, getErr := e.manager.MkEq(formal, argument)
		if getErr != nil {
			return ast.ASTHandle{}, result, getErr
		}
		conjuncts = append(conjuncts, equality)
	}
	formula, err := e.manager.MkAndArray(conjuncts)
	if err != nil {
		return ast.ASTHandle{}, result, err
	}
	if len(bindings) != 0 {
		substitute, makeErr := astrewriter.NewVarSubstitution(e.manager, false)
		if makeErr != nil {
			return ast.ASTHandle{}, result, makeErr
		}
		defer func() { resultErr = errors.Join(resultErr, substitute.Close()) }()
		rewritten, makeErr := e.manager.NewRef(ast.ASTHandle{})
		if makeErr != nil {
			return ast.ASTHandle{}, result, makeErr
		}
		if makeErr = substitute.Substitute(formula, bindings, rewritten); makeErr != nil {
			return ast.ASTHandle{}, result, errors.Join(makeErr, e.manager.DelRef(&rewritten))
		}
		formula, makeErr = e.manager.RefGet(rewritten)
		if makeErr != nil {
			return ast.ASTHandle{}, result, errors.Join(makeErr, e.manager.DelRef(&rewritten))
		}
		defer func() { resultErr = errors.Join(resultErr, e.manager.DelRef(&rewritten)) }()
	}
	if freeCount != 0 {
		if err = free.Reverse(); err != nil {
			return ast.ASTHandle{}, result, err
		}
		sorts := free.Sorts()
		names := make([]usymbol.Symbol, freeCount)
		for i := range names {
			names[len(names)-i-1] = usymbol.Numerical(uint32(i))
		}
		formula, err = e.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: false, Sorts: sorts, Names: names, Body: formula})
		if err != nil {
			return ast.ASTHandle{}, result, err
		}
	}
	result, err = e.manager.NewRef(formula)
	return declaration, result, err
}
