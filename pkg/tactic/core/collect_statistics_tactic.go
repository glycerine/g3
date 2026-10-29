package core

// Source: src/tactic/core/collect_statistics_tactic.cpp/.h.

import (
	"errors"
	"io"
	"os"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type CollectStatisticsTactic struct {
	self       *CollectStatisticsTactic
	manager    *ast.ASTManager
	output     io.Writer
	stats      map[string]uint64
	refs       uint32
	ownerBytes uint64
	closed     bool
}

var _ tactic.Tactic = (*CollectStatisticsTactic)(nil)

func NewCollectStatisticsTactic(m *ast.ASTManager, p *uparams.ParamsRef) (*CollectStatisticsTactic, error) {
	return NewCollectStatisticsTacticWithOutput(m, p, os.Stdout)
}

func NewCollectStatisticsTacticWithOutput(m *ast.ASTManager, _ *uparams.ParamsRef, output io.Writer) (*CollectStatisticsTactic, error) {
	if m == nil || output == nil {
		return nil, ast.ErrASTHandle
	}
	amount := uint64(reflect.TypeOf(CollectStatisticsTactic{}).Size())
	if err := m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	t := &CollectStatisticsTactic{manager: m, output: output, stats: make(map[string]uint64), ownerBytes: amount}
	t.self = t
	return t, nil
}

func (t *CollectStatisticsTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *CollectStatisticsTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *CollectStatisticsTactic) DecRef() error {
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
func (t *CollectStatisticsTactic) Close() error {
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

func (t *CollectStatisticsTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("collect-statistics", goal)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	collector := collectStatisticsTraversal{owner: t, seenExpr: make(map[ast.ASTHandle]bool), seenSort: make(map[ast.ASTHandle]bool), seenDecl: make(map[ast.ASTHandle]bool)}
	n, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		h, e := goal.Form(i)
		if e != nil {
			return e
		}
		if e = collector.visit(h); e != nil {
			return e
		}
	}
	if err = t.write(); err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}

func (t *CollectStatisticsTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *CollectStatisticsTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *CollectStatisticsTactic) CollectStatistics() map[string]float64         { return nil }

// Source reset_statistics calls reset/cleanup, both no-ops; accumulated output
// counters intentionally survive this virtual reset.
func (t *CollectStatisticsTactic) ResetStatistics() error                 { return t.open() }
func (t *CollectStatisticsTactic) Cleanup() error                         { return t.open() }
func (t *CollectStatisticsTactic) Reset() error                           { return t.open() }
func (t *CollectStatisticsTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *CollectStatisticsTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *CollectStatisticsTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	return NewCollectStatisticsTacticWithOutput(m, nil, t.output)
}
