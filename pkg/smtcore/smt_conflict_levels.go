package smtcore

// Source: src/smt/smt_conflict_resolution.cpp:255-304.

import "github.com/glycerine/g3/pkg/ast"

// The public queries accept explicit levels. During search the assignment
// level already resides in the solver's Boolean variable record.
type smtAssignmentLevels struct {
	explicit  []uint32
	variables []SMTBoolVarData
}

func (s smtAssignmentLevels) level(literal SMTLiteral) (uint32, error) {
	v := literal.Var()
	if s.variables != nil {
		if v < 0 || int(v) >= len(s.variables) {
			return 0, ast.ErrASTDomain
		}
		return s.variables[v].ScopeLevel(), nil
	}
	if v < 0 || int(v) >= len(s.explicit) {
		return 0, ast.ErrASTDomain
	}
	return s.explicit[v], nil
}
func SMTJustificationMaxLevel(expander *SMTAntecedentExpander, justification SMTJustification, levels []uint32) (uint32, error) {
	return smtJustificationMaxLevel(expander, justification, smtAssignmentLevels{explicit: levels})
}

func smtJustificationMaxLevel(expander *SMTAntecedentExpander, justification SMTJustification, levels smtAssignmentLevels) (uint32, error) {
	if expander == nil || justification == nil {
		return 0, ast.ErrASTHandle
	}
	antecedents, err := expander.expandReusing(justification)
	if err != nil {
		return 0, err
	}
	var maximum uint32
	for _, literal := range antecedents {
		level, err := levels.level(literal)
		if err != nil {
			return 0, err
		}
		if level > maximum {
			maximum = level
		}
	}
	return maximum, nil
}

func SMTBooleanJustificationMaxLevel(expander *SMTAntecedentExpander, consequent SMTLiteral, justification SMTBooleanJustification, levels []uint32) (uint32, error) {
	return smtBooleanJustificationMaxLevel(expander, consequent, justification, smtAssignmentLevels{explicit: levels})
}

func smtBooleanJustificationMaxLevel(expander *SMTAntecedentExpander, consequent SMTLiteral, justification SMTBooleanJustification, levels smtAssignmentLevels) (uint32, error) {
	var maximum uint32
	if consequent != SMTFalseLiteral {
		level, err := levels.level(consequent)
		if err != nil {
			return 0, err
		}
		maximum = level
	}
	update := func(literal SMTLiteral) error {
		level, err := levels.level(literal)
		if err != nil {
			return err
		}
		if level > maximum {
			maximum = level
		}
		return nil
	}
	switch justification.Kind() {
	case SMTBooleanJustificationClause:
		clause, ok := justification.Clause()
		if !ok || clause == nil || len(clause.literals) < 2 {
			return 0, ast.ErrASTDomain
		}
		i := 0
		if consequent != SMTFalseLiteral {
			if clause.literals[0] == consequent {
				i = 1
			} else if clause.literals[1] == consequent {
				if err := update(clause.literals[0]); err != nil {
					return 0, err
				}
				i = 2
			} else {
				return 0, ast.ErrASTDomain
			}
		}
		for ; i < len(clause.literals); i++ {
			if err := update(clause.literals[i]); err != nil {
				return 0, err
			}
		}
		if j := clause.Justification(); j != nil {
			level, err := smtJustificationMaxLevel(expander, j, levels)
			if err != nil {
				return 0, err
			}
			if level > maximum {
				maximum = level
			}
		}
	case SMTBooleanJustificationBinaryClause:
		literal, ok := justification.Literal()
		if !ok {
			return 0, ast.ErrASTDomain
		}
		if err := update(literal); err != nil {
			return 0, err
		}
	case SMTBooleanJustificationAxiom:
	case SMTBooleanJustificationObject:
		object, ok := justification.Justification()
		if !ok || object == nil {
			return 0, ast.ErrASTDomain
		}
		level, err := smtJustificationMaxLevel(expander, object, levels)
		if err != nil {
			return 0, err
		}
		if level > maximum {
			maximum = level
		}
	default:
		return 0, ast.ErrASTDomain
	}
	return maximum, nil
}
