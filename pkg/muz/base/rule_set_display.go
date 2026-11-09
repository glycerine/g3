package base

// Source: src/muz/base/dl_rule_set.cpp rule_set::display.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// WriteDisplay writes the rule-set diagnostic in m_head2rules table order. The
// source defaults to profile output disabled and a zero timing threshold.
func (s *RuleSet) WriteDisplay(out io.Writer) error {
	return s.WriteDisplayWithProfile(out, false, 0)
}

// WriteDisplayWithProfile writes the source Datalog rule listing with its
// profile annotation and millisecond threshold controls.
func (s *RuleSet) WriteDisplayWithProfile(out io.Writer, outputProfile bool, profileTimeoutMilliseconds uint32) error {
	if out == nil {
		return io.ErrClosedPipe
	}
	if err := s.open(); err != nil {
		return err
	}
	predicates := 0
	var heads []ast.ASTHandle
	if s.headTable != nil {
		predicates = s.headTable.Len()
		for _, entry := range s.headTable.Entries() {
			heads = append(heads, entry.Key)
		}
	}
	if _, err := fmt.Fprintf(out, "; rule count: %d\n; predicate count: %d\n", len(s.rules), predicates); err != nil {
		return err
	}
	// m_output_preds is a func_decl_set: its lines follow table order.
	outputs := ast.NewObjMap(s.manager.manager)
	for _, declaration := range s.outputOrder {
		if err := outputs.Insert(declaration, [2]ast.ASTHandle{}); err != nil {
			return err
		}
	}
	for _, entry := range outputs.Entries() {
		declaration := entry.Key
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
	for _, declaration := range heads {
		for _, rule := range s.byHead[declaration] {
			ruleState, err := rule.open()
			if err != nil {
				return err
			}
			if ruleState.manager != s.manager.manager {
				return ErrRuleOwner
			}
			passes, err := ruleState.accounting.PassesOutputThreshold(profileTimeoutMilliseconds)
			if err != nil {
				return err
			}
			if !passes {
				continue
			}
			if err := s.manager.WriteRuleDisplayWithProfile(out, rule, outputProfile); err != nil {
				return err
			}
		}
	}
	return nil
}
