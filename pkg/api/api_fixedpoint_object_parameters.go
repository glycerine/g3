package api

// Source: src/api/api_datalog.cpp fixedpoint parameter descriptions and help.

import "errors"

// ParameterDescriptions returns an independently owned fixedpoint catalog.
func (f *APIFixedpoint) ParameterDescriptions() (*ParameterDescriptions, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	return (&SMTContext{smtContextState: f.owner}).ParameterDescriptions("fixedpoint")
}

// ParameterHelp renders the fixedpoint catalog in source parameter-help format.
func (f *APIFixedpoint) ParameterHelp() (text string, resultErr error) {
	d, err := f.ParameterDescriptions()
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, d.Close()) }()
	return d.value.Display(0, false, true)
}
