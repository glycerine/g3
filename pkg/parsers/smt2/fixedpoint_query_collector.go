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
	c := &fixedpointQueryCollector{}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, c.clear())
		}
	}()
	if err := parseCommands(context, input, io.Discard, true, c); err != nil {
		return nil, err
	}
	return c.queries, nil
}

type fixedpointQueryCollector struct {
	manager *ast.ASTManager
	queries []ast.ASTRef
	scopes  []int
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
		c.scopes = append(c.scopes, len(c.queries))
	}
}
func (c *fixedpointQueryCollector) truncate(n int) (result error) {
	for i := len(c.queries) - 1; i >= n; i-- {
		result = errors.Join(result, c.manager.DelRef(&c.queries[i]))
	}
	c.queries = c.queries[:n]
	return result
}
func (c *fixedpointQueryCollector) pop(n uint32) error {
	if n == 0 {
		return nil
	}
	if int(n) > len(c.scopes) {
		c.scopes = nil
		return c.truncate(0)
	}
	first := len(c.scopes) - int(n)
	err := c.truncate(c.scopes[first])
	c.scopes = c.scopes[:first]
	return err
}
func (c *fixedpointQueryCollector) clear() error {
	err := c.truncate(0)
	c.manager = nil
	c.scopes = nil
	return err
}
