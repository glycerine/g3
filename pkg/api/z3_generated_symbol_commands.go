package api

// Source: generated src/api/api_commands.cpp, symbol creation commands.

// RegisterZ3GeneratedSymbolCommands installs integer and string symbol replay.
// The generated source ignores these functions' return values; later symbol
// arguments are serialized by value in the log.
func RegisterZ3GeneratedSymbolCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	if err := r.RegisterCommand(29, "Z3_mk_int_symbol", replayMkIntSymbol); err != nil {
		return err
	}
	return r.RegisterCommand(30, "Z3_mk_string_symbol", replayMkStringSymbol)
}
func replayMkIntSymbol(r *Z3Replayer) error {
	context, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.Int(1)
	if err != nil {
		return err
	}
	_, err = context.context.SignedIntegerSymbol(value)
	return err
}
func replayMkStringSymbol(r *Z3Replayer) error {
	context, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.String(1)
	if err != nil {
		return err
	}
	_, err = context.context.StringSymbol(value)
	return err
}
