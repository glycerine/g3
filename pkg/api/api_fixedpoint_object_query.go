package api

// Source: src/api/api_datalog.cpp:277-327.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (f *APIFixedpoint) Query(declaration ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	f.hasLastExpression = false
	f.hasLastRelations = false
	status, err := f.executeQuery(true, func(p *uparams.ParamsRef) (lbool.LBool, error) {
		return f.command.QueryFixedpointTimed(declaration, p)
	})
	if err == nil {
		f.lastQuery, f.hasLastQuery = declaration, true
	}
	return status, err
}

// QueryExpression evaluates a Boolean fixedpoint query.
func (f *APIFixedpoint) QueryExpression(query ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	f.hasLastExpression = false
	f.hasLastRelations = false
	status, err := f.executeQuery(true, func(p *uparams.ParamsRef) (lbool.LBool, error) {
		return f.command.QueryFixedpointExpressionTimed(query, p)
	})
	if err != nil {
		return status, err
	}
	// Z3_fixedpoint_query is context::query(expr*) for every expression,
	// nullary relations included; its answer and model are the expression
	// query's.
	f.hasLastExpression = true
	return status, nil
}

// QueryExpressionFromLevel runs a Boolean query at a Spacer proof level.
func (f *APIFixedpoint) QueryExpressionFromLevel(query ast.ASTHandle, level uint32) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	f.hasLastExpression = false
	f.hasLastRelations = false
	status, err := f.executeQuery(true, func(p *uparams.ParamsRef) (lbool.LBool, error) {
		return f.command.QueryFixedpointExpressionFromLevelTimed(query, level, p)
	})
	if err == nil {
		f.hasLastExpression = true
	}
	return status, err
}

func (f *APIFixedpoint) QueryRelations(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := f.open(); err != nil {
		return lbool.Undef, err
	}
	f.hasLastQuery = false
	f.hasLastExpression = false
	f.hasLastRelations = false
	status, err := f.executeQuery(false, func(p *uparams.ParamsRef) (lbool.LBool, error) {
		return f.command.QueryFixedpointRelationsTimed(declarations, p)
	})
	if err == nil {
		if len(declarations) == 1 {
			f.lastQuery, f.hasLastQuery = declarations[0], true
		} else {
			f.hasLastRelations = true
		}
	}
	return status, err
}
