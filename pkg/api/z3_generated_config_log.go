package api

// Source: generated src/api/api_log_macros.cpp, config command records.
// The caller emits Result after a successful config creation, matching the
// source RETURN_Z3 macro's separate result record.

func (log *Z3LogWriter) LogZ3MkConfig() error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	return log.Call(3)
}
func (log *Z3LogWriter) LogZ3DelConfig(pointer uint64) error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	if err := log.Pointer(pointer); err != nil {
		return err
	}
	return log.Call(4)
}
func (log *Z3LogWriter) LogZ3SetParamValue(pointer uint64, name, value string) error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	if err := log.Pointer(pointer); err != nil {
		return err
	}
	if err := log.String(name); err != nil {
		return err
	}
	if err := log.String(value); err != nil {
		return err
	}
	return log.Call(5)
}
