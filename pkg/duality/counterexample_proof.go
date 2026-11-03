package duality

// Source: muz/duality/duality_dl_interface.cpp, extract_proof/get_proof.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/solver"
)

// Proof returns an owned hyper-resolution derivation of the original query.
// Original clauses and evaluated substitutions are copied into the proof, so
// the returned reference survives counterexample and backend destruction.
// The borrowed factory reconstructs witnesses removed by preprocessing.
func (c *Counterexample) Proof(factory solver.Factory, background ...*Expr) (out ast.ASTRef, result error) {
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
	pendingStart := len(c.tree.pendingProofWitnesses)
	m := c.tree.context.manager
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return out, err
	}
	call := &proofWitnessCall{manager: m, proofScope: scope}
	defer func() {
		pending := false
		if cleanup := call.close(); cleanup != nil {
			pending = true
			result = errors.Join(result, cleanup)
			// Proof mode must be restored before releasing a proof produced
			// under that mode. Move the output behind the same retry owner.
			if out != (ast.ASTRef{}) {
				call.refs = append(call.refs, out)
				out = ast.ASTRef{}
			}
		} else if result != nil && out != (ast.ASTRef{}) {
			cleanup := m.DelRef(&out)
			result = errors.Join(result, cleanup)
			if out != (ast.ASTRef{}) {
				call.refs = append(call.refs, out)
				out = ast.ASTRef{}
				pending = true
			}
		}
		if pending {
			// Recursive extraction defers run first and may already have queued
			// proof refs created under this scope. A refused proof-mode restore
			// must remain ahead of all of those newly queued owners.
			if call.proofScope != nil {
				c.tree.insertPendingProofWitness(pendingStart, call)
			} else {
				c.tree.pendingProofWitnesses = append(c.tree.pendingProofWitnesses, call)
			}
		}
	}()
	return c.tree.extractCounterexampleProof(c.root, make(map[*Node]bool), factory, background...)
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
	call := &proofWitnessCall{manager: m}
	defer func() {
		pending := false
		if err := call.close(); err != nil {
			pending = true
			result = errors.Join(result, err)
		}
		if result != nil && out != (ast.ASTRef{}) {
			cleanup := m.DelRef(&out)
			result = errors.Join(result, cleanup)
			if out != (ast.ASTRef{}) {
				call.refs = append(call.refs, out)
				out = ast.ASTRef{}
				pending = true
			}
		}
		if pending {
			g.pendingProofWitnesses = append(g.pendingProofWitnesses, call)
		}
	}()
	hold := func(term *Expr, err error) (*Expr, error) {
		if term != nil {
			call.terms = append(call.terms, term)
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
	call.refs = append(call.refs, ref)
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
		call.refs = append(call.refs, proof)
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
	call.refs = append(call.refs, conclusionRef)
	if len(substitutions[0]) > 0 {
		witnessBackground := background
		if edge.Map.quantifiedTheory {
			sourceGraph := edge.Map.Owner
			currentIndex, found := sourceGraph.clauseIndices[edge.Map]
			if !found {
				return out, ErrWrapperOwner
			}
			witnessBackground = append([]*Expr(nil), background...)
			for i, source := range sourceGraph.sourceClauses {
				if i == currentIndex {
					continue
				}
				h, err := m.RefGet(source)
				if err != nil {
					return out, err
				}
				axiom, err := hold(NewExpr(g.context, h))
				if err != nil {
					return out, err
				}
				witnessBackground = append(witnessBackground, axiom)
			}
		}
		values, err := g.recoverProofWitness(factory, quantified, clause, positions, resolved, premises, conclusion, g.hasMappedQuantifiedTheory(), witnessBackground...)
		if err != nil {
			return out, fmt.Errorf("proof witness source edge %d exceptional=%v: %w", edge.Map.Number, edge.Map.quantifiedTheory, err)
		}
		call.refs = append(call.refs, values...)
		for i, value := range values {
			substitutions[0][i], err = m.RefGet(value)
			if err != nil {
				return out, err
			}
		}
	}

	{
		instantiated := clause
		if len(substitutions[0]) > 0 {
			call.substitution, err = rewriter.NewVarSubstitution(m, false)
			if err != nil {
				return out, err
			}
			ref, err := m.NewRef(ast.ASTHandle{})
			if err != nil {
				return out, err
			}
			call.refs = append(call.refs, ref)
			if err := call.substitution.Substitute(clause, substitutions[0], ref); err != nil {
				return out, err
			}
			instantiated, err = m.RefGet(ref)
			if err != nil {
				return out, err
			}
		}
		backgroundProofs, err := g.backgroundLiteralProofs(background, &call.refs)
		if err != nil {
			return out, err
		}
		var programAxioms []*Expr
		programPremises := func(target ast.ASTHandle) ([]*Expr, error) {
			sourceGraph := edge.Map.Owner
			currentIndex, found := sourceGraph.clauseIndices[edge.Map]
			if !found {
				return nil, ErrWrapperOwner
			}
			programAxioms = nil
			indices, err := sourceGraph.proofProgramPremiseIndices(target, currentIndex)
			if err != nil {
				return nil, err
			}
			for _, i := range indices {
				source := sourceGraph.sourceClauses[i]
				h, err := m.RefGet(source)
				if err != nil {
					return nil, err
				}
				axiom, err := hold(NewExpr(g.context, h))
				if err != nil {
					return nil, err
				}
				programAxioms = append(programAxioms, axiom)
			}
			return append(append([]*Expr(nil), background...), programAxioms...), nil
		}
		family, err := m.GetFamilyID(instantiated)
		if err != nil {
			return out, err
		}
		op, err := m.GetDeclKind(instantiated)
		if err != nil {
			return out, err
		}
		if family == m.BasicFamilyID() && op == ast.ASTOpImplies {
			body, err := m.GetArg(instantiated, 0)
			if err != nil {
				return out, err
			}
			atoms := []ast.ASTHandle{body}
			family, err = m.GetFamilyID(body)
			if err != nil {
				return out, err
			}
			op, err = m.GetDeclKind(body)
			if err != nil {
				return out, err
			}
			if family == m.BasicFamilyID() && op == ast.ASTOpAnd {
				n, err := m.GetNumArgs(body)
				if err != nil {
					return out, err
				}
				atoms = make([]ast.ASTHandle, n)
				for i := range atoms {
					atoms[i], err = m.GetArg(body, uint32(i))
					if err != nil {
						return out, err
					}
				}
			}
			predicatePositions := make(map[uint32]bool)
			for _, group := range positions {
				for _, position := range group {
					predicatePositions[position] = true
				}
			}
			for i, atom := range atoms {
				if !predicatePositions[uint32(i+1)] {
					if _, found := backgroundProofs[atom]; !found {
						axioms := background
						quantified, err := g.proofContainsQuantifier(atom)
						if err != nil {
							return out, err
						}
						var lemma ast.ASTRef
						if quantified && edge.Map.quantifiedTheory {
							sourceGraph := edge.Map.Owner
							currentIndex, found := sourceGraph.clauseIndices[edge.Map]
							if !found {
								return out, ErrWrapperOwner
							}
							lemma, err = g.proveQuantifiedHornLiteral(factory, sourceGraph, currentIndex, atom, &call.refs)
							if err != nil {
								return out, err
							}
						}
						if quantified && lemma == (ast.ASTRef{}) {
							axioms, err = programPremises(atom)
							if err != nil {
								return out, err
							}
						}
						if lemma == (ast.ASTRef{}) && len(axioms) > 0 {
							lemma, err = g.proveBackgroundLiteral(factory, axioms, atom)
						}
						if err != nil {
							return out, err
						}
						if lemma != (ast.ASTRef{}) {
							call.refs = append(call.refs, lemma)
							h, err := m.RefGet(lemma)
							if err != nil {
								return out, err
							}
							backgroundProofs[atom] = h
						}
					}
				}
				if proof, ok := backgroundProofs[atom]; ok {
					premises = append(premises, proof)
					substitutions = append(substitutions, nil)
					resolved = append(resolved, ast.ASTProofPosition{First: uint32(i + 1), Second: 0})
				}
			}
		}
	}
	proof, err := m.MkProofHyperResolve(premises, conclusion, resolved, substitutions)
	if err != nil {
		return out, err
	}
	return m.NewRef(proof)
}

func (g *RPFP) proofContainsQuantifier(root ast.ASTHandle) (bool, error) {
	m := g.context.manager
	todo := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(todo) > 0 {
		i := len(todo) - 1
		h := todo[i]
		todo = todo[:i]
		if seen[h] {
			continue
		}
		seen[h] = true
		kind, err := m.GetKind(h)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTQuantifier {
			return true, nil
		}
		if kind != ast.ASTApp {
			continue
		}
		n, err := m.GetNumArgs(h)
		if err != nil {
			return false, err
		}
		for j := uint32(0); j < n; j++ {
			child, err := m.GetArg(h, j)
			if err != nil {
				return false, err
			}
			todo = append(todo, child)
		}
	}
	return false, nil
}

func (g *RPFP) proofClauseHeadBody(formula ast.ASTHandle) (head, body ast.ASTHandle, err error) {
	m := g.context.manager
	if kind, getErr := m.GetKind(formula); getErr != nil {
		return head, body, getErr
	} else if kind == ast.ASTQuantifier {
		formula, err = m.GetQuantifierBody(formula)
		if err != nil {
			return head, body, err
		}
	}
	family, err := m.GetFamilyID(formula)
	if err != nil {
		return head, body, err
	}
	op, err := m.GetDeclKind(formula)
	if err != nil {
		return head, body, err
	}
	if family == m.BasicFamilyID() && op == ast.ASTOpImplies {
		body, err = m.GetArg(formula, 0)
		if err == nil {
			head, err = m.GetArg(formula, 1)
		}
		return head, body, err
	}
	body, err = m.MkTrue()
	return formula, body, err
}

func (g *RPFP) proofPredicateDeclarations(root ast.ASTHandle) (map[ast.ASTHandle]bool, error) {
	m := g.context.manager
	heads := make(map[ast.ASTHandle]bool)
	for _, source := range g.sourceClauses {
		h, err := m.RefGet(source)
		if err != nil {
			return nil, err
		}
		head, _, err := g.proofClauseHeadBody(h)
		if err != nil {
			return nil, err
		}
		decl, err := m.GetDecl(head)
		if err == nil {
			heads[decl] = true
		}
	}
	result := make(map[ast.ASTHandle]bool)
	todo := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(todo) > 0 {
		i := len(todo) - 1
		h := todo[i]
		todo = todo[:i]
		if seen[h] {
			continue
		}
		seen[h] = true
		kind, err := m.GetKind(h)
		if err != nil {
			return nil, err
		}
		if kind == ast.ASTQuantifier {
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return nil, err
			}
			todo = append(todo, body)
			continue
		}
		if kind != ast.ASTApp {
			continue
		}
		decl, err := m.GetDecl(h)
		if err != nil {
			return nil, err
		}
		if heads[decl] {
			result[decl] = true
		}
		n, err := m.GetNumArgs(h)
		if err != nil {
			return nil, err
		}
		for j := uint32(0); j < n; j++ {
			child, err := m.GetArg(h, j)
			if err != nil {
				return nil, err
			}
			todo = append(todo, child)
		}
	}
	return result, nil
}

