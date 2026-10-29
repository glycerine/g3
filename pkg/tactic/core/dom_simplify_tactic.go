package core

// Source: src/tactic/core/dom_simplify_tactic.cpp/.h.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"reflect"
)

type DomSimplifyTactic struct {
	self       *DomSimplifyTactic
	manager    *ast.ASTManager
	dominators *expressionDominators
	maxDepth   uint32
	forward    bool
	traceTags  *diagnostic.TraceTags
	refs       uint32
	ownerBytes uint64
	closed     bool
}

func (t *DomSimplifyTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if e := t.open(); e != nil {
		return e
	}
	t.traceTags = tags
	return nil
}

var _ tactic.Tactic = (*DomSimplifyTactic)(nil)

func NewDomSimplifyTactic(m *ast.ASTManager, _ *uparams.ParamsRef) (*DomSimplifyTactic, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(DomSimplifyTactic{}).Size())
	if e := m.Budget().Reserve(n); e != nil {
		return nil, e
	}
	t := &DomSimplifyTactic{manager: m, maxDepth: 1024, ownerBytes: n}
	t.self = t
	t.dominators = newExpressionDominators(m)
	return t, nil
}
func (t *DomSimplifyTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *DomSimplifyTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *DomSimplifyTactic) DecRef() error {
	if e := t.open(); e != nil {
		return e
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}
func (t *DomSimplifyTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *DomSimplifyTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || out == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = out.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("dom-simplify", g)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	changed := true
	for changed {
		inconsistent, checkErr := g.Inconsistent()
		if checkErr != nil {
			return checkErr
		}
		if inconsistent {
			break
		}
		changed = false
		for _, forward := range []bool{true, false} {
			t.forward = forward
			n, e := g.Size()
			if e != nil {
				return e
			}
			forms := make([]ast.ASTHandle, n)
			for i := uint32(0); i < n; i++ {
				forms[i], e = g.Form(i)
				if e != nil {
					return e
				}
			}
			root, e := t.manager.MkAndArgs(forms)
			if e != nil {
				return e
			}
			if _, e = t.dominators.compile(root); e != nil {
				return e
			}
			t.traceDominatorTree()
			ctx := &domContext{}
			order := make([]uint32, n)
			for i := range order {
				if forward {
					order[i] = uint32(i)
				} else {
					order[i] = n - 1 - uint32(i)
				}
			}
			for _, i := range order {
				old := forms[i]
				next, e := t.simplify(old, ctx, 0)
				if e != nil {
					return e
				}
				changed = changed || next != old
				if next != old {
					t.traceRewrite(old, next)
				}
				pr, e := g.Proof(i)
				if e != nil {
					return e
				}
				if !pr.IsNull() && next != old {
					rw, e := t.manager.MkProofRewrite(old, next)
					if e != nil {
						return e
					}
					pr, e = t.manager.MkProofModusPonens(pr, rw)
					if e != nil {
						return e
					}
				}
				dep, e := g.Dependency(i)
				if e != nil {
					return e
				}
				if e = g.Update(i, next, pr, dep); e != nil {
					return e
				}
				if e = t.assertContext(ctx, next, false); e != nil {
					return e
				}
			}
		}
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	return out.AddGoal(g)
}
func (t *DomSimplifyTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *DomSimplifyTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *DomSimplifyTactic) CollectStatistics() map[string]float64         { return nil }
func (t *DomSimplifyTactic) ResetStatistics() error                        { return t.open() }
func (t *DomSimplifyTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *DomSimplifyTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *DomSimplifyTactic) Cleanup() error {
	if e := t.open(); e != nil {
		return e
	}
	t.dominators.reset()
	return nil
}
func (t *DomSimplifyTactic) Reset() error { return t.Cleanup() }
func (t *DomSimplifyTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	return NewDomSimplifyTactic(m, nil)
}
