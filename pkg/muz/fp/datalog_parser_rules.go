package fp

// Source: dparser::parse_rule, parse_body, parse_infix and add_rule.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (p *datalogParser) parseRule(name string, declaration ast.ASTHandle) error {
	p.vars = make(map[string]ast.ASTHandle)
	p.nextVar = 0
	arguments, err := p.parseArguments(declaration)
	if err != nil {
		return err
	}
	head, err := p.manager.MkApp(declaration, arguments)
	if err != nil {
		return err
	}
	if p.token.kind == datalogPeriod {
		p.advance()
		return p.addRule(head, nil, nil)
	}
	return p.parseRuleBody(head)
}

func (p *datalogParser) parseRuleBody(head ast.ASTHandle) error {
	if err := p.expect(datalogLeftArrow, "expected '.' or ':-' after rule head"); err != nil {
		return err
	}
	var tail []ast.ASTHandle
	var negated []bool
	for {
		negative := false
		if p.token.kind == datalogNegation {
			negative = true
			p.advance()
		}
		expression, err := p.parseBodyExpression()
		if err != nil {
			return err
		}
		tail = append(tail, expression)
		negated = append(negated, negative)
		if p.token.kind == datalogPeriod {
			p.advance()
			break
		}
		if err = p.expect(datalogComma, "expected comma or period in rule body"); err != nil {
			return err
		}
	}
	return p.addRule(head, tail, negated)
}

func (p *datalogParser) parseBodyExpression() (ast.ASTHandle, error) {
	if p.token.kind != datalogIdentifier {
		return ast.ASTHandle{}, p.fail("expected predicate in rule body")
	}
	first := p.token
	p.advance()
	if p.token.kind == datalogEqual || p.token.kind == datalogNotEqual || p.token.kind == datalogLess || p.token.kind == datalogGreater {
		left, exists := p.vars[first.text]
		if !exists {
			return ast.ASTHandle{}, p.fail("infix relation requires a previously typed variable")
		}
		operator := p.token.kind
		p.advance()
		sort, err := p.manager.GetSort(left)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		right, err := p.parseArgument(sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		switch operator {
		case datalogEqual:
			return p.manager.MkEq(left, right)
		case datalogNotEqual:
			equal, e := p.manager.MkEq(left, right)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return p.manager.MkNot(equal)
		case datalogLess:
			return p.manager.MkArithmeticLT(left, right)
		default:
			return p.manager.MkArithmeticLT(right, left)
		}
	}
	name, err := p.symbol(first.text)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, exists, err := p.context.Predicate(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !exists {
		return ast.ASTHandle{}, p.fail("predicate must be declared before use")
	}
	arguments, err := p.parseArguments(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.manager.MkApp(declaration, arguments)
}

func (p *datalogParser) addRule(head ast.ASTHandle, tail []ast.ASTHandle, negated []bool) (resultErr error) {
	manager, err := p.context.RuleManager()
	if err != nil {
		return err
	}
	rule, err := manager.NewRule(head, tail, negated, "")
	if err != nil {
		return err
	}
	defer func() { resultErr = joinErrors(resultErr, rule.Close()) }()
	return p.context.AddRule(rule)
}