// proofProgramPremiseIndices closes the source program backwards from a
// quantified proof obligation. Facts remain available as source axioms, while
// selected rule bodies add their predicate dependencies until a fixed point is
// reached. The returned indices preserve source order and never include the
// clause whose proof is currently being reconstructed.
func (g *RPFP) proofProgramPremiseIndices(target ast.ASTHandle, currentIndex int) ([]int, error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if currentIndex < 0 || currentIndex >= len(g.sourceClauses) {
		return nil, ErrWrapperOwner
	}
	m := g.context.manager
	needed, err := g.proofPredicateDeclarations(target)
	if err != nil {
		return nil, err
	}
	selected := make([]bool, len(g.sourceClauses))
	changed := true
	for changed {
		changed = false
		for i, source := range g.sourceClauses {
			if i == currentIndex || selected[i] {
				continue
			}
			h, err := m.RefGet(source)
			if err != nil {
				return nil, err
			}
			head, body, err := g.proofClauseHeadBody(h)
			if err != nil {
				return nil, err
			}
			headDecl, err := m.GetDecl(head)
			if err != nil {
				return nil, err
			}
			if !m.IsTrue(body) && !needed[headDecl] {
				continue
			}
			selected[i] = true
			changed = true
			dependencies, err := g.proofPredicateDeclarations(body)
			if err != nil {
				return nil, err
			}
			for declaration := range dependencies {
				if !needed[declaration] {
					needed[declaration] = true
					changed = true
				}
			}
		}
	}
	indices := make([]int, 0, len(g.sourceClauses))
	for i, include := range selected {
		if include {
			indices = append(indices, i)
		}
	}
	return indices, nil
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
