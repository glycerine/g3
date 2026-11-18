package api

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/opt"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (o *APIOptimize) objectiveBound(index uint32, lower bool) (ast.ASTRef, error) {
	if err := o.open(); err != nil {
		return ast.ASTRef{}, err
	}
	bound, err := o.command.ObjectiveBound(index, lower)
	if err != nil {
		return ast.ASTRef{}, err
	}
	manager, err := o.command.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	// The bound is a temporary.
	ref, err := opt.ArithmeticValueExpression(manager, &bound.Value, bound.Integer)
	return ref, errors.Join(err, bound.Release(manager.NumeralService().Manager))
}

func (o *APIOptimize) objectiveBoundVector(index uint32, lower bool) (result []ast.ASTRef, resultErr error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	bound, err := o.command.ObjectiveBound(index, lower)
	if err != nil {
		return nil, err
	}
	manager, err := o.command.Manager()
	if err != nil {
		return nil, err
	}
	result, err = optimizationBoundVector(manager, bound)
	return result, errors.Join(err, bound.Release(manager.NumeralService().Manager))
}

func optimizationBoundVector(manager *ast.ASTManager, bound cmd_context.OptimizationBound) (result []ast.ASTRef, resultErr error) {
	defer func() {
		if resultErr != nil {
			for i := range result {
				resultErr = errors.Join(resultErr, manager.DelRef(&result[i]))
			}
			result = nil
		}
	}()
	// The infinity, rational and infinitesimal coefficients, in the source
	// vector order.
	for _, value := range []*unumeral.Rational{&bound.Value.Infinity, &bound.Value.Main, &bound.Value.Epsilon} {
		integer, err := manager.NumeralService().Manager.IsInt(value)
		if err != nil {
			return result, err
		}
		handle, err := manager.ArithmeticNumeralFromRational(value, integer)
		if err != nil {
			return result, err
		}
		ref, err := manager.NewRef(handle)
		if err != nil {
			return result, err
		}
		result = append(result, ref)
	}
	return result, nil
}
