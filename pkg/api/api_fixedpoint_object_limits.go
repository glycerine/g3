package api

// Source: src/api/api_datalog.cpp query scopes and fixedpoint parameter snapshot.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (f *APIFixedpoint) effectiveQueryTimeout() (uint32, error) {
	if f.hasTimeout {
		return f.timeout, nil
	}
	return (&SMTContext{smtContextState: f.owner}).contextTimeout()
}

// executeQuery copies the cumulative engine settings but selects the timeout
// from the latest API parameter update, falling back to the current parent.
func (f *APIFixedpoint) executeQuery(limited bool, run func(*uparams.ParamsRef) (lbool.LBool, error)) (status lbool.LBool, resultErr error) {
	context := &SMTContext{smtContextState: f.owner}
	timeout, err := f.effectiveQueryTimeout()
	if err != nil {
		return lbool.Undef, err
	}
	queryParams, err := context.NewParameterSet()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, queryParams.Close()) }()
	if f.params != nil {
		if err = queryParams.params.Append(&queryParams.value, &f.params.value); err != nil {
			return lbool.Undef, err
		}
	}
	if err = queryParams.SetUInt("timeout", timeout); err != nil {
		return lbool.Undef, err
	}
	if limited {
		text, found, e := f.owner.command.GetOption("rlimit")
		if e != nil {
			return lbool.Undef, e
		}
		if !found {
			return lbool.Undef, ErrSMTContextOwner
		}
		limit, e := strconv.ParseUint(text, 10, 32)
		if e != nil {
			return lbool.Undef, e
		}
		manager, e := f.command.Manager()
		if e != nil {
			return lbool.Undef, e
		}
		scope, e := control.NewLimitScope(manager.ResourceLimit(), uint32(limit))
		if e != nil {
			return lbool.Undef, e
		}
		defer scope.Close()
	}
	return run(&queryParams.value)
}
