package base

// Source: src/muz/base/dl_rule_set.cpp rule_set::display.

import (
	"fmt"
	"io"
)

// WriteDisplay writes the rule-set diagnostic in grouped-head order. The
// source defaults to profile output disabled and a zero timing threshold.
func (s *RuleSet) WriteDisplay(out io.Writer) error {
	if out == nil {
		return io.ErrClosedPipe
	}
	if err := s.open(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "; rule count: %d\n; predicate count: %d\n", len(s.rules), len(s.byHead)); err != nil {
		return err
	}
	for _, declaration := range s.outputOrder {
		name, err := s.manager.manager.GetASTName(declaration)
		if err != nil {
			return err
		}
		text, err := s.manager.manager.Symbols.String(name)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "; output: %s\n", text); err != nil {
			return err
		}
	}
	for _, declaration := range s.headOrder {
		for _, rule := range s.byHead[declaration] {
			if err := s.manager.WriteRuleDisplay(out, rule); err != nil {
				return err
			}
		}
	}
	return nil
}
