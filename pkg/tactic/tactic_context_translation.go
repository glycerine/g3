package tactic

// Source: src/tactic/tactical.cpp tactic translation. This concrete traversal
// rebuilds the live composition nodes while target-specific tactic leaves are
// translated by their owning subsystem.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// TranslateWithContext rebuilds supported tactic compositions for a target
// parameter service. translateLeaf handles the concrete subsystem tactics;
// the caller owns the returned uncounted tactic in the usual tactic protocol.
func TranslateWithContext(value Tactic, m *ast.ASTManager, params *uparams.ParamsContext, translateLeaf func(Tactic) (Tactic, error)) (result Tactic, resultErr error) {
	if value == nil || m == nil || params == nil || translateLeaf == nil {
		return nil, ast.ErrASTHandle
	}
	owned := make([]Tactic, 0, 2)
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, owned[i].DecRef())
		}
		if resultErr != nil && result != nil {
			if e := result.IncRef(); e == nil {
				resultErr = errors.Join(resultErr, result.DecRef())
			} else {
				resultErr = errors.Join(resultErr, e)
			}
			result = nil
		}
	}()
	child := func(v Tactic) (Tactic, error) {
		translated, err := TranslateWithContext(v, m, params, translateLeaf)
		if err != nil {
			return nil, err
		}
		if err = translated.IncRef(); err != nil {
			return nil, err
		}
		owned = append(owned, translated)
		return translated, nil
	}
	switch t := value.(type) {
	case *primitiveTactic:
		return t.Translate(m)
	case *andThenTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		a, err := child(t.first)
		if err != nil {
			return nil, err
		}
		b, err := child(t.second)
		if err != nil {
			return nil, err
		}
		return newAndThenTactic(a, b)
	case *orElseTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		children := make([]Tactic, 0, len(t.children))
		for _, v := range t.children {
			c, err := child(v)
			if err != nil {
				return nil, err
			}
			children = append(children, c)
		}
		return NewOrElseTactic(children...)
	case *usingParamsTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		c, err := child(t.child)
		if err != nil {
			return nil, err
		}
		copied, err := t.context.TranslateTo(&t.params, params)
		if err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, params.Del(&copied)) }()
		return NewUsingParamsTactic(params, c, &copied)
	case *annotateTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		c, err := child(t.child)
		if err != nil {
			return nil, err
		}
		return NewAnnotateTactic(t.name, c)
	case *unaryTactic:
		if err := t.open(); err != nil {
			return nil, err
		}
		c, err := child(t.child)
		if err != nil {
			return nil, err
		}
		return newUnaryTactic(t.kind, c, t.threshold)
	default:
		return translateLeaf(value)
	}
}
