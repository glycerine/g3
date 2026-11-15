package smt2

// Source: src/parsers/smt2/smt2parser.cpp:2490-2546,
// parse_assumptions, parse_check_sat and parse_check_sat_assuming.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *commandParser) checkSatAssumptions(parenthesized bool) ([]ast.ASTHandle, error) {
	if parenthesized {
		// check_lparen_next("invalid check-sat-assuming command, '(', expected")
		if p.token != LeftParenToken {
			return nil, fmt.Errorf("invalid check-sat-assuming command, '(', expected")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	result, err := p.parseAssumptions()
	if err != nil {
		return nil, err
	}
	if parenthesized {
		// check_rparen_next("invalid check-sat-assuming command, ')', expected")
		if p.token != RightParenToken {
			return nil, fmt.Errorf("invalid check-sat-assuming command, ')', expected")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// parseAssumptions is parser::parse_assumptions: every assumption is a
// constant symbol or (not <symbol>), and must denote a Boolean literal.
func (p *commandParser) parseAssumptions() ([]ast.ASTHandle, error) {
	manager, err := p.astManager()
	if err != nil {
		return nil, err
	}
	var result []ast.ASTHandle
	for p.token != RightParenToken {
		var sign bool
		if p.token == LeftParenToken {
			if err = p.advance(); err != nil {
				return nil, err
			}
			// check_id_next(m_not, ...)
			if p.token != SymbolToken || p.scanner.ID() != "not" {
				return nil, fmt.Errorf("invalid check-sat command, 'not' expected, assumptions must be Boolean literals")
			}
			if err = p.advance(); err != nil {
				return nil, err
			}
			if p.token != SymbolToken {
				return nil, fmt.Errorf("invalid check-sat command, literal expected")
			}
			sign = true
		} else {
			if p.token != SymbolToken {
				return nil, fmt.Errorf("invalid check-sat command, literal or ')' expected")
			}
			sign = false
		}
		name := p.scanner.ID()
		if err = p.advance(); err != nil {
			return nil, err
		}
		// m_ctx.mk_const(n, t_ref) runs after next(): its errors carry the
		// following token's position.
		value, err := p.indexedApplication(name, nil, nil, ast.ASTHandle{})
		if err != nil {
			return nil, err
		}
		boolean, err := p.booleanExpression(value)
		if err != nil {
			return nil, err
		}
		if !boolean {
			return nil, fmt.Errorf("invalid check-sat command, argument must be a Boolean literal")
		}
		if sign {
			constant, err := parserIsUninterpConst(manager, value)
			if err != nil {
				return nil, err
			}
			if !constant {
				return nil, fmt.Errorf("invalid check-sat command, argument must be a Boolean literal")
			}
			if value, err = p.pin(manager.MkNot(value)); err != nil {
				return nil, err
			}
		} else {
			constant, err := parserIsUninterpConst(manager, value)
			if err != nil {
				return nil, err
			}
			if !constant {
				negation, arg, err := manager.IsNot(value)
				if err != nil {
					return nil, err
				}
				if negation {
					if constant, err = parserIsUninterpConst(manager, arg); err != nil {
						return nil, err
					}
				}
				if !constant {
					return nil, fmt.Errorf("invalid check-sat command, argument must be a Boolean literal")
				}
			}
		}
		result = append(result, value)
		if sign {
			// check_rparen_next("invalid check-sat command, ')' expected")
			if p.token != RightParenToken {
				return nil, fmt.Errorf("invalid check-sat command, ')' expected")
			}
			if err = p.advance(); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// parserIsUninterpConst is is_uninterp_const: an application with no
// arguments whose declaration has the null family.
func parserIsUninterpConst(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil || n != 0 {
		return false, err
	}
	decl, err := m.GetDecl(h)
	if err != nil {
		return false, err
	}
	family, err := m.GetFamilyID(decl)
	return family == ast.NullFamilyID, err
}

func (p *commandParser) booleanExpression(value ast.ASTHandle) (bool, error) {
	manager, err := p.astManager()
	if err != nil {
		return false, err
	}
	actual, err := manager.GetSort(value)
	if err != nil {
		return false, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	return actual == boolean, nil
}
