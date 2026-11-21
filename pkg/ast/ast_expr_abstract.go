package ast

// Source: src/ast/expr_abstract.cpp:22-145; src/util/ref_buffer.h:38-99.

import (
	"errors"
	"math"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ExprAbstractor replaces selected constants with de Bruijn variables. Its
// pinned slice owns generated expressions until their abstraction returns.
// An interrupted call leaves its remaining references available to Close.
type ExprAbstractor struct {
	self    *ExprAbstractor
	manager *ASTManager
	pinned  []ASTRef
	closed  bool
}

func NewExprAbstractor(m *ASTManager) (*ExprAbstractor, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return nil, e
	}
	m.mu.Unlock()
	a := &ExprAbstractor{manager: m}
	a.self = a
	return a, nil
}

func (a *ExprAbstractor) pin(h ASTHandle) error {
	r, e := a.manager.NewRef(h)
	if e != nil {
		return e
	}
	a.pinned = append(a.pinned, r)
	return nil
}

// Reserve ownership before a constructor can produce a fresh expression. The
// source ref_buffer acquires its expression in the same construction scope;
// a failed Go reference allocation must not strand an unreferenced AST node.
func (a *ExprAbstractor) reservePin() (ASTRef, error) {
	r, err := a.manager.NewRef(ASTHandle{})
	if r != (ASTRef{}) {
		a.pinned = append(a.pinned, r)
	}
	return r, err
}

func (a *ExprAbstractor) setPin(ref ASTRef, node ASTHandle, cause error) error {
	if !node.IsNull() {
		cause = errors.Join(cause, a.manager.SetRef(ref, node))
	}
	return cause
}

func (a *ExprAbstractor) Close() error {
	if a == nil || a.self != a || a.manager == nil {
		return ErrASTHandle
	}
	if a.closed {
		return nil
	}
	if e := a.releasePins(0); e != nil {
		return e
	}
	a.pinned = nil
	a.closed = true
	return nil
}

// Each recursive expr_abstract has its own m_pinned. Release that suffix
// in creation order once its caller owns the result.
func (a *ExprAbstractor) releasePins(start int) error {
	for i := start; i < len(a.pinned); i++ {
		if err := a.manager.DelRef(&a.pinned[i]); err != nil {
			if a.pinned[i] == (ASTRef{}) {
				i++
			}
			a.pinned = append(a.pinned[:start], a.pinned[i:]...)
			return err
		}
	}
	a.pinned = a.pinned[:start]
	return nil
}

// Abstract writes an owning result reference after replacing bound constants.
// A failed call retains acquired references for an explicit Close retry.
func (a *ExprAbstractor) Abstract(base uint32, bound []ASTHandle, root ASTHandle, out ASTRef) error {
	if a == nil || a.self != a || a.manager == nil || a.closed || len(a.pinned) != 0 || len(bound) > math.MaxUint32 {
		return ErrASTHandle
	}
	return a.abstractInto(base, bound, root, out)
}

func (a *ExprAbstractor) abstractInto(base uint32, bound []ASTHandle, root ASTHandle, out ASTRef) error {
	start := len(a.pinned)
	result, e := a.abstract(base, bound, root)
	if e != nil {
		return e
	}
	if e = a.manager.SetRef(out, result); e != nil {
		return e
	}
	if e = a.releasePins(start); e != nil {
		return e
	}
	return a.traceExprAbstract(root, nil, result, false)
}

// MkForall abstracts the supplied constant applications and publishes the
// resulting quantifier through the caller's owning result reference.
func (a *ExprAbstractor) MkForall(bound []ASTHandle, body ASTHandle, out ASTRef) error {
	return a.mkQuantifier(true, bound, body, out)
}

// MkExists is the existential counterpart of MkForall.
func (a *ExprAbstractor) MkExists(bound []ASTHandle, body ASTHandle, out ASTRef) error {
	return a.mkQuantifier(false, bound, body, out)
}

func (a *ExprAbstractor) mkQuantifier(forall bool, bound []ASTHandle, body ASTHandle, out ASTRef) error {
	if e := a.Abstract(0, bound, body, out); e != nil {
		return e
	}
	if len(bound) == 0 {
		return a.traceExprAbstract(body, nil, body, true)
	}
	m := a.manager
	sorts := make([]ASTHandle, 0, len(bound))
	names := make([]usymbol.Symbol, 0, len(bound))
	for _, b := range bound {
		kind, e := m.GetKind(b)
		if e != nil {
			return e
		}
		if kind != ASTApp {
			return ErrASTDomain
		}
		sort, e := m.GetSort(b)
		if e != nil {
			return e
		}
		decl, e := m.GetDecl(b)
		if e != nil {
			return e
		}
		name, e := m.GetASTName(decl)
		if e != nil {
			return e
		}
		sorts = append(sorts, sort)
		names = append(names, name)
	}
	abstracted, e := m.RefGet(out)
	if e != nil {
		return e
	}
	q, e := m.MkQuantifier(ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Body: abstracted})
	// Registration can publish the quantifier before its trace stream fails.
	// Capture that returned node even when construction reports the failure.
	if e = a.setPin(out, q, e); e != nil {
		return e
	}
	return a.traceExprAbstract(body, bound, q, true)
}

