package smt2

// Source: src/muz/fp/dl_cmds.cpp collect_query and scope trail.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
)

// ParseAssertionsWithFixedpointQueries parses without executing checks and
// returns owned references to the final scoped fixedpoint query expressions.
// The caller must release each reference with the context's AST manager.
func ParseAssertionsWithFixedpointQueries(context *cmd_context.Context, input io.Reader) (queries []ast.ASTRef, resultErr error) {
	program, err := ParseFixedpointProgram(context, input)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			for i := range program.Queries {
				resultErr = errors.Join(resultErr, program.manager.DelRef(&program.Queries[i]))
			}
			queries = nil
		}
	}()
	for i := range program.Rules {
		resultErr = errors.Join(resultErr, program.manager.DelRef(&program.Rules[i]))
	}
	return program.Queries, resultErr
}

// FixedpointProgram holds owned query and bound rule references. Callers must
// release all references through the parser context's manager.
type FixedpointProgram struct {
	Queries []ast.ASTRef
	Rules   []ast.ASTRef
	Names   []string
	manager *ast.ASTManager
}

// ParseFixedpointProgram collects fixedpoint commands without executing checks
// or ingesting rules into the parser's fixedpoint engine.
func ParseFixedpointProgram(context *cmd_context.Context, input io.Reader) (program FixedpointProgram, resultErr error) {
	c := &fixedpointQueryCollector{}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, c.clear())
		}
	}()
	if err := parseCommands(context, input, io.Discard, true, c); err != nil {
		return program, err
	}
	return FixedpointProgram{Queries: c.queries, Rules: c.rules, Names: c.names, manager: c.manager}, nil
}

type fixedpointCollectorScope struct{ queries, rules int }
type fixedpointQueryCollector struct {
	manager *ast.ASTManager
	queries []ast.ASTRef
	rules   []ast.ASTRef
	names   []string
	scopes  []fixedpointCollectorScope
}

func (c *fixedpointQueryCollector) addRule(p *commandParser, formula ast.ASTHandle, name string) error {
	m, err := p.astManager()
	if err != nil {
		return err
	}
	c.manager = m
	ref, err := p.context.BindFixedpointRule(formula)
	if err != nil {
		return err
	}
	c.rules = append(c.rules, ref)
	c.names = append(c.names, name)
	return nil
}

func (c *fixedpointQueryCollector) add(p *commandParser, declaration ast.ASTHandle) error {
	m, err := p.astManager()
	if err != nil {
		return err
	}
	c.manager = m
	count, err := m.GetArity(declaration)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		sort, e := m.GetDomain(declaration, i)
		if e != nil {
			return e
		}
		args[i], e = p.pin(m.MkVar(i, sort))
		if e != nil {
			return e
		}
	}
	app, err := m.MkApp(declaration, args)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(app)
	if err != nil {
		return err
	}
	c.queries = append(c.queries, ref)
	return nil
}
func (c *fixedpointQueryCollector) push(n uint32) {
	for i := uint32(0); i < n; i++ {
		c.scopes = append(c.scopes, fixedpointCollectorScope{len(c.queries), len(c.rules)})
	}
}
func (c *fixedpointQueryCollector) truncate(scope fixedpointCollectorScope) (result error) {
	for i := len(c.queries) - 1; i >= scope.queries; i-- {
		result = errors.Join(result, c.manager.DelRef(&c.queries[i]))
	}
	for i := len(c.rules) - 1; i >= scope.rules; i-- {
		result = errors.Join(result, c.manager.DelRef(&c.rules[i]))
	}
	c.queries = c.queries[:scope.queries]
	c.rules = c.rules[:scope.rules]
	c.names = c.names[:scope.rules]
	return result
}
func (c *fixedpointQueryCollector) pop(n uint32) error {
	if n == 0 {
		return nil
	}
	if int(n) > len(c.scopes) {
		c.scopes = nil
		return c.truncate(fixedpointCollectorScope{})
	}
	first := len(c.scopes) - int(n)
	err := c.truncate(c.scopes[first])
	c.scopes = c.scopes[:first]
	return err
}
func (c *fixedpointQueryCollector) clear() error {
	err := c.truncate(fixedpointCollectorScope{})
	c.manager = nil
	c.scopes = nil
	return err
}
