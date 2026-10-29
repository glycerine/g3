package core

import "github.com/glycerine/g3/pkg/ast"

func domOp(m *ast.ASTManager, h ast.ASTHandle, op int32) (bool, error) {
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	f, e := m.GetFamilyID(h)
	if e != nil || f != m.BasicFamilyID() {
		return false, e
	}
	v, e := m.GetDeclKind(h)
	return v == op, e
}

func (t *DomSimplifyTactic) simplify(h ast.ASTHandle, c *domContext, depth uint32) (ast.ASTHandle, error) {
	h = c.find(h)
	if depth > t.maxDepth {
		return h, nil
	}
	m := t.manager
	k, e := m.GetKind(h)
	if e != nil {
		return h, e
	}
	if k != ast.ASTApp {
		return h, nil
	}
	if yes, e := domOp(m, h, ast.ASTOpIte); e != nil {
		return h, e
	} else if yes {
		return t.simplifyITE(h, c, depth)
	}
	if yes, e := domOp(m, h, ast.ASTOpAnd); e != nil {
		return h, e
	} else if yes {
		return t.simplifyAndOr(h, c, depth, true)
	}
	if yes, e := domOp(m, h, ast.ASTOpOr); e != nil {
		return h, e
	} else if yes {
		return t.simplifyAndOr(h, c, depth, false)
	}
	if yes, arg, e := m.IsNot(h); e != nil {
		return h, e
	} else if yes {
		next, e := t.simplify(arg, c, depth+1)
		if e != nil {
			return h, e
		}
		truth, e := m.MkTrue()
		if e != nil {
			return h, e
		}
		falsity, e := m.MkFalse()
		if e != nil {
			return h, e
		}
		if next == truth {
			return falsity, nil
		}
		if next == falsity {
			return truth, nil
		}
		if next == arg {
			return h, nil
		}
		return m.MkNot(next)
	}
	n, e := m.GetNumArgs(h)
	if e != nil {
		return h, e
	}
	args := make([]ast.ASTHandle, n)
	changed := false
	for i := uint32(0); i < n; i++ {
		a, x := m.GetArg(h, i)
		if x != nil {
			return h, x
		}
		args[i], x = t.simplify(a, c, depth+1)
		if x != nil {
			return h, x
		}
		changed = changed || args[i] != a
	}
	if !changed {
		if yes, e := domOp(m, h, ast.ASTOpEq); e != nil {
			return h, e
		} else if yes && n == 2 && args[0] == args[1] {
			return m.MkTrue()
		}
		return c.find(h), nil
	}
	if yes, e := domOp(m, h, ast.ASTOpEq); e != nil {
		return h, e
	} else if yes && n == 2 && args[0] == args[1] {
		return m.MkTrue()
	}
	d, e := m.GetDecl(h)
	if e != nil {
		return h, e
	}
	return m.MkApp(d, args)
}

func (t *DomSimplifyTactic) simplifyAndOr(h ast.ASTHandle, c *domContext, depth uint32, and bool) (ast.ASTHandle, error) {
	m := t.manager
	n, e := m.GetNumArgs(h)
	if e != nil {
		return h, e
	}
	level := c.level()
	defer c.pop(level)
	args := make([]ast.ASTHandle, n)
	order := make([]uint32, n)
	for i := range order {
		if t.forward {
			order[i] = uint32(i)
		} else {
			order[i] = n - 1 - uint32(i)
		}
	}
	truth, e := m.MkTrue()
	if e != nil {
		return h, e
	}
	falsity, e := m.MkFalse()
	if e != nil {
		return h, e
	}
	for _, i := range order {
		a, x := m.GetArg(h, i)
		if x != nil {
			return h, x
		}
		r, x := t.simplify(a, c, depth+1)
		if x != nil {
			return h, x
		}
		args[i] = r
		if and && r == falsity {
			return falsity, nil
		}
		if !and && r == truth {
			return truth, nil
		}
		if x = t.assertContext(c, r, !and); x != nil {
			return h, x
		}
	}
	filtered := args[:0]
	for _, a := range args {
		if and && a == truth || !and && a == falsity {
			continue
		}
		filtered = append(filtered, a)
	}
	if len(filtered) == 0 {
		if and {
			return truth, nil
		}
		return falsity, nil
	}
	if len(filtered) == 1 {
		return filtered[0], nil
	}
	if and {
		return m.MkAndArgs(filtered)
	}
	return m.MkOrArgs(filtered)
}

func (t *DomSimplifyTactic) simplifyITE(h ast.ASTHandle, c *domContext, depth uint32) (ast.ASTHandle, error) {
	m := t.manager
	condition, e := m.GetArg(h, 0)
	if e != nil {
		return h, e
	}
	thenBranch, e := m.GetArg(h, 1)
	if e != nil {
		return h, e
	}
	elseBranch, e := m.GetArg(h, 2)
	if e != nil {
		return h, e
	}
	condition, e = t.simplify(condition, c, depth+1)
	if e != nil {
		return h, e
	}
	truth, e := m.MkTrue()
	if e != nil {
		return h, e
	}
	falsity, e := m.MkFalse()
	if e != nil {
		return h, e
	}
	if condition == truth {
		return t.simplify(thenBranch, c, depth+1)
	}
	if condition == falsity {
		return t.simplify(elseBranch, c, depth+1)
	}
	level := c.level()
	if e = t.assertContext(c, condition, false); e != nil {
		return h, e
	}
	yes, e := t.simplify(thenBranch, c, depth+1)
	if e != nil {
		return h, e
	}
	c.pop(level)
	if e = t.assertContext(c, condition, true); e != nil {
		return h, e
	}
	no, e := t.simplify(elseBranch, c, depth+1)
	if e != nil {
		return h, e
	}
	c.pop(level)
	if yes == no {
		return yes, nil
	}
	if yes == thenBranch && no == elseBranch {
		return h, nil
	}
	return m.MkIte(condition, yes, no)
}
