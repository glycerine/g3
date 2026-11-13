package smt2

// Source: src/parsers/smt2/smt2parser.cpp consume_attributes,
// push_pattern_frame, process_last_symbol and pop_quant_frame.

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	parserutil "github.com/glycerine/g3/pkg/parsers/util"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (p *commandParser) configurePatternOptions() (result error) {
	globals, err := p.context.GlobalParameters()
	if err != nil {
		return err
	}
	params, err := parserutil.NewParserParams(globals.Context, globals, nil)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, params.Close()) }()
	p.ignoreUserPatterns, err = params.IgnoreUserPatterns()
	if err != nil {
		return err
	}
	p.ignoreBadPatterns, err = params.IgnoreBadPatterns()
	return err
}

func (p *commandParser) annotatedExpression(quantifier *ast.ASTQuantifierSpec) (ast.ASTHandle, string, error) {
	body, named, err := p.expression()
	if err != nil {
		return ast.ASTHandle{}, "", err
	}
	for p.token == KeywordToken {
		key := p.scanner.ID()
		if err = p.advance(); err != nil {
			return ast.ASTHandle{}, "", err
		}
		quantifierAttribute := key == ":weight" || key == ":qid" || key == ":skolemid" || key == ":pattern" || key == ":no-pattern"
		if quantifierAttribute && !(p.ignoreUserPatterns && (key == ":pattern" || key == ":no-pattern")) && quantifier == nil {
			return ast.ASTHandle{}, "", fmt.Errorf("invalid attribute, not in the scope of a quantifier")
		}
		switch key {
		case ":named":
			named, err = p.symbol()
			if err == nil {
				var free bool
				free, err = p.manager.HasFreeVars(body)
				if err == nil && free {
					err = fmt.Errorf("invalid named expression, expression contains free variables")
				}
			}
			if err == nil {
				err = p.context.DefineFunction(named, nil, body)
			}
		case ":lblpos", ":lblneg":
			var name string
			name, err = p.symbol()
			if err == nil {
				var boolean ast.ASTHandle
				boolean, err = p.manager.MkBoolSort()
				if err == nil {
					var sort ast.ASTHandle
					sort, err = p.manager.GetSort(body)
					if err == nil && sort != boolean {
						err = fmt.Errorf("invalid labeled expression, expression must have Bool sort")
					}
				}
			}
			var label usymbol.Symbol
			if err == nil {
				label, err = p.manager.Symbols.Intern(name)
			}
			if err == nil {
				body, err = p.pinExpression(p.manager.MkLabel(key == ":lblpos", []usymbol.Symbol{label}, body))
			}
		case ":weight":
			var weight uint32
			weight, err = p.unsigned()
			quantifier.Weight = int32(weight)
		case ":qid", ":skolemid":
			var name string
			name, err = p.symbol()
			if err == nil {
				if key == ":qid" {
					quantifier.QID, err = p.manager.Symbols.Intern(name)
				} else {
					quantifier.SkolemID, err = p.manager.Symbols.Intern(name)
				}
			}
		case ":pattern", ":no-pattern":
			if p.ignoreUserPatterns {
				err = p.skipSExpr()
			} else if key == ":pattern" {
				var pattern ast.ASTHandle
				pattern, err = p.pattern()
				if err == nil {
					if pattern.IsNull() {
						if !p.ignoreBadPatterns {
							err = fmt.Errorf("invalid empty pattern")
						}
					} else {
						quantifier.Patterns = append(quantifier.Patterns, pattern)
					}
				}
			} else {
				var term ast.ASTHandle
				start := len(p.exprs.refs)
				term, _, err = p.expression()
				if err == nil {
					_, err = p.pin(term, nil)
					if err == nil {
						quantifier.NoPatterns = append(quantifier.NoPatterns, term)
					}
				}
				err = errors.Join(err, p.exprs.shrink(p.manager, start))
			}
		default:
			if err = diagnostic.BorrowProcessWarningService().Warning(fmt.Sprintf("unknown attribute %s", key)); err == nil {
				err = p.skipSExpr()
			}
		}
		if err != nil {
			return ast.ASTHandle{}, "", err
		}
	}
	if err = p.expect(RightParenToken); err != nil {
		return ast.ASTHandle{}, "", err
	}
	return body, named, nil
}

