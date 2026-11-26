package shell

import "errors"

// Source: src/shell/dimacs_frontend.cpp read_dimacs reads dimacs.core from the
// SAT parameters with a false default. Read the current SAT module so callers
// can still update the setting between constructing and running the frontend.
func (f *DIMACSFrontend) coreEnabled() (bool, error) {
	options, err := f.globals.SourceGetModuleName("sat")
	if err != nil {
		return false, err
	}
	key := "dimacs.core"
	core, err := f.globals.Context.GetBoolName(&options, &key, false)
	return core, errors.Join(err, f.globals.Context.Del(&options))
}
