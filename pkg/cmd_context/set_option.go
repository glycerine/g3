package cmd_context

// Source: src/cmd_context/basic_cmds.cpp set_get_option_cmd and
// set_option_cmd (is_builtin_option, to_bool, to_unsigned,
// check_not_initialized, set_param, set_symbol, set_next_arg).

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// setOptionBuiltins is set_get_option_cmd::is_builtin_option: the exact
// keyword symbols. :numeral-as-real, :error-behavior and
// :int-real-coercions have set_symbol branches but are not in this list.
var setOptionBuiltins = map[string]bool{
	":print-success": true, ":print-warning": true, ":expand-definitions": true,
	":interactive-mode": true, ":produce-proofs": true, ":produce-unsat-cores": true,
	":produce-unsat-assumptions": true, ":produce-models": true, ":produce-assignments": true,
	":produce-interpolants": true, ":regular-output-channel": true, ":diagnostic-output-channel": true,
	":random-seed": true, ":verbosity": true, ":global-decls": true, ":global-declarations": true,
	":produce-assertions": true, ":reproducible-resource-limit": true,
}

func setOptionToBool(value string) (bool, error) {
	if value != "true" && value != "false" {
		return false, fmt.Errorf("invalid option value, true/false expected")
	}
	return value == "true", nil
}

// NumeralService is the rational service shared by this context's AST
// manager (rational's mpq manager); it exists before the manager does.
func (c *Context) NumeralService() *unumeral.RationalService { return c.config.Params.NumeralsField }

func setOptionToUnsigned(q *unumeral.RationalManager, value *unumeral.Rational) (uint32, error) {
	ok, err := q.IsUnsigned(value)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("option value is too big to fit in a machine integer.")
	}
	return q.Unsigned(value)
}

func (c *Context) setOptionCheckNotInitialized(name string) error {
	if c.manager != nil {
		return fmt.Errorf("error setting '%s', option value cannot be modified after initialization", name)
	}
	return nil
}

// setOptionParam is set_option_cmd::set_param: gparams::set(m_option, value)
// and global_params_updated. The command context also keeps local copies of
// a few context parameters; they are refreshed from the stored value.
func (c *Context) setOptionParam(keyword, value string) error {
	name := strings.ReplaceAll(strings.TrimPrefix(keyword, ":"), "-", "_")
	if err := c.config.Globals.SourceSetName(name, value); err != nil {
		return err
	}
	if err := c.globalParametersUpdated(); err != nil {
		return err
	}
	switch name {
	case "model":
		return c.SetBoolOption(name, value == "true")
	case "proof", "unsat_core":
		// The manager is already built with these settings; the source
		// only updates context_params, which no longer matters.
		if c.manager == nil {
			return c.SetBoolOption(name, value == "true")
		}
	case "timeout", "rlimit":
		stored, err := c.config.Globals.SourceGetValueName(name)
		if err != nil {
			return err
		}
		n, err := strconv.ParseUint(stored, 10, 32)
		if err != nil {
			return err
		}
		return c.SetUIntOption(name, uint32(n))
	case "dot_proof_file", "trace_file_name":
		return c.SetStringOption(name, value)
	}
	return nil
}

// SetOptionSymbol is set_option_cmd::set_symbol for keyword (with its
// colon, not normalized). unsupported is m_unsupported (:expand-definitions).
func (c *Context) SetOptionSymbol(keyword, value string) (unsupported bool, err error) {
	if err = c.valid(); err != nil {
		return false, err
	}
	boolean := func(set func(bool) error) error {
		b, err := setOptionToBool(value)
		if err != nil {
			return err
		}
		return set(b)
	}
	initial := func(name string, set func(bool) error) error {
		if err := c.setOptionCheckNotInitialized(name); err != nil {
			return err
		}
		return boolean(set)
	}
	switch keyword {
	case ":print-success":
		return false, boolean(func(b bool) error { c.PrintSuccess = b; return nil })
	case ":print-warning":
		return false, boolean(diagnostic.BorrowProcessWarningService().Enable)
	case ":expand-definitions":
		return true, nil
	case ":interactive-mode", ":produce-assertions":
		return false, initial(":produce-assertions", func(b bool) error { return c.SetBoolOption("interactive_mode", b) })
	case ":produce-proofs":
		return false, initial(":produce-proofs", func(b bool) error { return c.SetBoolOption("produce_proofs", b) })
	case ":produce-interpolants":
		return false, initial(":produce-interpolants", func(b bool) error { return c.SetBoolOption("produce_interpolants", b) })
	case ":produce-unsat-cores":
		return false, initial(":produce-unsat-cores", func(b bool) error { return c.SetBoolOption("produce_unsat_cores", b) })
	case ":produce-unsat-assumptions":
		return false, initial(":produce-unsat-assumptions", func(b bool) error { return c.SetBoolOption("produce_unsat_assumptions", b) })
	case ":produce-models":
		return false, boolean(func(b bool) error { return c.SetBoolOption("produce_models", b) })
	case ":produce-assignments":
		return false, boolean(func(b bool) error { return c.SetBoolOption("produce_assignments", b) })
	case ":global-decls", ":global-declarations":
		return false, initial(":global-decls", func(b bool) error { return c.SetBoolOption("global_declarations", b) })
	case ":numeral-as-real":
		return false, boolean(func(b bool) error { return c.SetBoolOption("numeral_as_real", b) })
	case ":int-real-coercions":
		return false, boolean(func(b bool) error { return c.SetBoolOption("int_real_coercions", b) })
	case ":error-behavior":
		return false, c.SetSymbolOption("error_behavior", value)
	}
	if setOptionBuiltins[keyword] {
		return false, fmt.Errorf("option value is not a symbol")
	}
	return false, c.setOptionParam(keyword, value)
}

// SetOptionNumeral is set_option_cmd::set_next_arg(rational).
// SetOptionNumeral is set_option_cmd::set_next_arg(rational const&); value
// is a rational of NumeralService().Manager.
func (c *Context) SetOptionNumeral(keyword string, value *unumeral.Rational) error {
	if err := c.valid(); err != nil {
		return err
	}
	q := c.NumeralService().Manager
	switch keyword {
	case ":random-seed":
		n, err := setOptionToUnsigned(q, value)
		if err != nil {
			return err
		}
		c.RandomSeed = n
		return nil
	case ":reproducible-resource-limit":
		// ctx.params().set_rlimit: the context parameters only, not gparams.
		n, err := setOptionToUnsigned(q, value)
		if err != nil {
			return err
		}
		name, text := "rlimit", strconv.FormatUint(uint64(n), 10)
		return c.parameters.Set(&name, &text)
	case ":verbosity":
		n, err := setOptionToUnsigned(q, value)
		if err != nil {
			return err
		}
		return diagnostic.SetProcessVerbosityLevel(n)
	}
	if setOptionBuiltins[keyword] {
		return fmt.Errorf("option value is not a numeral")
	}
	// rational::to_string: an integer, or num/den.
	text, err := q.String(value)
	if err != nil {
		return err
	}
	return c.setOptionParam(keyword, text)
}

// SetOptionString is set_option_cmd::set_next_arg(char const*).
func (c *Context) SetOptionString(keyword, value string) error {
	if err := c.valid(); err != nil {
		return err
	}
	switch keyword {
	case ":regular-output-channel":
		return c.SetStringOption("regular_output_channel", value)
	case ":diagnostic-output-channel":
		return c.SetStringOption("diagnostic_output_channel", value)
	}
	if setOptionBuiltins[keyword] {
		return fmt.Errorf("option value is not a string")
	}
	return c.setOptionParam(keyword, value)
}
