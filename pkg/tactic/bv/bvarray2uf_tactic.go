package bv

// Source: src/tactic/bv/bvarray2uf_tactic.cpp/.h.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	"reflect"
)

type BVArray2UFTactic struct {
	self                         *BVArray2UFTactic
	manager                      *ast.ASTManager
	services                     model.ModelEvaluatorConfig
	rewriter                     *bvArrayRewriter
	arrayFamily, bitVectorFamily ast.FamilyID
	trace                        *diagnostic.TraceTags
	refs                         uint32
	ownerBytes                   uint64
	closed                       bool
}

var _ tactic.Tactic = (*BVArray2UFTactic)(nil)

func NewBVArray2UFTactic(m *ast.ASTManager, _ *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*BVArray2UFTactic, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(BVArray2UFTactic{}).Size())
	if e := m.Budget().Reserve(n); e != nil {
		return nil, e
	}
	t := &BVArray2UFTactic{manager: m, services: services, ownerBytes: n}
	t.self = t
	var e error
	if t.bitVectorFamily, e = m.RegisterBitVectorPlugin(); e != nil {
		m.Budget().Release(n)
		return nil, e
	}
	if t.arrayFamily, e = m.RegisterArrayPlugin(); e != nil {
		m.Budget().Release(n)
		return nil, e
	}
	if t.rewriter, e = newBVArrayRewriter(t); e != nil {
		if t.rewriter != nil {
			e = errors.Join(e, t.rewriter.close())
		}
		m.Budget().Release(n)
		return nil, e
	}
	return t, nil
}
func (t *BVArray2UFTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *BVArray2UFTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *BVArray2UFTactic) DecRef() error {
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
func (t *BVArray2UFTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if e := t.rewriter.close(); e != nil {
		return e
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *BVArray2UFTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if e := t.open(); e != nil {
		return e
	}
	t.trace = tags
	return nil
}
func (t *BVArray2UFTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || out == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = out.Prepare(t.manager); err != nil {
		return err
	}
	// tactic_report precedes fail_if_unsat_core_generation in the source.
	report, err := tactic.NewTacticReport("bvarray2uf", g)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	if err = tactic.FailIfUnsatCoreGeneration("bvarray2uf", g); err != nil {
		return err
	}
	models, err := g.ModelsEnabled()
	if err != nil {
		return err
	}
	if err = t.prepareModels(models); err != nil {
		return err
	}
	modelsPrepared := models
	defer func() {
		if modelsPrepared {
			err = errors.Join(err, t.discardPreparedModels())
		}
	}()
	// m_rw.reset()
	if err = t.rewriter.traversal.Reset(); err != nil {
		return err
	}
	n, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		bad, e := g.Inconsistent()
		if e != nil {
			return e
		}
		if bad {
			break
		}
		h, e := g.Form(i)
		if e != nil {
			return e
		}
		// m_produce_proofs is never set in the source, so the rewriter's
		// own proof goes to goal::update without a modus ponens step.
		next, pr, e := t.rewriter.rewrite(h)
		if e != nil {
			return e
		}
		dep, e := g.Dependency(i)
		if e != nil {
			return e
		}
		if e = g.Update(i, next, pr, dep); e != nil {
			return e
		}
	}
	for _, h := range t.rewriter.extra {
		if e := g.Assert(h); e != nil {
			return e
		}
	}
	if err = t.publishModels(out); err != nil {
		return err
	}
	modelsPrepared = false
	if err = g.IncDepth(); err != nil {
		return err
	}
	return out.AddGoal(g)
}
func (t *BVArray2UFTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *BVArray2UFTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *BVArray2UFTactic) CollectStatistics(*statistics.Statistics)      {}
func (t *BVArray2UFTactic) ResetStatistics() error                        { return t.open() }
func (t *BVArray2UFTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *BVArray2UFTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *BVArray2UFTactic) Cleanup() error {
	if e := t.open(); e != nil {
		return e
	}
	if e := t.discardPreparedModels(); e != nil {
		return e
	}
	// imp * d = alloc(imp, m, m_params); std::swap(d, m_imp); dealloc(d);
	fresh, e := newBVArrayRewriter(t)
	if e != nil {
		if fresh != nil {
			e = errors.Join(e, fresh.close())
		}
		return e
	}
	old := t.rewriter
	t.rewriter = fresh
	return old.close()
}
func (t *BVArray2UFTactic) Reset() error { return t.Cleanup() }
func (t *BVArray2UFTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	return NewBVArray2UFTactic(m, nil, t.services)
}

// MakeBVArray2UFTactic follows the source factory's cleanup wrapper.
func MakeBVArray2UFTactic(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewBVArray2UFTactic(m, p, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
