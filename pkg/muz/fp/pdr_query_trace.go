package fp

// Source: src/muz/pdr/pdr_dl_interface.cpp:68,144 reset/query traces.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (e *PropositionalEngine) tracePDRFreshRule(index int) error {
	tags := e.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", "check_reset", "src/muz/pdr/pdr_dl_interface.cpp", 68,
		func() bool { return true }, func(out io.Writer) (resultErr error) {
			rule, err := e.set.Rule(index)
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, rule.Close()) }()
			writer := pdrTraceOperandWriter{out: out}
			_, _ = io.WriteString(writer, "Fresh rule ")
			return e.rules.WriteRuleDisplay(writer, rule)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// The trace owner records ostream failures. Suppress their Go write returns
// here so subsequent source operands (including dump exceptions) still run.
type pdrTraceOperandWriter struct{ out io.Writer }

func (w pdrTraceOperandWriter) Write(body []byte) (int, error) {
	_, _ = w.out.Write(body)
	return len(body), nil
}

func (e *PropositionalEngine) tracePDRTransformedRules(rules *base.RuleSet) error {
	tags := e.manager.BorrowSourceTraceTags()
	if !tags.ShouldTrace("pdr") {
		return nil
	}
	outputs, err := rules.OutputPredicates()
	if err != nil || len(outputs) == 0 {
		return err
	}
	err = tags.EmitConditional("pdr", "query", "src/muz/pdr/pdr_dl_interface.cpp", 144,
		func() bool { return true }, func(out io.Writer) (resultErr error) {
			writer := pdrTraceOperandWriter{out: out}
			_, _ = io.WriteString(writer, "rules:\n")
			if err := rules.WriteDisplay(writer); err != nil {
				return err
			}
			parameters, err := e.fixedpointParamView()
			if err != nil {
				return err
			}
			defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
			config := base.DefaultContextSMT2Config()
			config.FixedpointExtensions, err = parameters.Bool("print_fixedpoint_extensions", true)
			if err != nil {
				return err
			}
			config.LowLevel, err = parameters.Bool("print_low_level_smt2", false)
			if err != nil {
				return err
			}
			config.DeclareVariables, err = parameters.Bool("print_with_variable_declarations", true)
			if err != nil {
				return err
			}
			for _, ref := range e.assertions {
				h, err := e.manager.RefGet(ref)
				if err != nil {
					return err
				}
				config.Background = append(config.Background, h)
			}
			if len(e.assertions) == 0 {
				h, err := e.manager.RefGet(e.background)
				if err != nil {
					return err
				}
				if !e.manager.IsTrue(h) {
					config.Background = append(config.Background, h)
				}
			}
			return rules.WriteContextSMT2(writer, config)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
