package duality

// Source: muz/duality/duality_dl_interface.cpp, extract_proof/get_proof.

import (
	"errors"

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

// extractCounterexampleProof is extract_proof (FORK FIX #93 and #63): the
// original clause's asserted proof, then each used child's proof once per
// body literal it resolves, the substitution of the original rule's
// witness (find_step_witness, else the counterexample model's value of each
// skolem) in variable-index order, the background axiom (or and-elim
// conjunct) proof of each body literal that is one, and the original head
// as the conclusion.
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
	// prems.push_back(mk_asserted(uncook(t)))
	original, err := hold(edge.Map.Owner.OriginalClause(edge.Map))
	if err != nil {
		return out, err
	}
	t, err := original.Raw()
	if err != nil {
		return out, err
	}
	asserted, err := call.pin(m.MkProofAsserted(t))
	if err != nil {
		return out, err
	}
	premises := []ast.ASTHandle{asserted}
	substitutions := [][]ast.ASTHandle{nil}
	rawClause, isForall, err := stripForall(m, t)
	if err != nil {
		return out, err
	}
	positions, shaped, err := g.witnessPositions(edge.Map.Owner, rawClause)
	if err != nil {
		return out, err
	}
	shaped = shaped && len(positions) == len(edge.Children)
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
		childPositions := []uint32{uint32(i + 1)}
		if shaped {
			childPositions = positions[i]
		}
		for _, position := range childPositions {
			resolved = append(resolved, ast.ASTProofPosition{First: position, Second: 0})
			premises = append(premises, h)
			substitutions = append(substitutions, nil)
		}
	}
	if isForall {
		witness, hasWitness, err := g.findStepWitness(factory, node, background)
		if err != nil {
			return out, err
		}
		call.refs = append(call.refs, witness...)
		bound, err := m.GetNumDeclarations(t)
		if err != nil {
			return out, err
		}
		byDecl := make([]ast.ASTHandle, bound)
		for j := uint32(0); j < bound; j++ {
			name, err := m.GetDeclarationName(t, j)
			if err != nil {
				return out, err
			}
			sort, err := m.GetDeclarationSort(t, j)
			if err != nil {
				return out, err
			}
			constant, err := call.pin(m.MkConst(name, sort))
			if err != nil {
				return out, err
			}
			skolem, err := hold(NewExpr(g.context, constant))
			if err != nil {
				return out, err
			}
			if hasWitness {
				if byDecl[j], err = m.RefGet(witness[j]); err != nil {
					return out, err
				}
			} else {
				value, err := hold(g.Eval(edge, skolem))
				if err != nil {
					return out, err
				}
				if byDecl[j], err = value.Raw(); err != nil {
					return out, err
				}
			}
			// expr local_skolem = tree->Localize(&edge, skolem)
			if _, err = hold(g.Localize(edge, skolem)); err != nil {
				return out, err
			}
		}
		// FORK FIX #93: variable k is declaration bound-1-k
		substitutions[0] = make([]ast.ASTHandle, bound)
		for k := uint32(0); k < bound; k++ {
			substitutions[0][k] = byDecl[bound-1-k]
		}
	}
	// FORK FIX #93: a body literal that is a background axiom (or a
	// conjunct of one) is resolved with that axiom's proof.
	if shaped {
		instance := rawClause
		if len(substitutions[0]) > 0 {
			if call.substitution, err = rewriter.NewVarSubstitution(m, false); err != nil {
				return out, err
			}
			ref, err := m.NewRef(ast.ASTHandle{})
			if err != nil {
				return out, err
			}
			call.refs = append(call.refs, ref)
			if err = call.substitution.Substitute(rawClause, substitutions[0], ref); err != nil {
				return out, err
			}
			if instance, err = m.RefGet(ref); err != nil {
				return out, err
			}
		}
		backgroundProofs, err := g.backgroundLiteralProofs(background, &call.refs)
		if err != nil {
			return out, err
		}
		implies, err := isBasicOp(m, instance, ast.ASTOpImplies)
		if err != nil {
			return out, err
		}
		if implies {
			_, _, atoms, err := witnessClauseShape(m, instance)
			if err != nil {
				return out, err
			}
			predicatePositions := make(map[uint32]bool)
			for _, group := range positions {
				for _, position := range group {
					predicatePositions[position] = true
				}
			}
			for i, atom := range atoms {
				if proof, found := backgroundProofs[atom]; found && !predicatePositions[uint32(i+1)] {
					resolved = append(resolved, ast.ASTProofPosition{First: uint32(i + 1), Second: 0})
					premises = append(premises, proof)
					substitutions = append(substitutions, nil)
				}
			}
		}
	}
	// conc = node.Name(Eval(edge, IndParams...))
	nameDecl, err := m.RefGet(node.Name)
	if err != nil {
		return out, err
	}
	args := make([]ast.ASTHandle, len(edge.F.individualParams))
	for i, paramRef := range edge.F.individualParams {
		h, err := m.RefGet(paramRef)
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
		if args[i], err = value.Raw(); err != nil {
			return out, err
		}
	}
	conclusion, err := call.pin(m.MkApp(nameDecl, args))
	if err != nil {
		return out, err
	}
	// FORK FIX #93: conclude the original head
	head, _, _, err := witnessClauseShape(m, rawClause)
	if err != nil {
		return out, err
	}
	if shaped {
		kind, err := m.GetKind(head)
		if err != nil {
			return out, err
		}
		if kind == ast.ASTApp {
			decl, err := m.GetDecl(head)
			if err != nil {
				return out, err
			}
			arity, err := m.GetArity(decl)
			if err != nil {
				return out, err
			}
			if int(arity) <= len(args) {
				if conclusion, err = call.pin(m.MkApp(decl, args[:arity])); err != nil {
					return out, err
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
