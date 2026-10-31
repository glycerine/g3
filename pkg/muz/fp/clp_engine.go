package fp

// Source: src/muz/clp/clp_context.cpp and src/muz/fp/dl_register_engine.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/clp"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) queryCLPPredicate(declaration ast.ASTHandle) (result lbool.LBool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, err := e.set.Finalize()
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
	state := e.predicates[declaration]
	if state == nil {
		return lbool.Undef, ErrPropositionalHorn
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, e.manager.DelRef(&refs[i]))
		}
	}()
	args := make([]ast.ASTHandle, len(state.sorts))
	for i, sort := range state.sorts {
		args[i], err = e.manager.MkVar(uint32(i), sort)
		if err != nil {
			return lbool.Undef, err
		}
		ref, err := e.manager.NewRef(args[i])
		if err != nil {
			return lbool.Undef, err
		}
		refs = append(refs, ref)
	}
	query, err := e.manager.MkApp(declaration, args)
	if err != nil {
		return lbool.Undef, err
	}
	queryRef, err := e.manager.NewRef(query)
	if err != nil {
		return lbool.Undef, err
	}
	refs = append(refs, queryRef)
	local, err := e.params.NewValue()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, e.params.Del(&local)) }()
	mbqi := "mbqi"
	if err = e.params.SetBoolName(&local, &mbqi, false); err != nil {
		return lbool.Undef, err
	}
	timeout := "timeout"
	if err = e.params.SetUIntName(&local, &timeout, 1000); err != nil {
		return lbool.Undef, err
	}
	backend, err := smtcore.NewSMTSolver(e.manager, &local, "", smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, backend.Close()) }()
	engine, err := clp.NewEngine(e.manager, e.set, backend, 20)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, engine.Close()) }()
	return engine.Query(query)
}
