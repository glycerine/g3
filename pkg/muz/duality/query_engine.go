package duality

import (
	"errors"

	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// QueryEngine owns the fixedpoint query's RPFP and Duality solver across
// repeated queries. Context and solver services are borrowed and must outlive
// Close. Registration and rule preprocessing are performed by its caller.
// Source: duality_dl_interface.cpp, duality_data and query solver lifecycle.
type QueryEngine struct {
	self    *QueryEngine
	context *graph.Context
	config  graph.EngineConfig
	active  *queryRun
	runs    []*queryRun
	status  lbool.LBool
	closed  bool
}
type queryRun struct {
	graph  *graph.RPFP
	engine *graph.Engine
}

func (r *queryRun) close() error {
	if r.engine != nil {
		if err := r.engine.Close(); err != nil {
			return err
		}
		r.engine = nil
	}
	if r.graph != nil {
		if err := r.graph.Close(); err != nil {
			return err
		}
		r.graph = nil
	}
	return nil
}

func NewQueryEngine(context *graph.Context, config graph.EngineConfig) (*QueryEngine, error) {
	if _, err := context.Manager(); err != nil {
		return nil, err
	}
	if config.Native == nil || config.Auxiliary == nil || config.Factory == nil || config.Simplifier == nil || config.Quantifiers == nil {
		return nil, graph.ErrWrapperOwner
	}
	e := &QueryEngine{context: context, config: config, status: lbool.Undef}
	e.self = e
	return e, nil
}
func (e *QueryEngine) open() error {
	if e == nil || e.self != e || e.closed {
		return graph.ErrWrapperOwner
	}
	_, err := e.context.Manager()
	return err
}
func (e *QueryEngine) reap() error {
	var result error
	kept := e.runs[:0]
	for _, run := range e.runs {
		if run == e.active {
			kept = append(kept, run)
			continue
		}
		if err := run.close(); err != nil {
			result = errors.Join(result, err)
			kept = append(kept, run)
		}
	}
	e.runs = kept
	return result
}

// Query learns from the previous solver before solving the new prepared graph.
// Duality safety means fixedpoint UNSAT; a counterexample means SAT. The source
// active bounded-result branch also returns UNSAT for bounded safety.
func (e *QueryEngine) Query(prepared *PreparedQuery) (status lbool.LBool, result error) {
	if err := e.open(); err != nil {
		return lbool.Undef, err
	}
	e.status = lbool.Undef
	nextGraph, err := prepared.BuildGraph(e.context, e.config.Simplifier, e.config.Quantifiers)
	if err != nil {
		return lbool.Undef, err
	}
	next := &queryRun{graph: nextGraph}
	e.runs = append(e.runs, next)
	defer func() {
		result = errors.Join(result, e.reap())
		if result != nil {
			status = lbool.Undef
			e.status = lbool.Undef
		}
	}()
	next.engine, err = graph.NewEngine(nextGraph, e.config)
	if err != nil {
		return lbool.Undef, err
	}
	if e.active != nil {
		if err := next.engine.LearnFrom(e.active.engine); err != nil {
			return lbool.Undef, err
		}
	}
	solved, err := next.engine.Solve()
	if err != nil {
		return lbool.Undef, err
	}
	status = lbool.True
	if solved {
		status = lbool.False
	}
	e.active = next
	e.status = status
	return status, nil
}

// Counterexample returns a borrowed result, invalidated by Query or Close.
func (e *QueryEngine) Counterexample() *graph.Counterexample {
	if e == nil || e.self != e || e.closed || e.active == nil || e.status != lbool.True {
		return nil
	}
	return e.active.engine.GetCounterexample()
}
func (e *QueryEngine) IsResultRecursionBounded() bool {
	return e != nil && e.self == e && !e.closed && e.active != nil && e.status == lbool.False && e.active.engine.IsResultRecursionBounded()
}
func (e *QueryEngine) Close() error {
	if e == nil {
		return nil
	}
	if e.self != e {
		return graph.ErrWrapperOwner
	}
	if e.closed {
		return nil
	}
	e.active = nil
	if err := e.reap(); err != nil {
		return err
	}
	e.closed = true
	e.status = lbool.Undef
	return nil
}
