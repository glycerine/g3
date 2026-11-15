package base

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
)

func (s *RuleSet) Strata() ([][]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if !s.finalized {
		return nil, fmt.Errorf("muz/base: finalize rule set before reading strata")
	}
	// rule_stratifier::get_strats, verbatim.
	result := make([][]ast.ASTHandle, len(s.strats))
	for i := range s.strats {
		result[i] = append([]ast.ASTHandle(nil), s.strats[i]...)
	}
	return result, nil
}
