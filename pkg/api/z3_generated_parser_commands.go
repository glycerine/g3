package api

// Source: generated api_commands.cpp, api_ast.cpp and api_parsers.cpp.

func replayBenchmarkString(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	fields := make([]string, 4)
	for i := range fields {
		fields[i], err = r.String(uint32(i + 1))
		if err != nil {
			return err
		}
	}
	assumptions, err := replayASTArray(r, owner, 5, 6)
	if err != nil {
		return err
	}
	formula, err := r.astObject(owner, 7)
	if err != nil {
		return err
	}
	_, err = owner.context.BenchmarkSMT2(formula.handle, APIBenchmarkConfig{
		Name: fields[0], Logic: fields[1], Status: fields[2], Attributes: fields[3], Assumptions: assumptions,
	})
	return err
}

func replayEvaluateSMT2(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	input, err := r.String(1)
	if err != nil {
		return err
	}
	output, err := owner.context.EvaluateSMT2(input)
	if err != nil {
		owner.parserError = output
		if owner.parserError == "" {
			owner.parserError = err.Error()
		}
		owner.results.setErrorCode(4) // Z3_PARSER_ERROR.
	}
	return nil
}

func replayGetParserError(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	owner.results.setErrorCode(0)
	_ = owner.parserError
	return nil
}

func replayGetErrorCode(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	_ = owner.results.errorCode
	return nil
}

func replaySetErrorCode(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	code, err := r.Uint(1)
	if err != nil {
		return err
	}
	owner.results.setErrorCode(code)
	return nil
}

func replayErrorMessage(r *Z3Replayer) error {
	id, err := r.Object(0)
	if err != nil {
		return err
	}
	if id != 0 && r.contexts[id] == nil {
		return ErrZ3ReplayReference
	}
	_, err = r.Uint(1)
	return err
}
