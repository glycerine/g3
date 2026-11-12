package smt2

// Source: src/parsers/smt2/smt2parser.cpp parse_indexed_identifier_core and
// parse_func_decl_ref. Retained applications include array map and as-array.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

type identifierIndex struct {
	number   uint32
	function ast.ASTHandle
}

func (p *commandParser) indexedExpression() (string, []identifierIndex, error) {
	if p.token != SymbolToken {
		return "", nil, errors.New("invalid indexed identifier, symbol expected")
	}
	name, err := p.symbol()
	if err != nil {
		return "", nil, err
	}
	var indices []identifierIndex
	for p.token != RightParenToken {
		var index identifierIndex
		switch p.token {
		case IntegerToken:
			index.number, err = p.unsigned()
		case SymbolToken, LeftParenToken:
			index.function, err = p.functionDeclarationReference()
		default:
			err = fmt.Errorf("invalid indexed identifier, integer, identifier or '(' expected")
		}
		if err != nil {
			return "", nil, err
		}
		indices = append(indices, index)
	}
	if len(indices) == 0 {
		return "", nil, fmt.Errorf("invalid indexed identifier, index expected")
	}
	return name, indices, p.advance()
}

func (p *commandParser) functionDeclarationReference() (ast.ASTHandle, error) {
	if p.token == SymbolToken {
		// find_func_decl runs before next(), so lookup errors carry the
		// identifier's own position.
		declaration, err := p.pin(p.context.FindFunctionDeclaration(p.scanner.ID(), nil, nil, ast.ASTHandle{}))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return declaration, p.advance()
	}
	const refExpected = "invalid function declaration reference, symbol or '(' expected"
	if p.token != LeftParenToken {
		return ast.ASTHandle{}, errors.New(refExpected)
	}
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	var name string
	var indices []uint32
	var err error
	if p.token == SymbolToken {
		name, err = p.symbol()
		if err != nil {
			return ast.ASTHandle{}, err
		}
	} else {
		if p.token != LeftParenToken {
			return ast.ASTHandle{}, errors.New(refExpected)
		}
		if err = p.advance(); err != nil {
			return ast.ASTHandle{}, err
		}
		if p.token != SymbolToken || p.scanner.ID() != "_" {
			return ast.ASTHandle{}, errors.New("invalid indexed function declaration reference, '_' expected")
		}
		if err = p.advance(); err != nil {
			return ast.ASTHandle{}, err
		}
		if p.token != SymbolToken {
			return ast.ASTHandle{}, errors.New("invalid indexed function declaration reference, symbol expected")
		}
		if name, err = p.symbol(); err != nil {
			return ast.ASTHandle{}, err
		}
		for p.token != RightParenToken {
			if p.token != IntegerToken {
				return ast.ASTHandle{}, errors.New("invalid indexed function declaration reference, integer or ')' expected")
			}
			value, err := p.unsigned()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			indices = append(indices, value)
		}
		if len(indices) == 0 {
			return ast.ASTHandle{}, errors.New("invalid indexed function declaration reference, index expected")
		}
		if err = p.advance(); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	// parse_sorts(context) and parse_sort("Invalid function name").
	const sortsContext = "Invalid function name. Expecting sort list startig with '(' to disambiguate function name"
	if p.token != LeftParenToken {
		return ast.ASTHandle{}, errors.New(sortsContext)
	}
	if err = p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, 0)
	for p.token != RightParenToken {
		sort, err := p.sortIn(sortsContext)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		domain = append(domain, sort)
	}
	if err = p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	rangeSort, err := p.sortIn("Invalid function name")
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := p.pin(p.context.FindFunctionDeclaration(name, indices, domain, rangeSort))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if p.token != RightParenToken {
		return ast.ASTHandle{}, errors.New("invalid function declaration reference, ')' expected")
	}
	return declaration, p.advance()
}
