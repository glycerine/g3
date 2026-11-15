package cmd_context

// Source: src/cmd_context/basic_cmds.cpp get_option_cmd.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// getOptionKeywords are the keyword symbols get_option_cmd::set_next_arg
// compares an SMT-LIB keyword against; any other spelling (":produce_models",
// ":Print-Success") goes to gparams::get_value.
var getOptionKeywords = map[string]bool{
	":print-success": true, ":expand-definitions": true, ":interactive-mode": true,
	":produce-assertions": true, ":produce-proofs": true, ":produce-interpolants": true,
	":produce-unsat-cores": true, ":produce-models": true, ":produce-assignments": true,
	":global-decls": true, ":global-declarations": true, ":random-seed": true,
	":verbosity": true, ":regular-output-channel": true, ":diagnostic-output-channel": true,
	":error-behavior": true, ":int-real-coercions": true,
}

// GetOption reads an option. A name with a leading colon is an SMT-LIB
// get-option keyword: only the exact built-in keywords are context options.
func (c *Context) GetOption(name string) (string, bool, error) {
	if err := c.valid(); err != nil {
		return "", false, err
	}
	keyword := strings.HasPrefix(name, ":") && !getOptionKeywords[name]
	name = strings.ReplaceAll(strings.TrimPrefix(name, ":"), "-", "_")
	state, err := c.parameters.State()
	if err != nil {
		return "", false, err
	}
	boolean := func(value bool) (string, bool, error) {
		return strconv.FormatBool(value), true, nil
	}
	selector := name
	if keyword && name != "timeout" && name != "rlimit" {
		// gparams::get_value(opt); timeout and rlimit are read from the
		// context parameters gparams::set updated.
		selector = ""
	}
	switch selector {
	case "print_success":
		return boolean(c.PrintSuccess)
	case "interactive_mode", "produce_assertions":
		return boolean(c.InteractiveMode)
	case "produce_proofs", "produce_interpolants":
		return boolean(state.Proof)
	case "produce_unsat_cores":
		return boolean(state.UnsatCore)
	case "produce_models":
		return boolean(state.Model)
	case "produce_assignments":
		return boolean(c.ProduceAssignments)
	case "global_decls", "global_declarations":
		return boolean(c.GlobalDeclarations)
	case "random_seed":
		return strconv.FormatUint(uint64(c.RandomSeed), 10), true, nil
	case "verbosity":
		value, e := diagnostic.ProcessVerbosityLevel()
		return strconv.FormatUint(uint64(value), 10), e == nil, e
	case "regular_output_channel":
		if c.regularOutputName != "" {
			return c.regularOutputName, true, nil
		}
		return "stdout", true, nil
	case "diagnostic_output_channel":
		if c.diagnosticOutputName != "" {
			return c.diagnosticOutputName, true, nil
		}
		return "stderr", true, nil
	case "error_behavior":
		if c.ExitOnError {
			return "immediate-exit", true, nil
		}
		return "continued-execution", true, nil
	case "int_real_coercions":
		manager, e := c.Manager()
		if e != nil {
			return "", false, e
		}
		value, e := manager.IntRealCoercions()
		if e != nil {
			return "", false, e
		}
		return boolean(value)
	case "timeout", "rlimit":
		defaultValue := ^uint32(0)
		if name == "rlimit" {
			defaultValue = 0
		}
		value, e := c.config.Params.GetUIntName(&c.params, &name, defaultValue)
		return strconv.FormatUint(uint64(value), 10), e == nil, e
	case "expand_definitions", "produce_unsat_assumptions", "numeral_as_real", "reproducible_resource_limit":
		return "", false, nil
	}
	value, err := c.config.Globals.SourceGetValueName(name)
	if err == nil {
		return value, true, nil
	}
	var unsupported z3exception.Z3Exception
	if errors.As(err, &unsupported) {
		return "", false, nil
	}
	return "", false, err
}
