package fp

// Source: src/muz/pdr/pdr_dl_interface.cpp:68 check_reset's first fresh rule.

import (
	"errors"
	"io"
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
			if _, err = io.WriteString(out, "Fresh rule "); err != nil {
				return err
			}
			return e.rules.WriteRuleDisplay(out, rule)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