func (p *commandParser) pattern() (result ast.ASTHandle, resultErr error) {
	start := len(p.exprs.refs)
	defer func() { resultErr = errors.Join(resultErr, p.exprs.shrink(p.manager, start)) }()
	if p.token != LeftParenToken {
		if !p.ignoreBadPatterns {
			return ast.ASTHandle{}, fmt.Errorf("invalid pattern, '(' expected")
		}
		return ast.ASTHandle{}, p.skipSExpr()
	}
	if err := p.advance(); err != nil {
		return ast.ASTHandle{}, err
	}
	if p.token == RightParenToken {
		return ast.ASTHandle{}, p.advance()
	}
	var terms []ast.ASTHandle
	if p.token == SymbolToken {
		isNullary, err := p.context.IsNullaryFunction(p.scanner.ID())
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if isNullary {
			if !p.ignoreBadPatterns {
				return ast.ASTHandle{}, fmt.Errorf("invalid constant pattern")
			}
			for p.token != RightParenToken {
				if err = p.skipSExpr(); err != nil {
					return ast.ASTHandle{}, err
				}
			}
			return ast.ASTHandle{}, p.advance()
		}
		// Retain the source's older unary syntax :pattern (f x).
		name, err := p.symbol()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		var arguments []ast.ASTHandle
		for p.token != RightParenToken {
			term, _, err := p.expression()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			arguments = append(arguments, term)
		}
		term, err := p.indexedApplication(name, nil, arguments, ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		terms = append(terms, term)
	} else {
		for p.token != RightParenToken {
			term, _, err := p.expression()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			terms = append(terms, term)
		}
	}
	if err := p.expect(RightParenToken); err != nil {
		return ast.ASTHandle{}, err
	}
	valid := len(terms) != 0
	for _, term := range terms {
		kind, err := p.manager.GetKind(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		valid = valid && kind == ast.ASTApp
	}
	if !valid {
		if p.ignoreBadPatterns {
			return ast.ASTHandle{}, nil
		}
		return ast.ASTHandle{}, fmt.Errorf("invalid empty or non-application pattern")
	}
	return p.pin(p.manager.MkPattern(terms))
}

func (p *commandParser) validateQuantifierPatterns(spec *ast.ASTQuantifierSpec) (result error) {
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return err
	}
	sort, err := p.manager.GetSort(spec.Body)
	if err != nil {
		return err
	}
	if sort != boolean {
		return fmt.Errorf("quantifier body must be a Boolean expression")
	}
	var warnings bytes.Buffer
	validator, err := parserutil.NewPatternValidator(p.manager, &warnings)
	if err != nil {
		return err
	}
	// pop_quant_frame compacts pattern_stack() in place: set(j, pat) and
	// the final shrink drop stack references, and a rejected pattern dies
	// when its last one goes, before mk_quantifier. The parser holds one pin
	// per parsed pattern; held tracks the stack references of each handle.
	slots := append([]ast.ASTHandle(nil), spec.Patterns...)
	held := make(map[ast.ASTHandle]int, len(slots))
	pins := make(map[ast.ASTHandle]int, len(slots))
	for _, pattern := range slots {
		held[pattern]++
		pins[pattern]++
	}
	drop := func(pattern ast.ASTHandle) error {
		held[pattern]--
		for pins[pattern] > held[pattern] {
			pins[pattern]--
			if err := p.unpinPattern(pattern); err != nil {
				return err
			}
		}
		return nil
	}
	j := 0
	defer func() {
		if result != nil {
			return
		}
		for k := j; k < len(slots); k++ {
			if result = drop(slots[k]); result != nil {
				return
			}
		}
	}()
	accepted := spec.Patterns[:0]
	for _, pattern := range spec.Patterns {
		valid, err := validator.ValidateNew(uint32(len(spec.Sorts)), pattern, uint32(p.scanner.Line()), uint32(p.scanner.Position()))
		if err != nil {
			return err
		}
		if warnings.Len() != 0 {
			for _, message := range strings.Split(strings.TrimSuffix(warnings.String(), "\n"), "\n") {
				if err = diagnostic.BorrowProcessWarningService().Warning(message); err != nil {
					return err
				}
			}
			warnings.Reset()
		}
		if !valid {
			if !p.ignoreBadPatterns {
				return fmt.Errorf("invalid pattern")
			}
			continue
		}
		if old := slots[j]; old != pattern {
			slots[j] = pattern
			held[pattern]++
			if err = drop(old); err != nil {
				return err
			}
		}
		j++
		accepted = append(accepted, pattern)
	}
	spec.Patterns = accepted
	return nil
}

// unpinPattern drops the parser's pin of a pattern removed from the stack.
func (p *commandParser) unpinPattern(pattern ast.ASTHandle) error {
	for i := len(p.refs) - 1; i >= 0; i-- {
		held, err := p.manager.RefGet(p.refs[i])
		if err != nil {
			return err
		}
		if held != pattern {
			continue
		}
		ref := p.refs[i]
		p.refs = append(p.refs[:i], p.refs[i+1:]...)
		return p.manager.DelRef(&ref)
	}
	return nil
}
