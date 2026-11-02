package fp

// Source: src/muz/fp/dl_cmds.cpp query-local params_ref update and
// src/muz/base/fixedpoint_params.hpp local-over-module lookup precedence.

import (
	"errors"

	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type fixedpointParamView struct {
	context *uparams.ParamsContext
	local   *uparams.ParamsRef
	global  uparams.ParamsRef
}

func (e *PropositionalEngine) fixedpointParamView() (*fixedpointParamView, error) {
	if e.params == nil || e.globals == nil {
		return nil, ErrPropositionalEngine
	}
	global, err := e.globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return nil, err
	}
	view := &fixedpointParamView{context: e.params, global: global}
	if e.queryParamsReady {
		view.local = &e.queryParams
	}
	return view, nil
}

func (v *fixedpointParamView) Close() error {
	if v == nil || v.context == nil {
		return nil
	}
	err := v.context.Del(&v.global)
	v.context, v.local = nil, nil
	return err
}

func (v *fixedpointParamView) Bool(name string, fallback bool) (bool, error) {
	if v.local == nil {
		return v.context.GetBoolName(&v.global, &name, fallback)
	}
	return v.context.GetBoolFallbackName(v.local, &name, &v.global, fallback)
}

func (v *fixedpointParamView) UInt(name string, fallback uint32) (uint32, error) {
	if v.local == nil {
		return v.context.GetUIntName(&v.global, &name, fallback)
	}
	return v.context.GetUIntFallbackName(v.local, &name, &v.global, fallback)
}

func (v *fixedpointParamView) Symbol(name string, fallback usymbol.Symbol) (usymbol.Symbol, error) {
	if v.local == nil {
		return v.context.GetSymbolName(&v.global, &name, fallback)
	}
	return v.context.GetSymbolFallbackName(v.local, &name, &v.global, fallback)
}

// SetQueryParameters retains a query-local parameter snapshot. A nil value
// clears the previous snapshot so module parameters become the sole source.
func (e *PropositionalEngine) SetQueryParameters(params *uparams.ParamsRef) (result error) {
	if err := e.open(); err != nil {
		return err
	}
	// PDR and Spacer updt_params rebuild their contexts so constructor
	// policy snapshots cannot survive a parameter update.
	if err := e.closePDRForReplacement(); err != nil {
		return err
	}
	if err := e.closeSpacerForReplacement(); err != nil {
		return err
	}
	if e.queryParamsReady {
		result = e.params.Del(&e.queryParams)
		e.queryParamsReady = false
	}
	if result == nil && params != nil {
		e.queryParams = e.params.EmptyValue()
		result = e.params.Assign(&e.queryParams, params)
		e.queryParamsReady = result == nil
		if result != nil {
			result = errors.Join(result, e.params.Del(&e.queryParams))
		}
	}
	if result == nil {
		e.revision++
	}
	return result
}

// SelectConfiguredEngine applies query-local :engine before the fixedpoint
// module setting and then the source auto-config default.
func (e *PropositionalEngine) SelectConfiguredEngine() (result error) {
	view, err := e.fixedpointParamView()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, view.Close()) }()
	selection, err := view.Symbol("engine", usymbol.Symbol{})
	if err != nil {
		return err
	}
	name := "auto-config"
	if selection.Value != 0 {
		name, _, err = e.params.SymbolsField.BareString(selection)
		if err != nil {
			return err
		}
	}
	return e.SelectEngine(name)
}
