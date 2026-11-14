package core

// Source: src/tactic/core/dom_simplify_tactic.cpp/.h.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
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
	domSimp    DomSimplifier
	domCache   map[ast.ASTHandle]ast.ASTHandle // m_result
	domTrail   []ast.ASTRef                    // m_trail
	domArgs    []ast.ASTRef                    // m_args
}

// DomSimplifier is the callback contract used by the source's dominator
// simplification tactic. Assertion callbacks create one scope per assertion.
type DomSimplifier interface {
	AssertExpression(ast.ASTHandle, bool) (bool, error)
	Rewrite(ast.ASTRef) error
	Pop(uint32) error
	ScopeLevel() uint32
	TranslateDom(*ast.ASTManager) (DomSimplifier, error)
	UpdateParams(*uparams.ParamsRef) error
	Close() error
}

func (t *DomSimplifyTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if e := t.open(); e != nil {
		return e
	}
	t.traceTags = tags
	return nil
}

var _ tactic.Tactic = (*DomSimplifyTactic)(nil)

func newDomSimplifyTactic(m *ast.ASTManager, _ *uparams.ParamsRef) (*DomSimplifyTactic, error) {
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

// NewDomSimplifyTactic is mk_dom_simplify_tactic without the clean wrapper:
// dom_simplify_tactic over an expr_substitution_simplifier.
func NewDomSimplifyTactic(m *ast.ASTManager, p *uparams.ParamsRef) (*DomSimplifyTactic, error) {
	t, err := newDomSimplifyTactic(m, p)
	if err != nil {
		return nil, err
	}
	s, err := newExprSubstitutionSimplifier(m)
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	t.domSimp = s
	return t, nil
}

func NewDomSimplifyTacticWithSimplifier(m *ast.ASTManager, s DomSimplifier, p *uparams.ParamsRef) (*DomSimplifyTactic, error) {
	t, err := newDomSimplifyTactic(m, p)
	if err != nil {
		if s != nil {
			err = errors.Join(err, s.Close())
		}
		return nil, err
	}
	if s == nil {
		_ = t.Close()
		return nil, ast.ErrASTHandle
	}
	if p != nil {
		err = s.UpdateParams(p)
	}
	if err != nil {
		return nil, errors.Join(err, s.Close(), t.Close())
	}
	t.domSimp = s
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
	if err := t.domCleanup(); err != nil {
		return err
	}
	if t.domSimp != nil {
		if err := t.domSimp.Close(); err != nil {
			return err
		}
		t.domSimp = nil
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
	// mk_dom_simplify_tactic wraps the tactic in clean(): cleanup() follows
	// operator().
	defer func() { err = errors.Join(err, t.domCleanup()) }()
	return t.applyDomSimplifier(g, out)
}
func (t *DomSimplifyTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *DomSimplifyTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *DomSimplifyTactic) CollectStatistics(*statistics.Statistics)      {}
func (t *DomSimplifyTactic) ResetStatistics() error                        { return t.open() }
func (t *DomSimplifyTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *DomSimplifyTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *DomSimplifyTactic) Cleanup() error {
	if e := t.open(); e != nil {
		return e
	}
	return t.domCleanup()
}
func (t *DomSimplifyTactic) Reset() error { return t.Cleanup() }
func (t *DomSimplifyTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	s, err := t.domSimp.TranslateDom(m)
	if err != nil {
		return nil, err
	}
	return NewDomSimplifyTacticWithSimplifier(m, s, nil)
}
