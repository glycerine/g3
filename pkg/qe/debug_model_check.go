package qe

// Source: src/qe/qsat.cpp validate_assumptions/validate_defs,
// src/qe/qe_mbp.cpp validate_model and src/qe/qe_arith.cpp
// linearize/validate_model: SASSERT and DEBUG_CODE model evaluations.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// debugModelEval evaluates forms in md for a debug-only source assertion and
// passes each form with its value to visit, which returns false to stop. The
// source runs these checks only in debug builds, some of them through an
// evaluator that the release path also uses. Here every form gets a fresh
// evaluator (with the given model-completion setting), each value is
// released before the next form, and the whole check runs under
// ASTManager.IsolatedCheck, so debug builds keep the release builds' AST IDs,
// evaluator caches and trace log.
func debugModelEval(m *ast.ASTManager, config Config, md *model.ModelCore, completion bool, forms []ast.ASTHandle, visit func(h, v ast.ASTHandle) (bool, error)) error {
	if md == nil || md.Manager() != m {
		return ast.ErrASTHandle
	}
	_, err := m.IsolatedCheck(func() error {
		for _, h := range forms {
			more, err := debugModelEvalOne(m, config, md, completion, h, visit)
			if err != nil || !more {
				return err
			}
		}
		return nil
	})
	return err
}
func debugModelEvalOne(m *ast.ASTManager, config Config, md *model.ModelCore, completion bool, h ast.ASTHandle, visit func(h, v ast.ASTHandle) (bool, error)) (more bool, err error) {
	ev, err := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace})
	if ev != nil {
		defer func() { err = errors.Join(err, ev.Close()) }()
	}
	if err != nil {
		return false, err
	}
	val, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&val)) }()
	if err = ev.SetModelCompletion(completion); err != nil {
		return false, err
	}
	if err = ev.Evaluate(h, val); err != nil {
		return false, err
	}
	v, err := m.RefGet(val)
	if err != nil {
		return false, err
	}
	return visit(h, v)
}

// debugRefHandles reads borrowed handles for a debug check.
func debugRefHandles(m *ast.ASTManager, forms []ast.ASTRef) ([]ast.ASTHandle, error) {
	hs := make([]ast.ASTHandle, len(forms))
	for i, r := range forms {
		h, err := m.RefGet(r)
		if err != nil {
			return nil, err
		}
		hs[i] = h
	}
	return hs, nil
}
