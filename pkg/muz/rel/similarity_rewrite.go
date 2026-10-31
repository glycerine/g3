package rel

// Source: dl_mk_similarity_compressor.cpp, merge_class rule construction.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"math"
)

// similarityRewriteRule builds the representative rule for an already
// registered auxiliary predicate. The result is owned by the caller.
func similarityRewriteRule(rm *base.RuleManager, rule *base.Rule, positions []similarityConstant, aux ast.ASTHandle) (result *base.Rule, err error) {
	if rm == nil || len(positions) == 0 {
		return nil, ast.ErrASTDomain
	}
	m, e := rule.ASTManager()
	if e != nil {
		return nil, e
	}
	used, e := rule.UsedVariables()
	if e != nil {
		return nil, e
	}
	next := used.MaxFoundVarIndexPlusOne()
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
		if err != nil && result != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	pin := func(h ast.ASTHandle, e error) (ast.ASTHandle, error) {
		if e != nil {
			return ast.ASTHandle{}, e
		}
		r, e := m.NewRef(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		refs = append(refs, r)
		return h, nil
	}
	head, e := rule.Head()
	if e != nil {
		return nil, e
	}
	count, e := rule.TailSize()
	if e != nil {
		return nil, e
	}
	tails := make([]ast.ASTHandle, count, count+1)
	negs := make([]bool, count, count+1)
	for i := range tails {
		tails[i], negs[i], e = rule.Tail(i)
		if e != nil {
			return nil, e
		}
	}
	vars := make([]ast.ASTHandle, 0, len(positions))
	var auxArgs []ast.ASTHandle
	for i := 0; i < len(positions); {
		tail := positions[i].tail
		var atom ast.ASTHandle
		if tail < 0 {
			atom = head
		} else if tail < count {
			atom = tails[tail]
		} else {
			return nil, ast.ErrASTDomain
		}
		n, e := m.GetNumArgs(atom)
		if e != nil {
			return nil, e
		}
		args := make([]ast.ASTHandle, n)
		for j := range args {
			args[j], e = m.GetArg(atom, uint32(j))
			if e != nil {
				return nil, e
			}
		}
		for i < len(positions) && positions[i].tail == tail {
			pos := positions[i]
			if int(pos.argument) >= len(args) {
				return nil, ast.ErrASTDomain
			}
			var variable ast.ASTHandle
			if pos.parent >= 0 {
				if pos.parent >= len(vars) {
					return nil, ast.ErrASTDomain
				}
				variable = vars[pos.parent]
			} else {
				if uint64(next)+uint64(len(auxArgs)) > math.MaxUint32 {
					return nil, ast.ErrASTDomain
				}
				sort, e := m.GetSort(args[pos.argument])
				if e != nil {
					return nil, e
				}
				variable, e = pin(m.MkVar(next+uint32(len(auxArgs)), sort))
				if e != nil {
					return nil, e
				}
				auxArgs = append(auxArgs, variable)
			}
			vars = append(vars, variable)
			args[pos.argument] = variable
			i++
		}
		decl, e := m.GetDecl(atom)
		if e != nil {
			return nil, e
		}
		modified, e := pin(m.MkApp(decl, args))
		if e != nil {
			return nil, e
		}
		if tail < 0 {
			head = modified
		} else {
			tails[tail] = modified
		}
	}
	auxTail, e := pin(m.MkApp(aux, auxArgs))
	if e != nil {
		return nil, e
	}
	tails = append(tails, auxTail)
	negs = append(negs, false)
	name, e := rule.Name()
	if e != nil {
		return nil, e
	}
	result, e = rm.NewRule(head, tails, negs, name)
	if e != nil {
		return nil, e
	}
	if e = result.SetAccountingParentRule(rule); e != nil {
		return result, e
	}
	return result, nil
}
