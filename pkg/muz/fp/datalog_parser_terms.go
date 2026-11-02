package fp

// Source: dparser::parse_arg, mk_const and mk_symbol_const.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *datalogParser) parseArguments(declaration ast.ASTHandle) ([]ast.ASTHandle, error) {
	if p.token.kind != datalogLeftParen {
		arity, err := p.manager.GetArity(declaration)
		if err != nil {
			return nil, err
		}
		if arity != 0 {
			return nil, p.fail("too few arguments passed to predicate")
		}
		return nil, nil
	}
	p.advance()
	var arguments []ast.ASTHandle
	for p.token.kind != datalogRightParen {
		index := uint32(len(arguments))
		arity, err := p.manager.GetArity(declaration)
		if err != nil {
			return nil, err
		}
		if index >= arity {
			return nil, p.fail("too many arguments passed to predicate")
		}
		sort, err := p.manager.GetDomain(declaration, index)
		if err != nil {
			return nil, err
		}
		argument, err := p.parseArgument(sort)
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, argument)
		if p.token.kind == datalogRightParen {
			break
		}
		if err = p.expect(datalogComma, "expected ',' or ')' in predicate arguments"); err != nil {
			return nil, err
		}
	}
	p.advance()
	arity, err := p.manager.GetArity(declaration)
	if err != nil {
		return nil, err
	}
	if uint32(len(arguments)) != arity {
		return nil, p.fail("too few arguments passed to predicate")
	}
	return arguments, nil
}

func (p *datalogParser) parseArgument(sort ast.ASTHandle) (ast.ASTHandle, error) {
	token := p.token
	p.advance()
	return p.argumentFromToken(token, sort, true)
}

// argumentFromToken implements dparser::parse_arg and the constant/variable
// operands shared with parse_infix. Infix parsing disables creation so an
// identifier must name a variable typed by an earlier predicate occurrence.
func (p *datalogParser) argumentFromToken(token datalogToken, sort ast.ASTHandle, createVariable bool) (ast.ASTHandle, error) {
	switch token.kind {
	case datalogWildcard:
		if !createVariable {
			return ast.ASTHandle{}, p.fail("wildcard is not an infix operand")
		}
		result, err := p.manager.MkVar(p.nextVar, sort)
		p.nextVar++
		return result, err
	case datalogIdentifier:
		if prior, exists := p.vars[token.text]; exists {
			priorSort, err := p.manager.GetSort(prior)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if priorSort != sort {
				return ast.ASTHandle{}, p.fail("variable used with inconsistent sorts")
			}
			return prior, nil
		}
		if !createVariable {
			return ast.ASTHandle{}, p.fail("infix relation requires a previously typed variable")
		}
		result, err := p.manager.MkVar(p.nextVar, sort)
		if err == nil {
			p.nextVar++
			p.vars[token.text] = result
		}
		return result, err
	case datalogString:
		finite, err := p.manager.IsDatalogFiniteSort(sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if finite {
			name, err := p.symbol(token.text)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			number, err := p.context.SymbolNumber(sort, name)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return p.manager.MkDatalogNumeral(number, sort)
		}
		if _, err = strconv.ParseUint(token.text, 10, 64); err != nil {
			return ast.ASTHandle{}, p.fail("invalid integer")
		}
		return p.manager.ArithmeticIntegerNumeralText(token.text)
	case datalogNumber:
		_, err := strconv.ParseUint(token.text, 10, 64)
		if err != nil {
			return ast.ASTHandle{}, p.fail("invalid integer")
		}
		finite, err := p.manager.IsDatalogFiniteSort(sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if finite {
			name, err := p.symbol(token.text)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			number, err := p.context.SymbolNumber(sort, name)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return p.manager.MkDatalogNumeral(number, sort)
		}
		return p.manager.ArithmeticIntegerNumeralText(token.text)
	default:
		return ast.ASTHandle{}, p.fail("expected predicate argument")
	}
}