func (a *ExprAbstractor) abstract(base uint32, bound []ASTHandle, root ASTHandle) (ASTHandle, error) {
	m := a.manager
	if len(bound) == 0 {
		return root, nil
	}
	if ASTSourceDebug {
		count, e := m.GetRefCount(root)
		if e != nil {
			return ASTHandle{}, e
		}
		if count == 0 {
			return ASTHandle{}, ErrASTDomain
		}
	}
	result := make(map[ASTHandle]ASTHandle)
	stack := []ASTHandle{root}
	for i, b := range bound {
		sort, e := m.GetSort(b)
		if e != nil {
			return ASTHandle{}, e
		}
		pin, e := a.reservePin()
		if e != nil {
			return ASTHandle{}, e
		}
		v, e := m.MkVar(base+uint32(len(bound)-i-1), sort)
		if e = a.setPin(pin, v, e); e != nil {
			return ASTHandle{}, e
		}
		result[b] = v
	}
	for len(stack) != 0 {
		last := len(stack) - 1
		curr := stack[last]
		if _, ok := result[curr]; ok {
			stack = stack[:last]
			continue
		}
		kind, e := m.GetKind(curr)
		if e != nil {
			return ASTHandle{}, e
		}
		switch kind {
		case ASTVar:
			result[curr] = curr
			stack = stack[:last]
		case ASTApp:
			n, e := m.GetNumArgs(curr)
			if e != nil {
				return ASTHandle{}, e
			}
			args := make([]ASTHandle, 0, n)
			changed, visited := false, true
			for i := uint32(0); i < n; i++ {
				child, e := m.GetArg(curr, i)
				if e != nil {
					return ASTHandle{}, e
				}
				v, ok := result[child]
				if !ok {
					stack = append(stack, child)
					visited = false
				} else {
					changed = changed || v != child
					args = append(args, v)
				}
			}
			if !visited {
				continue
			}
			v := curr
			if changed {
				decl, e := m.GetDecl(curr)
				if e != nil {
					return ASTHandle{}, e
				}
				pin, e := a.reservePin()
				if e != nil {
					return ASTHandle{}, e
				}
				v, e = m.MkApp(decl, args)
				if e = a.setPin(pin, v, e); e != nil {
					return ASTHandle{}, e
				}
			}
			result[curr] = v
			stack = stack[:last]
		case ASTQuantifier:
			v, e := a.abstractQuantifier(base, bound, curr)
			if e != nil {
				return ASTHandle{}, e
			}
			result[curr] = v
			stack = stack[:last]
		default:
			return ASTHandle{}, ErrASTDomain
		}
	}
	v, found := result[root]
	if !found {
		return ASTHandle{}, ErrASTDomain
	}
	return v, nil
}

func (a *ExprAbstractor) abstractQuantifier(base uint32, bound []ASTHandle, q ASTHandle) (ASTHandle, error) {
	m := a.manager
	count, err := m.GetNumDeclarations(q)
	if err != nil {
		return ASTHandle{}, err
	}
	numPatterns, err := m.GetNumPatterns(q)
	if err != nil {
		return ASTHandle{}, err
	}
	// The enclosing map owns the updated quantifier. Its body and patterns
	// are temporary references local to this quantifier's reconstruction.
	pin, err := a.reservePin()
	if err != nil {
		return ASTHandle{}, err
	}
	start := len(a.pinned)
	result, err := a.reservePin()
	if err != nil {
		return ASTHandle{}, err
	}
	abstractChild := func(h ASTHandle, err error) (ASTHandle, error) {
		if err == nil {
			err = a.abstractInto(base+count, bound, h, result)
		}
		if err != nil {
			return ASTHandle{}, err
		}
		return m.RefGet(result)
	}
	patterns := make([]ASTHandle, numPatterns)
	for i := range patterns {
		p, err := abstractChild(m.GetPattern(q, uint32(i)))
		if err != nil {
			return ASTHandle{}, err
		}
		if err = a.pin(p); err != nil {
			return ASTHandle{}, err
		}
		patterns[i] = p
	}
	body, err := abstractChild(m.GetQuantifierBody(q))
	if err != nil {
		return ASTHandle{}, err
	}
	next, err := m.UpdateQuantifierPatterns(q, patterns, body)
	if err = a.setPin(pin, next, err); err != nil {
		return ASTHandle{}, err
	}
	return next, a.releasePins(start)
}
