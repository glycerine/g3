package smt2

// Source: src/parsers/smt2/smt2parser.cpp parse_expr, parse_expr_name,
// parse_sorted_vars and the quantifier/let frame reductions.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	usexpr "github.com/glycerine/g3/pkg/util/sexpr"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (p *commandParser) rootObjectExpression() (result ast.ASTHandle, resultErr error) {
	manager := p.manager
	if manager == nil {
		return ast.ASTHandle{}, fmt.Errorf("root-obj requires an initialized AST manager")
	}
	numerals := manager.NumeralService()
	if numerals == nil {
		return ast.ASTHandle{}, fmt.Errorf("root-obj requires arithmetic numerals")
	}
	sexprs, err := usexpr.NewSexprManager(usexpr.SexprConfig{Budget: manager.Budget(), Symbols: manager.Symbols, Rationals: numerals})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, sexprs.Close()) }()
	polynomialExpr, err := p.parseRootObjectSExpr(sexprs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if p.token != IntegerToken {
		return ast.ASTHandle{}, fmt.Errorf("invalid root-obj, (unsigned) integer expected")
	}
	indexValue := p.scanner.Number()
	if !indexValue.IsInt() || indexValue.Sign() < 0 || !indexValue.Num().IsUint64() || indexValue.Num().Uint64() > uint64(^uint32(0)) {
		return ast.ASTHandle{}, fmt.Errorf("invalid root-obj, index must fit in an unsigned machine integer")
	}
	index := uint32(indexValue.Num().Uint64())
	if index == 0 {
		return ast.ASTHandle{}, fmt.Errorf("invalid root-obj, index must be >= 1")
	}
	if err = p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.expect(RightParenToken); err != nil {
		return ast.ASTHandle{}, err
	}
	algebraic, err := manager.ArithmeticAlgebraicManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value := polynomial.Algebraic{}
	defer func() { resultErr = errors.Join(resultErr, algebraic.Del(&value)) }()
	if err = algebraic.MkSexprRoot(sexprs, polynomialExpr, index, &value); err != nil {
		return ast.ASTHandle{}, err
	}
	rational, err := algebraic.IsRational(&value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if rational {
		numeral, e := numerals.NewValue()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		defer func() { resultErr = errors.Join(resultErr, numerals.CleanupValue(&numeral)) }()
		if err = algebraic.ToRational(&value, &numeral.Raw); err != nil {
			return ast.ASTHandle{}, err
		}
		result, err = manager.ArithmeticNumeral(&numeral, false)
		return result, err
	}
	return manager.ArithmeticAlgebraicNumeral(&value)
}

func (p *commandParser) parseRootObjectSExpr(manager *usexpr.SexprManager) (result usexpr.SexprHandle, resultErr error) {
	line, pos := uint32(p.scanner.Line()), uint32(p.scanner.Position())
	if p.token == LeftParenToken {
		if err := p.advance(); err != nil {
			return usexpr.SexprHandle{}, err
		}
		children := make([]usexpr.SexprHandle, 0, 4)
		for p.token != RightParenToken {
			if p.token == EOFToken {
				return usexpr.SexprHandle{}, fmt.Errorf("invalid s-expression, unexpected end of file")
			}
			child, err := p.parseRootObjectSExpr(manager)
			if err != nil {
				return usexpr.SexprHandle{}, err
			}
			children = append(children, child)
		}
		if len(children) == 0 {
			return usexpr.SexprHandle{}, fmt.Errorf("invalid empty s-expression")
		}
		result, err := manager.MkComposite(children, line, pos)
		if err != nil {
			return usexpr.SexprHandle{}, err
		}
		return result, p.advance()
	}
	switch p.token {
	case SymbolToken:
		name, err := p.manager.Symbols.Intern(p.scanner.ID())
		if err != nil {
			return usexpr.SexprHandle{}, err
		}
		result, err := manager.MkSymbol(name, line, pos)
		if err != nil {
			return usexpr.SexprHandle{}, err
		}
		return result, p.advance()
	case KeywordToken:
		name, err := p.manager.Symbols.Intern(p.scanner.ID())
		if err != nil {
			return usexpr.SexprHandle{}, err
		}
		result, err := manager.MkKeyword(name, line, pos)
		if err != nil {
			return usexpr.SexprHandle{}, err
		}
		return result, p.advance()
	case StringToken:
		result, err := manager.MkString(p.scanner.String(), line, pos)
		if err != nil {
			return usexpr.SexprHandle{}, err
		}
		return result, p.advance()
	case IntegerToken, DecimalToken:
		numeral, err := p.manager.NumeralService().NewValue()
		if err != nil {
			return usexpr.SexprHandle{}, err
		}
		defer func() { resultErr = errors.Join(resultErr, p.manager.NumeralService().CleanupValue(&numeral)) }()
		if err = p.manager.NumeralService().SetString(&numeral, p.scanner.Number().RatString()); err != nil {
			return usexpr.SexprHandle{}, err
		}
		result, err := manager.MkNumeral(&numeral, line, pos)
		if err != nil {
			return usexpr.SexprHandle{}, err
		}
		return result, p.advance()
	case BitVectorToken:
		numeral, err := p.manager.NumeralService().NewValue()
		if err != nil {
			return usexpr.SexprHandle{}, err
		}
		defer func() { resultErr = errors.Join(resultErr, p.manager.NumeralService().CleanupValue(&numeral)) }()
		if err = p.manager.NumeralService().SetString(&numeral, p.scanner.Number().RatString()); err != nil {
			return usexpr.SexprHandle{}, err
		}
		result, err := manager.MkBVNumeral(&numeral, p.scanner.BitVectorSize(), line, pos)
		if err != nil {
			return usexpr.SexprHandle{}, err
		}
		return result, p.advance()
	default:
		return usexpr.SexprHandle{}, fmt.Errorf("invalid s-expression, unexpected input")
	}
}

func (p *commandParser) qualifiedLocal(name string, indices []identifierIndex, expectedSort ast.ASTHandle, hasSort bool) (value ast.ASTHandle, found bool, resultErr error) {
	symbol, err := p.manager.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	ref, found, err := p.env.resolveQualifiedLocalSort(p.manager, p.shifter, symbol, p.bindings, expectedSort, hasSort, uint32(len(indices)))
	if err != nil || !found {
		return ast.ASTHandle{}, found, err
	}
	p.refs = append(p.refs, ref)
	value, err = p.manager.RefGet(ref)
	return value, true, err
}

func (p *commandParser) expression() (ast.ASTHandle, string, error) {
	return p.expressionWithQuantifier(nil)
}

// Only the immediate annotated body of a quantifier can update its metadata.
// Recursive argument, let-body and attribute-body expressions use expression.
func (p *commandParser) expressionWithQuantifier(quantifier *ast.ASTQuantifierSpec) (ast.ASTHandle, string, error) {
	m, err := p.astManager()
	if err != nil {
		return ast.ASTHandle{}, "", err
	}
	var result ast.ASTHandle
	switch p.token {
	case IntegerToken, DecimalToken:
		integer := p.token == IntegerToken && !p.context.NumeralsAsReal()
		result, err = m.ArithmeticNumeralRat(p.scanner.Number(), integer)
		if err == nil {
			err = p.advance()
		}
	case BitVectorToken:
		result, err = m.BitVectorNumeralRat(p.scanner.Number(), p.scanner.BitVectorSize())
		if err == nil {
			err = p.advance()
		}
	case StringToken:
		result, err = m.MkString(p.scanner.String())
		if err == nil {
			err = p.advance()
		}
	case SymbolToken:
		name, e := p.symbol()
		if e != nil {
			return ast.ASTHandle{}, "", e
		}
		symbol, e := m.Symbols.Intern(name)
		if e != nil {
			return ast.ASTHandle{}, "", e
		}
		ref, found, e := p.env.resolveLocal(m, p.shifter, symbol, p.bindings)
		if e != nil {
			return ast.ASTHandle{}, "", e
		}
		if found {
			p.refs = append(p.refs, ref)
			result, err = m.RefGet(ref)
			return result, "", err
		}
		result, err = p.indexedApplication(name, nil, nil, ast.ASTHandle{})
	case LeftParenToken:
		if err := p.advance(); err != nil {
			return ast.ASTHandle{}, "", err
		}
		var name string
		var indices []identifierIndex
		var rangeSort ast.ASTHandle
		if p.token == LeftParenToken {
			if err := p.advance(); err != nil {
				return ast.ASTHandle{}, "", err
			}
			head, err := p.symbol()
			if err != nil {
				return ast.ASTHandle{}, "", err
			}
			if head == "_" {
				name, indices, err = p.indexedExpression()
			} else if head == "as" {
				name, indices, err = p.qualifiedIdentifier()
				if err == nil {
					rangeSort, err = p.sort()
				}
				if err == nil {
					err = p.expect(RightParenToken)
				}
			} else {
				err = fmt.Errorf("qualified identifier expected")
			}
			if err != nil {
				return ast.ASTHandle{}, "", err
			}
		} else {
			name, err = p.symbol()
			if err != nil {
				return ast.ASTHandle{}, "", err
			}
			switch name {
			case "root-obj":
				result, err = p.rootObjectExpression()
				if err != nil {
					return ast.ASTHandle{}, "", err
				}
				result, err = p.pin(result, nil)
				return result, "", err
			case "forall", "exists":
				return p.quantifier(name == "forall")
			case "let":
				return p.letExpression()
			case "!":
				return p.annotatedExpression(quantifier)
			case "_":
				name, indices, err := p.indexedExpression()
				if err != nil {
					return ast.ASTHandle{}, "", err
				}
				if value, found, localErr := p.qualifiedLocal(name, indices, ast.ASTHandle{}, false); localErr != nil || found {
					return value, "", localErr
				}
				var value ast.ASTHandle
				if _, numeral := parseBitVectorName(name); numeral {
					value, err = p.parseBVNumeral(name, indices)
				} else {
					value, err = p.indexedApplication(name, indices, nil, ast.ASTHandle{})
				}
				return value, "", err
			case "as":
				name, indices, err := p.qualifiedIdentifier()
				if err != nil {
					return ast.ASTHandle{}, "", err
				}
				rangeSort, err := p.sort()
				if err != nil {
					return ast.ASTHandle{}, "", err
				}
				if err = p.expect(RightParenToken); err != nil {
					return ast.ASTHandle{}, "", err
				}
				if value, found, localErr := p.qualifiedLocal(name, indices, rangeSort, true); localErr != nil || found {
					return value, "", localErr
				}
				if _, ok := parseBitVectorName(name); ok {
					if len(indices) != 1 || !indices[0].function.IsNull() {
						return ast.ASTHandle{}, "", fmt.Errorf("invalid bit-vector constant, index expected")
					}
					value, e := p.parseBVNumeral(name, indices)
					if e != nil {
						return ast.ASTHandle{}, "", e
					}
					actual, e := m.GetSort(value)
					if e != nil {
						return ast.ASTHandle{}, "", e
					}
					if actual != rangeSort {
						return ast.ASTHandle{}, "", fmt.Errorf("invalid qualified identifier, sort mismatch")
					}
					return value, "", nil
				}
				value, err := p.indexedApplication(name, indices, nil, rangeSort)
				return value, "", err
			}
		}
		var args []ast.ASTHandle
		for p.token != RightParenToken {
			arg, _, err := p.expression()
			if err != nil {
				return ast.ASTHandle{}, "", err
			}
			args = append(args, arg)
		}
		if err := p.advance(); err != nil {
			return ast.ASTHandle{}, "", err
		}
		result, err = p.indexedApplication(name, indices, args, rangeSort)
		return result, "", err
	default:
		return ast.ASTHandle{}, "", fmt.Errorf("invalid expression, unexpected input")
	}
	result, err = p.pin(result, err)
	return result, "", err
}

func (p *commandParser) sortedVariables() ([]usymbol.Symbol, []ast.ASTHandle, error) {
	if err := p.expect(LeftParenToken); err != nil {
		return nil, nil, err
	}
	var names []usymbol.Symbol
	var sorts []ast.ASTHandle
	for p.token != RightParenToken {
		if err := p.expect(LeftParenToken); err != nil {
			return nil, nil, err
		}
		name, err := p.symbol()
		if err != nil {
			return nil, nil, err
		}
		symbol, err := p.manager.Symbols.Intern(name)
		if err != nil {
			return nil, nil, err
		}
		sort, err := p.sort()
		if err != nil {
			return nil, nil, err
		}
		if err = p.expect(RightParenToken); err != nil {
			return nil, nil, err
		}
		names = append(names, symbol)
		sorts = append(sorts, sort)
	}
	return names, sorts, p.advance()
}

// Source parse_define_fun / parse_define_const: unlike argument coercions,
// the declared return sort must equal the body sort exactly.
func (p *commandParser) defineFunction(withArguments bool) (result error) {
	if _, err := p.astManager(); err != nil {
		return err
	}
	name, err := p.symbol()
	if err != nil {
		return err
	}
	var names []usymbol.Symbol
	var sorts []ast.ASTHandle
	if withArguments {
		names, sorts, err = p.sortedVariables()
		if err != nil {
			return err
		}
	}
	oldBindings, oldExprs := p.bindings, len(p.locals.refs)
	p.bindings, err = p.env.bindSortedVariables(p.manager, names, sorts, p.bindings, &p.locals)
	if err != nil {
		return err
	}
	defer func() {
		p.bindings = oldBindings
		result = errors.Join(result, p.env.endScopeWithOwningCleanup(), p.locals.shrink(p.manager, oldExprs))
	}()
	rangeSort, err := p.sort()
	if err != nil {
		return err
	}
	body, _, err := p.expression()
	if err != nil {
		return err
	}
	actual, err := p.manager.GetSort(body)
	if err != nil {
		return err
	}
	if actual != rangeSort {
		return fmt.Errorf("invalid function/constant definition, sort mismatch")
	}
	if err = p.requireEnd(); err != nil {
		return err
	}
	return p.context.DefineFunction(name, sorts, body)
}

func (p *commandParser) quantifier(forall bool) (value ast.ASTHandle, named string, result error) {
	names, sorts, err := p.sortedVariables()
	if err != nil {
		return ast.ASTHandle{}, "", err
	}
	oldBindings, oldExprs := p.bindings, len(p.locals.refs)
	p.bindings, err = p.env.bindSortedVariables(p.manager, names, sorts, p.bindings, &p.locals)
	if err != nil {
		return ast.ASTHandle{}, "", err
	}
	defer func() {
		p.bindings = oldBindings
		result = errors.Join(result, p.env.endScopeWithOwningCleanup(), p.locals.shrink(p.manager, oldExprs))
	}()
	spec := ast.ASTQuantifierSpec{Forall: forall, Sorts: sorts, Names: names, Weight: 1}
	body, _, err := p.expressionWithQuantifier(&spec)
	if err != nil {
		return ast.ASTHandle{}, "", err
	}
	spec.Body = body
	if err = p.validateQuantifierPatterns(&spec); err != nil {
		return ast.ASTHandle{}, "", err
	}
	if spec.QID.IsNull() {
		spec.QID = usymbol.Numerical(uint32(p.scanner.Line()))
	}
	if err = p.expect(RightParenToken); err != nil {
		return ast.ASTHandle{}, "", err
	}
	value, err = p.pin(p.manager.MkQuantifier(spec))
	return value, "", err
}

func (p *commandParser) letExpression() (value ast.ASTHandle, named string, result error) {
	if err := p.expect(LeftParenToken); err != nil {
		return ast.ASTHandle{}, "", err
	}
	start := len(p.locals.refs)
	defer func() { result = errors.Join(result, p.locals.shrink(p.manager, start)) }()
	var names []usymbol.Symbol
	for p.token != RightParenToken {
		if err := p.expect(LeftParenToken); err != nil {
			return ast.ASTHandle{}, "", err
		}
		name, err := p.symbol()
		if err != nil {
			return ast.ASTHandle{}, "", err
		}
		symbol, err := p.manager.Symbols.Intern(name)
		if err != nil {
			return ast.ASTHandle{}, "", err
		}
		term, _, err := p.expression()
		if err != nil {
			return ast.ASTHandle{}, "", err
		}
		if err = p.expect(RightParenToken); err != nil {
			return ast.ASTHandle{}, "", err
		}
		ref, err := p.manager.NewRef(term)
		if err != nil {
			return ast.ASTHandle{}, "", err
		}
		p.locals.pushOwned(ref)
		names = append(names, symbol)
	}
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, "", err
	}
	if err := p.env.bindLetLocals(p.manager, names, &p.locals, start, p.bindings); err != nil {
		return ast.ASTHandle{}, "", err
	}
	defer func() { result = errors.Join(result, p.env.endScopeWithOwningCleanup()) }()
	value, named, result = p.expression()
	if result != nil {
		return value, named, result
	}
	result = p.expect(RightParenToken)
	return value, named, result
}
