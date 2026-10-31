package smt2

// Source: src/cmd_context/extra_cmds/dbg_cmds.cpp, expression storage and size.

import "fmt"

func (p *commandParser) debugCommand(name string) error {
	switch name {
	case "dbg-set", "dbg-get-qbody":
		key, err := p.symbol()
		if err != nil {
			return err
		}
		expression, _, err := p.expression()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		if name == "dbg-get-qbody" {
			m, err := p.astManager()
			if err != nil {
				return err
			}
			expression, err = m.GetQuantifierBody(expression)
			if err != nil {
				return fmt.Errorf("invalid command, term must be a quantifier")
			}
		}
		return p.context.StoreDebugExpression(key, expression)
	case "dbg-pp-var":
		key, err := p.symbol()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		expression, err := p.context.DebugExpression(key)
		if err != nil {
			return err
		}
		if err = p.context.WriteExpression(p.out, expression); err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.out)
		return err
	case "dbg-size":
		expression, _, err := p.expression()
		if err != nil {
			return err
		}
		if err = p.requireEnd(); err != nil {
			return err
		}
		m, err := p.astManager()
		if err != nil {
			return err
		}
		count, err := m.GetNumExprs(expression)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(p.out, count)
		return err
	}
	return fmt.Errorf("unknown debug command %s", name)
}
