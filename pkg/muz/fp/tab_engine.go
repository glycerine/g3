package fp

// Source: src/muz/tab/tab_context.cpp and dl_register_engine.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/tab"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) clearTabAnswer() error {
	if e.tabAnswer == (ast.ASTRef{}) {
		return nil
	}
	return e.manager.DelRef(&e.tabAnswer)
}
func (e *PropositionalEngine) queryTabPredicate(declaration ast.ASTHandle) (status lbool.LBool, err error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	if err = e.clearTabAnswer(); err != nil {
		return lbool.Undef, err
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		var valid bool
		valid, err = e.set.Finalize()
		if err != nil {
			return lbool.Undef, err
		}
		if !valid {
			return lbool.Undef, fmt.Errorf("muz/fp: negation is not stratified")
		}
	}
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, view.Close()) }()
	fallback, err := e.manager.Symbols.Intern("weight")
	if err != nil {
		return lbool.Undef, err
	}
	selected, err := view.Symbol("tab.selection", fallback)
	if err != nil {
		return lbool.Undef, err
	}
	strategy, err := e.manager.Symbols.String(selected)
	if err != nil {
		return lbool.Undef, err
	}
	rw, err := rewriter.NewTheoryRewriter(e.manager, e.params, e.globals, nil)
	if err != nil {
		if rw != nil {
			err = errors.Join(err, rw.Close())
		}
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	cooperation, err := cooperate.NewCooperation(e.manager.Budget())
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, cooperation.Close()) }()
	lite, err := qe.NewLite(e.manager, nil, false, qe.Config{Context: e.params, Globals: e.globals, Cooperation: cooperation})
	if err != nil {
		if lite != nil {
			err = errors.Join(err, lite.Close())
		}
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, lite.Close()) }()
	params, err := e.params.NewValue()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, e.params.Del(&params)) }()
	mbqi, timeout := "mbqi", "timeout"
	if err = e.params.SetBoolName(&params, &mbqi, false); err != nil {
		return lbool.Undef, err
	}
	if err = e.params.SetUIntName(&params, &timeout, 1000); err != nil {
		return lbool.Undef, err
	}
	config := smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}
	querySolver, err := smtcore.NewSMTSolver(e.manager, &params, "", config)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, querySolver.Close()) }()
	subSolver, err := smtcore.NewSMTSolver(e.manager, nil, "", config)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, subSolver.Close()) }()
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return lbool.Undef, err
	}
	args := make([]ast.ASTHandle, arity)
	for i := range args {
		sort, e2 := e.manager.GetDomain(declaration, uint32(i))
		if e2 != nil {
			return lbool.Undef, e2
		}
		args[i], e2 = e.manager.MkVar(arity-uint32(i)-1, sort)
		if e2 != nil {
			return lbool.Undef, e2
		}
	}
	query, err := e.manager.MkApp(declaration, args)
	if err != nil {
		return lbool.Undef, err
	}
	ref, err := e.manager.NewRef(query)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, e.manager.DelRef(&ref)) }()
	status, e.tabAnswer, err = tab.QueryPredicate(e.set, query, rw, lite, querySolver, subSolver, strategy)
	return status, err
}
