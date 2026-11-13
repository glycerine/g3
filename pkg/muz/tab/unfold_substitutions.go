package tab

// Source: src/muz/tab/tab_context.cpp, unifier::extract_subst.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/ast/substitution"
)

type proofSubstitutions struct {
	manager *ast.ASTManager
	banks   [2][]ast.ASTRef
}

func (p *proofSubstitutions) Close() error {
	if p == nil || p.manager == nil {
		return nil
	}
	for bank := range p.banks {
		for i := range p.banks[bank] {
			if p.banks[bank][i] != (ast.ASTRef{}) {
				if err := p.manager.DelRef(&p.banks[bank][i]); err != nil {
					return err
				}
			}
		}
		p.banks[bank] = nil
	}
	p.manager = nil
	return nil
}
func (u *unfoldedClause) proofSubstitutions(target, source *Clause) (result *proofSubstitutions, err error) {
	if u == nil || u.manager == nil || u.bindings == nil || target == nil || source == nil || target.closed || source.closed || target.manager != u.manager || source.manager != u.manager {
		return nil, ast.ErrASTHandle
	}
	m := u.manager
	result = &proofSubstitutions{manager: m}
	var rename *rewriter.VarSubstitution
	defer func() {
		if rename != nil {
			err = errors.Join(err, rename.Close())
		}
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	names := make([]ast.ASTHandle, len(u.rename))
	for i, ref := range u.rename {
		if ref != (ast.ASTRef{}) {
			names[i], err = m.RefGet(ref)
			if err != nil {
				return result, err
			}
		}
	}
	if u.members != nil {
		// extract_subst applies the unifier's own m_S2; its m_sub1 and
		// m_sub2 stay with the unifier until the next reset.
		borrowed, e := u.members.varSubstitution()
		if e != nil {
			return result, e
		}
		defer func() {
			if err == nil {
				u.members.sub = result
			}
		}()
		return result, u.extractSubstitutions(result, target, source, names, borrowed)
	}
	rename, err = rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return result, err
	}
	return result, u.extractSubstitutions(result, target, source, names, rename)
}

func (u *unfoldedClause) extractSubstitutions(result *proofSubstitutions, target, source *Clause, names []ast.ASTHandle, rename *rewriter.VarSubstitution) error {
	m := u.manager
	for bank, clause := range []*Clause{target, source} {
		vars, e := clause.freeVariables()
		if e != nil {
			return e
		}
		for i := uint32(0); i < vars.Size(); i++ {
			sort, e := vars.Get(i)
			if e != nil {
				return e
			}
			out, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return e
			}
			result.banks[bank] = append(result.banks[bank], out)
			if sort.IsNull() {
				truth, e := m.MkTrue()
				if e != nil {
					return e
				}
				if e = m.SetRef(out, truth); e != nil {
					return e
				}
				continue
			}
			variable, e := m.MkVar(i, sort)
			if e != nil {
				return e
			}
			tmp, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return e
			}
			e = u.bindings.Apply(2, []uint32{0, u.maxVars}, substitution.NewExprOffset(variable, uint32(bank)), tmp)
			if e == nil {
				var h ast.ASTHandle
				h, e = m.RefGet(tmp)
				if e == nil {
					e = rename.Substitute(h, names, out)
				}
			}
			e = errors.Join(e, m.DelRef(&tmp))
			if e != nil {
				return e
			}
		}
	}
	return nil
}
