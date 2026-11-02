package duality

// Source: muz/duality/duality_dl_interface.cpp, extract_proof/get_proof.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

// Proof returns an owned hyper-resolution derivation of the original query.
// Original clauses and evaluated substitutions are copied into the proof, so
// the returned reference survives counterexample and backend destruction.
// The borrowed factory reconstructs witnesses removed by preprocessing.
func (c *Counterexample) Proof(factory solver.Factory) (out ast.ASTRef, result error) {
	if err := c.open(); err != nil {
		return out, err
	}
	if c.tree == nil || c.root == nil {
		return out, ErrWrapperOwner
	}
	if err := c.tree.open(); err != nil {
		return out, err
	}
	if err := c.tree.cleanupProofWitnesses(); err != nil {
		return out, err
	}
	m := c.tree.context.manager
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return out, err
	}
	defer func() {
		result = errors.Join(result, scope.Close())
		if result != nil {
			result = errors.Join(result, m.DelRef(&out))
		}
	}()
	return c.tree.extractCounterexampleProof(c.root, make(map[*Node]bool), factory)
}

func (g *RPFP) extractCounterexampleProof(node *Node, active map[*Node]bool, factory solver.Factory, background ...*Expr) (out ast.ASTRef, result error) {
	if node == nil || node.closed || node.Owner != g || node.Outgoing == nil || active[node] {
		return out, ErrWrapperOwner
	}
	edge := node.Outgoing
	if edge.Map == nil || edge.Map.closed {
		return out, ErrWrapperOwner
	}
	active[node] = true
	defer delete(active, node)
	m := g.context.manager
	var refs []ast.ASTRef
	var terms []*Expr
	defer func() {
		for _, term := range terms {
			result = errors.Join(result, term.Close())
		}
		for i := range refs {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
		if result != nil {
			result = errors.Join(result, m.DelRef(&out))
		}
	}()
	hold := func(term *Expr, err error) (*Expr, error) {
		if term != nil {
			terms = append(terms, term)
		}
		return term, err
	}
	original, err := hold(edge.Map.Owner.OriginalClause(edge.Map))
	if err != nil {
		return out, err
	}
	clause, err := original.Raw()
	if err != nil {
		return out, err
	}
	asserted, err := m.MkProofAsserted(clause)
	if err != nil {
		return out, err
	}
	ref, err := m.NewRef(asserted)
	if err != nil {
		return out, err
	}
	refs = append(refs, ref)
	premises := []ast.ASTHandle{asserted}
	substitutions := [][]ast.ASTHandle{nil}
	quantified := clause
	kind, err := m.GetKind(clause)
	if err != nil {
		return out, err
	}
	if kind == ast.ASTQuantifier {
		forall, err := m.IsForall(clause)
		if err != nil {
			return out, err
		}
		if forall {
			n, err := m.GetNumDeclarations(clause)
			if err != nil {
				return out, err
			}
			substitutions[0] = make([]ast.ASTHandle, n)
			clause, err = m.GetQuantifierBody(clause)
			if err != nil {
				return out, err
			}
		}
	}
	head, positions, err := edge.Map.Owner.proofClauseShape(clause)
	if err != nil {
		return out, err
	}
	if len(positions) != len(edge.Children) {
		return out, ast.ErrASTDomain
	}
	var resolved []ast.ASTProofPosition
	for i, child := range edge.Children {
		empty, err := g.Empty(child)
		if err != nil {
			return out, err
		}
		if empty {
			continue
		}
		proof, err := g.extractCounterexampleProof(child, active, factory, background...)
		if err != nil {
			return out, err
		}
		refs = append(refs, proof)
		h, err := m.RefGet(proof)
		if err != nil {
			return out, err
		}
		for _, position := range positions[i] {
			premises = append(premises, h)
			substitutions = append(substitutions, nil)
			resolved = append(resolved, ast.ASTProofPosition{First: position, Second: 0})
		}
	}
	decl, err := m.GetDecl(head)
	if err != nil {
		return out, err
	}
	arity, err := m.GetArity(decl)
	if err != nil {
		return out, err
	}
	if int(arity) > len(edge.F.individualParams) {
		return out, ast.ErrASTDomain
	}
	args := make([]ast.ASTHandle, arity)
	for i := range args {
		h, err := m.RefGet(edge.F.individualParams[i])
		if err != nil {
			return out, err
		}
		param, err := hold(NewExpr(g.context, h))
		if err != nil {
			return out, err
		}
		value, err := hold(g.Eval(edge, param))
		if err != nil {
			return out, err
		}
		args[i], err = value.Raw()
		if err != nil {
			return out, err
		}
	}
	// Use the original head, including false for the synthetic @Fail node.
	conclusion, err := m.MkApp(decl, args)
	if err != nil {
		return out, err
	}
	conclusionRef, err := m.NewRef(conclusion)
	if err != nil {
		return out, err
	}
	refs = append(refs, conclusionRef)
	if len(substitutions[0]) > 0 {
		values, err := g.recoverProofWitness(factory, quantified, clause, positions, resolved, premises, conclusion, background...)
		if err != nil {
			return out, err
		}
		refs = append(refs, values...)
		for i, value := range values {
			substitutions[0][i], err = m.RefGet(value)
			if err != nil {
				return out, err
			}
		}
	}
	proof, err := m.MkProofHyperResolve(premises, conclusion, resolved, substitutions)
	if err != nil {
		return out, err
	}
	return m.NewRef(proof)
}

// Retain literal positions from the original antecedent, including theory
// constraints interleaved with predicate applications.
func (g *RPFP) proofClauseShape(clause ast.ASTHandle) (ast.ASTHandle, [][]uint32, error) {
	m := g.context.manager
	op, err := m.GetDeclKind(clause)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	family, err := m.GetFamilyID(clause)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	if family != m.BasicFamilyID() || op != ast.ASTOpImplies {
		return clause, nil, nil
	}
	head, err := m.GetArg(clause, 1)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	body, err := m.GetArg(clause, 0)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	atoms := []ast.ASTHandle{body}
	if kind, err := m.GetKind(body); err != nil {
		return ast.ASTHandle{}, nil, err
	} else if kind == ast.ASTApp {
		family, err := m.GetFamilyID(body)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if k, err := m.GetDeclKind(body); err != nil {
			return ast.ASTHandle{}, nil, err
		} else if family == m.BasicFamilyID() && k == ast.ASTOpAnd {
			n, err := m.GetNumArgs(body)
			if err != nil {
				return ast.ASTHandle{}, nil, err
			}
			atoms = make([]ast.ASTHandle, n)
			for i := range atoms {
				atoms[i], err = m.GetArg(body, uint32(i))
				if err != nil {
					return ast.ASTHandle{}, nil, err
				}
			}
		}
	}
	predicates := make(map[ast.ASTHandle]bool)
	for _, ref := range g.sourceClauses {
		h, err := m.RefGet(ref)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if kind, err := m.GetKind(h); err != nil {
			return ast.ASTHandle{}, nil, err
		} else if kind == ast.ASTQuantifier {
			h, err = m.GetQuantifierBody(h)
			if err != nil {
				return ast.ASTHandle{}, nil, err
			}
		}
		family, err := m.GetFamilyID(h)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if family == m.BasicFamilyID() && op == ast.ASTOpImplies {
			h, err = m.GetArg(h, 1)
			if err != nil {
				return ast.ASTHandle{}, nil, err
			}
		}
		decl, err := m.GetDecl(h)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		family, err = m.GetFamilyID(h)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if family == ast.NullFamilyID {
			predicates[decl] = true
		}
	}
	var positions [][]uint32
	seen := make(map[ast.ASTHandle]int)
	for i, atom := range atoms {
		kind, err := m.GetKind(atom)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if kind != ast.ASTApp {
			continue
		}
		decl, err := m.GetDecl(atom)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if predicates[decl] {
			index, found := seen[atom]
			if !found {
				index = len(positions)
				seen[atom] = index
				positions = append(positions, nil)
			}
			positions[index] = append(positions[index], uint32(i+1))
		}
	}
	return head, positions, nil
}
