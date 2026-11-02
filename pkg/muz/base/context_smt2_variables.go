package base

// Source: dl_context.cpp:1257-1335 is_query and declare_vars.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

func (s *RuleSet) declareContextVariables(out io.Writer, printer *ast.LegacySMTPrinter, rules []ast.ASTRef, fresh *model.FreshName) (resultErr error) {
	m := s.manager.manager
	sub, err := rewriter.NewVarSubstitution(m, false)
	if sub != nil {
		defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	}
	if err != nil {
		return err
	}
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&roots[i]))
		}
	}()
	bySort := make(map[ast.ASTHandle][]ast.ASTHandle)
	for _, r := range rules {
		h, err := m.RefGet(r)
		if err != nil {
			return err
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return err
		}
		if kind != ast.ASTQuantifier {
			continue
		}
		forall, err := m.IsForall(h)
		if err != nil {
			return err
		}
		if !forall {
			continue
		}
		body, err := m.GetQuantifierBody(h)
		if err != nil {
			return err
		}
		nested, err := m.HasQuantifiers(body)
		if err != nil {
			return err
		}
		if nested {
			continue
		}
		count, err := m.GetNumDeclarations(h)
		if err != nil {
			return err
		}
		used := make(map[ast.ASTHandle]int)
		args := make([]ast.ASTHandle, count)
		for j := uint32(0); j < count; j++ {
			sort, err := m.GetDeclarationSort(h, count-1-j)
			if err != nil {
				return err
			}
			index := used[sort]
			used[sort] = index + 1
			pool := bySort[sort]
			if index == len(pool) {
				name, err := fresh.Next()
				if err != nil {
					return err
				}
				constant, err := m.MkConst(name, sort)
				if err != nil {
					return err
				}
				ref, err := m.NewRef(constant)
				if err != nil {
					return err
				}
				roots = append(roots, ref)
				pool = append(pool, constant)
				bySort[sort] = pool
				if _, err = io.WriteString(out, "(declare-var "); err != nil {
					return err
				}
				text, err := m.Symbols.String(name)
				if err != nil {
					return err
				}
				if _, err = io.WriteString(out, text+" "); err != nil {
					return err
				}
				if err = s.writeContextSort(out, sort); err != nil {
					return err
				}
				if _, err = io.WriteString(out, ")\n"); err != nil {
					return err
				}
			}
			args[j] = pool[index]
		}
		result, err := m.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		roots = append(roots, result)
		if err = sub.Substitute(body, args, result); err != nil {
			return err
		}
		value, err := m.RefGet(result)
		if err != nil {
			return err
		}
		if err = m.SetRef(r, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *RuleSet) isContextQuery(h ast.ASTHandle) (bool, error) {
	m := s.manager.manager
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	d, err := m.GetDecl(h)
	if err != nil {
		return false, err
	}
	if _, yes := s.manager.predicates[d]; !yes {
		return false, nil
	}
	count, err := m.GetNumArgs(h)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		a, e := m.GetArg(h, i)
		if e != nil {
			return false, e
		}
		k, e := m.GetKind(a)
		if e != nil {
			return false, e
		}
		if k != ast.ASTVar {
			return false, nil
		}
		index, e := m.GetVarIndex(a)
		if e != nil {
			return false, e
		}
		if index != i {
			return false, nil
		}
	}
	return true, nil
}

func (s *RuleSet) writeContextQuery(out io.Writer, printer *ast.LegacySMTPrinter, h ast.ASTHandle, low bool) (resultErr error) {
	m := s.manager.manager
	direct, err := s.isContextQuery(h)
	if err != nil {
		return err
	}
	var declaration ast.ASTHandle
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&roots[i]))
		}
	}()
	own := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return h, err
		}
		r, err := m.NewRef(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		roots = append(roots, r)
		return h, nil
	}
	if direct {
		declaration, err = m.GetDecl(h)
		if err != nil {
			return err
		}
	} else {
		vars, err := ast.NewASTUsedVars(m)
		if err != nil {
			return err
		}
		if err = vars.Collect(h); err != nil {
			return err
		}
		count := vars.MaxFoundVarIndexPlusOne()
		domain, args := make([]ast.ASTHandle, count), make([]ast.ASTHandle, count)
		boolean, err := m.MkBoolSort()
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			sort := vars.Contains(i)
			if sort.IsNull() {
				sort = boolean
			}
			domain[i] = sort
			args[i], err = own(m.MkVar(i, sort))
			if err != nil {
				return err
			}
		}
		prefix, err := m.Symbols.Intern("q")
		if err != nil {
			return err
		}
		suffix, err := m.Symbols.Intern("")
		if err != nil {
			return err
		}
		declaration, err = own(m.MkFreshFuncDecl(prefix, suffix, domain, boolean))
		if err != nil {
			return err
		}
		if err = s.writeContextRelation(out, printer, declaration); err != nil {
			return err
		}
		app, err := own(m.MkApp(declaration, args))
		if err != nil {
			return err
		}
		implication, err := own(m.MkImplies(h, app))
		if err != nil {
			return err
		}
		if _, err = io.WriteString(out, "(assert "); err != nil {
			return err
		}
		if err = s.writeContextExpression(out, printer, implication, low); err != nil {
			return err
		}
		if _, err = io.WriteString(out, ")\n"); err != nil {
			return err
		}
	}
	name, err := m.GetASTName(declaration)
	if err != nil {
		return err
	}
	text, err := m.Symbols.String(name)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, "(query "); err != nil {
		return err
	}
	if err = s.writeContextSymbol(out, text); err != nil {
		return err
	}
	_, err = io.WriteString(out, ")\n")
	return err
}
