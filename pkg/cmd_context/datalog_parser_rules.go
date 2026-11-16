package cmd_context

// Source: dparser::parse_decls, parse_rule, parse_body and add_rule.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *datalogParser) parseDecls(tok dtoken) (dtoken, error) {
	var err error
	for tok != dtkEOS && tok != dtkError {
		switch tok {
		case dtkID:
			tok, err = p.parseRule(tok)
		case dtkNewline:
			tok = p.lexer.nextToken()
		case dtkInclude:
			tok = p.lexer.nextToken()
			if tok != dtkString {
				tok, err = p.unexpected(tok, "a string")
				break
			}
			tok, err = p.parseInclude(p.lexer.tokenData(), false)
		default:
			tok, err = p.unexpected(tok, "identifier")
		}
		if err != nil {
			return tok, err
		}
	}
	return tok, nil
}

func (p *datalogParser) parseRule(tok dtoken) (_ dtoken, result error) {
	p.numVars = 0
	p.vars = make(map[string]ast.ASTHandle)

	switch tok {
	case dtkEOS:
		return tok, nil
	case dtkID:
		s := p.lexer.tokenData()
		tok = p.lexer.nextToken()
		// app_ref pred: released when parse_rule returns.
		tok, predRef, isPredicateDeclaration, err := p.parsePred(tok, s)
		defer func() { result = errors.Join(result, p.release(&predRef)) }()
		if err != nil {
			return tok, err
		}
		pred, err := p.manager.RefGet(predRef)
		if err != nil {
			return tok, err
		}
		switch tok {
		case dtkPeriod:
			if isPredicateDeclaration {
				return p.unexpected(tok, "predicate declaration should not end with '.'")
			}
			if err = p.addRule(pred, nil, nil); err != nil {
				return tok, err
			}
			return p.lexer.nextToken(), nil
		case dtkLeftArrow:
			return p.parseBody(pred)
		case dtkNewline, dtkEOS:
			if !isPredicateDeclaration {
				return p.unexpected(tok, "'.' expected at the end of rule")
			}
			return tok, nil
		default:
			return p.unexpected(tok, "unexpected token")
		}
	default:
		return p.unexpected(tok, "rule expected")
	}
}

func (p *datalogParser) parseBody(head ast.ASTHandle) (_ dtoken, result error) {
	// app_ref_vector body: released front to back when parse_body returns.
	var body []ast.ASTHandle
	var bodyRefs []ast.ASTRef
	defer func() { result = errors.Join(result, p.releaseAll(bodyRefs)) }()
	var polarity []bool
	tok := p.lexer.nextToken()
	for tok != dtkError && tok != dtkEOS {
		if tok == dtkPeriod {
			if err := p.addRule(head, body, polarity); err != nil {
				return tok, err
			}
			return p.lexer.nextToken(), nil
		}
		var predRef ast.ASTRef
		var err error
		isNeg := false
		if tok == dtkNeg {
			tok = p.lexer.nextToken()
			isNeg = true
		}

		// td is a pointer into the lexer's buffer, so it reads the data of
		// the token after a '!'.
		td := p.lexer.tokenData()
		_, isVar := p.vars[td]
		if tok == dtkString || tok == dtkNum || (tok == dtkID && isVar) {
			tok, predRef, err = p.parseInfix(tok, td)
		} else if tok == dtkID {
			tok = p.lexer.nextToken()
			// SASSERT(!is_declaration) is debug only: a predicate first used
			// in a body is declared there.
			tok, predRef, _, err = p.parsePred(tok, td)
		} else {
			return p.unexpected(tok, "expected predicate or relation")
		}
		if err != nil {
			return tok, errors.Join(err, p.release(&predRef))
		}
		pred, err := p.manager.RefGet(predRef)
		if err == nil {
			var r ast.ASTRef
			if r, err = p.hold(pred); err == nil {
				bodyRefs = append(bodyRefs, r)
			}
		}
		// The loop's app_ref pred ends with the iteration.
		if err = errors.Join(err, p.release(&predRef)); err != nil {
			return tok, err
		}
		body = append(body, pred)
		polarity = append(polarity, isNeg)

		if tok == dtkComma {
			tok = p.lexer.nextToken()
		} else if tok == dtkPeriod {
			continue
		} else {
			return p.unexpected(tok, "expected comma or period")
		}
	}
	return tok, nil
}

// addRule sends a ground body-less head to context::add_fact and any other
// rule, as the formula `body => head`, to context::add_rule.
// addRule is dparser::add_rule: a fact goes to context::add_fact; any
// other rule is built by rule_manager::mk(head, sz, body, is_neg) and added
// with context::add_rule(rule_ref&), keeping the body in source order.
func (p *datalogParser) addRule(head ast.ASTHandle, body []ast.ASTHandle, isNeg []bool) error {
	if len(body) == 0 {
		args, fact, err := p.factArgs(head)
		if err != nil {
			return err
		}
		if fact {
			decl, err := p.manager.GetDecl(head)
			if err != nil {
				return err
			}
			return p.context.AddFixedpointFact(decl, args)
		}
	}
	return p.context.AddFixedpointRuleObject(head, body, isNeg)
}

// factArgs is rule_manager::is_fact: every argument of head is a value.
func (p *datalogParser) factArgs(head ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	n, err := p.manager.GetNumArgs(head)
	if err != nil {
		return nil, false, err
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		if args[i], err = p.manager.GetArg(head, uint32(i)); err != nil {
			return nil, false, err
		}
		value, err := p.manager.IsValue(args[i])
		if err != nil || !value {
			return nil, false, err
		}
	}
	return args, true, nil
}
