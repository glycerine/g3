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

// SetParameters validates and copies fixedpoint-local options for later queries.
func (f *APIFixedpoint) SetParameters(parameters *ParameterSet) (resultErr error) {
	if err := f.open(); err != nil {
		return err
	}
	if err := parameters.open(); err != nil {
		return err
	}
	if parameters.owner != f.owner {
		return ErrSMTContextOwner
	}
	descriptions, err := f.ParameterDescriptions()
	if err != nil {
		return err
	}
	if err = parameters.Validate(descriptions); err != nil {
		return errors.Join(err, descriptions.Close())
	}
	if err = descriptions.Close(); err != nil {
		return err
	}
	next, err := (&SMTContext{smtContextState: f.owner}).NewParameterSet()
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			resultErr = errors.Join(resultErr, next.Close())
		}
	}()
	if f.params != nil {
		if err = next.params.Append(&next.value, &f.params.value); err != nil {
			return err
		}
	}
	if err = next.params.Append(&next.value, &parameters.value); err != nil {
		return err
	}
	old := f.params
	f.params, published = next, true
	if old != nil {
		if err = old.Close(); err != nil {
			f.retiredParams = append(f.retiredParams, old)
			return err
		}
	}
	return nil
}
