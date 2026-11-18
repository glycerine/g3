package duality

// Source: muz/duality/duality_dl_interface.cpp, validate_counterexample (fork
// fix, AUDIT duality_itp_sat).
//
// Duality's solvers use MBQI only for interpolant quantifiers (mbqi.id=itp),
// so an SMT sat on a formula with other quantifiers is trusted although they
// were never checked: (=> (and (= x 1) (forall ((z Int)) (> z x))) err) was
// answered sat. When a step of the counterexample or a background axiom has a
// quantifier, the derivation is checked against the original rules with a
// full solver: each step's rule, instantiated with fresh constants, must
// derive its fact from the facts of the steps before it (child-first order),
// every predicate atom of its body standing for the disjunction of those
// facts. A derivation that fails, or that has a predicate in a non-positive
// position, makes the query report incompleteness.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// cexValidation holds the references one validate_counterexample call pins.
type cexValidation struct {
	m     *ast.ASTManager
	refs  []ast.ASTRef
	preds map[ast.ASTHandle]bool
}

func (v *cexValidation) pin(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := v.m.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	v.refs = append(v.refs, ref)
	return h, nil
}

func (v *cexValidation) close() error {
	var result error
	for len(v.refs) > 0 {
		i := len(v.refs) - 1
		result = errors.Join(result, v.m.DelRef(&v.refs[i]))
		v.refs = v.refs[:i]
	}
	return result
}

// cexSteps is cex_steps: the child-first list of the steps a counterexample
// uses, as print_proof visits them. It returns false when a used node has no
// outgoing edge.
func cexSteps(tree *RPFP, node *Node, steps *[]*Node) (bool, error) {
	edge := node.Outgoing
	if edge == nil {
		return false, nil
	}
	for _, child := range edge.Children {
		empty, err := tree.Empty(child)
		if err != nil {
			return false, err
		}
		if !empty {
			ok, err := cexSteps(tree, child, steps)
			if err != nil || !ok {
				return false, err
			}
		}
	}
	*steps = append(*steps, node)
	return true, nil
}

// stepClause is dd->clauses[dd->map[node->Outgoing->map]].
func stepClause(node *Node) (ast.ASTHandle, error) {
	source := node.Outgoing.Map
	if source == nil || source.Owner == nil {
		return ast.ASTHandle{}, ErrWrapperOwner
	}
	g := source.Owner
	index, found := g.clauseIndices[source]
	if !found || index < 0 || index >= len(g.sourceClauses) {
		return ast.ASTHandle{}, ErrWrapperOwner
	}
	return g.context.manager.RefGet(g.sourceClauses[index])
}

// stripForall returns the body of a universal quantifier, otherwise t.
func stripForall(m *ast.ASTManager, t ast.ASTHandle) (ast.ASTHandle, bool, error) {
	kind, err := m.GetKind(t)
	if err != nil || kind != ast.ASTQuantifier {
		return t, false, err
	}
	forall, err := m.IsForall(t)
	if err != nil || !forall {
		return t, false, err
	}
	body, err := m.GetQuantifierBody(t)
	return body, true, err
}

func (v *cexValidation) isBasic(h ast.ASTHandle, op int32) (bool, error) {
	kind, err := v.m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := v.m.GetFamilyID(h)
	if err != nil || family != v.m.BasicFamilyID() {
		return false, err
	}
	decl, err := v.m.GetDeclKind(h)
	return decl == op, err
}

// isUninterpretedApp is is_app(e) && to_app(e)->get_family_id() == null_family_id.
func (v *cexValidation) isUninterpretedApp(h ast.ASTHandle) (bool, error) {
	kind, err := v.m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := v.m.GetFamilyID(h)
	return family == ast.NullFamilyID, err
}

// containsPred is cex_contains_pred.
func (v *cexValidation) containsPred(e ast.ASTHandle, visited map[ast.ASTHandle]bool) (bool, error) {
	if visited[e] {
		return false, nil
	}
	visited[e] = true
	kind, err := v.m.GetKind(e)
	if err != nil {
		return false, err
	}
	switch kind {
	case ast.ASTApp:
		decl, err := v.m.GetDecl(e)
		if err != nil {
			return false, err
		}
		if v.preds[decl] {
			return true, nil
		}
		n, err := v.m.GetNumArgs(e)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < n; i++ {
			arg, err := v.m.GetArg(e, i)
			if err != nil {
				return false, err
			}
			found, err := v.containsPred(arg, visited)
			if err != nil || found {
				return found, err
			}
		}
		return false, nil
	case ast.ASTQuantifier:
		body, err := v.m.GetQuantifierBody(e)
		if err != nil {
			return false, err
		}
		return v.containsPred(body, visited)
	}
	return false, nil
}

// replaceAtoms is cex_replace_atoms: each predicate atom in a positive
// position of a body becomes the disjunction of the derived facts of its
// predicate. It returns false if a predicate occurs in another position.
func (v *cexValidation) replaceAtoms(facts []ast.ASTHandle, e ast.ASTHandle) (ast.ASTHandle, bool, error) {
	m := v.m
	kind, err := m.GetKind(e)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	switch kind {
	case ast.ASTApp:
		decl, err := m.GetDecl(e)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		n, err := m.GetNumArgs(e)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if v.preds[decl] {
			var disjuncts []ast.ASTHandle
			for _, fact := range facts {
				factDecl, err := m.GetDecl(fact)
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				if factDecl != decl {
					continue
				}
				var eqs []ast.ASTHandle
				for j := uint32(0); j < n; j++ {
					a, err := m.GetArg(e, j)
					if err != nil {
						return ast.ASTHandle{}, false, err
					}
					b, err := m.GetArg(fact, j)
					if err != nil {
						return ast.ASTHandle{}, false, err
					}
					eq, err := v.pin(m.MkEq(a, b))
					if err != nil {
						return ast.ASTHandle{}, false, err
					}
					eqs = append(eqs, eq)
				}
				conjunction, err := v.pin(m.MkAndArray(eqs))
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				disjuncts = append(disjuncts, conjunction)
			}
			res, err := v.pin(m.MkOrArray(disjuncts))
			return res, err == nil, err
		}
		and, err := v.isBasic(e, ast.ASTOpAnd)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		or, err := v.isBasic(e, ast.ASTOpOr)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		implies, err := v.isBasic(e, ast.ASTOpImplies)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if and || or || implies {
			args := make([]ast.ASTHandle, 0, n)
			for i := uint32(0); i < n; i++ {
				arg, err := m.GetArg(e, i)
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				if implies && i == 0 {
					found, err := v.containsPred(arg, make(map[ast.ASTHandle]bool))
					if err != nil || found {
						return ast.ASTHandle{}, false, err
					}
				} else {
					replaced, ok, err := v.replaceAtoms(facts, arg)
					if err != nil || !ok {
						return ast.ASTHandle{}, false, err
					}
					arg = replaced
				}
				args = append(args, arg)
			}
			res, err := v.pin(m.MkApp(decl, args))
			return res, err == nil, err
		}
		found, err := v.containsPred(e, make(map[ast.ASTHandle]bool))
		if err != nil || found {
			return ast.ASTHandle{}, false, err
		}
		return e, true, nil
	case ast.ASTQuantifier:
		body, err := m.GetQuantifierBody(e)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		replaced, ok, err := v.replaceAtoms(facts, body)
		if err != nil || !ok {
			return ast.ASTHandle{}, false, err
		}
		res, err := v.pin(m.UpdateQuantifierBody(e, replaced))
		return res, err == nil, err
	}
	return e, true, nil
}

// Validate is validate_counterexample: it reports whether the counterexample
// is a derivation of the query from the original rules. A counterexample
// whose step rules and background axioms have no quantifier is accepted
// without a check. The factory and background axioms are borrowed.
func (c *Counterexample) Validate(factory solver.Factory, background ...*Expr) (valid bool, result error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if c.tree == nil || c.root == nil || factory == nil {
		return false, ErrWrapperOwner
	}
	tree := c.tree
	if err := tree.open(); err != nil {
		return false, err
	}
	m := tree.context.manager
	v := &cexValidation{m: m}
	defer func() { result = errors.Join(result, v.close()) }()
	axioms := make([]ast.ASTHandle, 0, len(background))
	for _, axiom := range background {
		if err := axiom.open(); err != nil {
			return false, err
		}
		if axiom.value.object.context != tree.context {
			return false, ErrContextMismatch
		}
		h, err := axiom.Raw()
		if err != nil {
			return false, err
		}
		axioms = append(axioms, h)
	}
	var steps []*Node
	ok, err := cexSteps(tree, c.root, &steps)
	if err != nil || !ok {
		return false, err
	}
	// only a step rule or an axiom with a quantifier needs the check
	quantified := false
	for _, axiom := range axioms {
		has, err := m.HasQuantifiers(axiom)
		if err != nil {
			return false, err
		}
		quantified = quantified || has
	}
	for _, step := range steps {
		t, err := stepClause(step)
		if err != nil {
			return false, err
		}
		t, _, err = stripForall(m, t)
		if err != nil {
			return false, err
		}
		has, err := m.HasQuantifiers(t)
		if err != nil {
			return false, err
		}
		quantified = quantified || has
	}
	if !quantified {
		return true, nil
	}
	// the predicate symbols are the uninterpreted clause heads
	source := c.root.Outgoing.Map.Owner
	v.preds = make(map[ast.ASTHandle]bool)
	for _, ref := range source.sourceClauses {
		clause, err := m.RefGet(ref)
		if err != nil {
			return false, err
		}
		kind, err := m.GetKind(clause)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTQuantifier {
			if clause, err = m.GetQuantifierBody(clause); err != nil {
				return false, err
			}
		}
		implies, err := v.isBasic(clause, ast.ASTOpImplies)
		if err != nil {
			return false, err
		}
		if implies {
			if clause, err = m.GetArg(clause, 1); err != nil {
				return false, err
			}
		}
		uninterpreted, err := v.isUninterpretedApp(clause)
		if err != nil {
			return false, err
		}
		if uninterpreted {
			decl, err := m.GetDecl(clause)
			if err != nil {
				return false, err
			}
			v.preds[decl] = true
		}
	}
	var facts, constraints []ast.ASTHandle
	// expr_ref_vector facts(m), constraints(m): owned until the end and
	// destroyed constraints first, each front to back.
	var factRefs, constraintRefs []ast.ASTRef
	keep := func(refs *[]ast.ASTRef, h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		*refs = append(*refs, ref)
		return h, nil
	}
	defer func() {
		for _, refs := range []*[]ast.ASTRef{&constraintRefs, &factRefs} {
			for i := range *refs {
				result = errors.Join(result, m.DelRef(&(*refs)[i]))
			}
		}
	}()
	// The loop body's locals (expr_ref inst, body, head, the skolem and
	// args vectors) die with each iteration; facts and constraints, and
	// what they reference, live to the end.
	var iteration []ast.ASTRef
	release := func(refs *[]ast.ASTRef) error {
		var result error
		for i := len(*refs) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&(*refs)[i]))
		}
		*refs = nil
		return result
	}
	defer func() { result = errors.Join(result, release(&iteration)) }()
	local := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		iteration = append(iteration, ref)
		return h, nil
	}
	for i, step := range steps {
		if err := release(&iteration); err != nil {
			return false, err
		}
		t, err := stepClause(step)
		if err != nil {
			return false, err
		}
		inst := t
		if body, forall, err := stripForall(m, t); err != nil {
			return false, err
		} else if forall {
			n, err := m.GetNumDeclarations(t)
			if err != nil {
				return false, err
			}
			// expr_ref inst is declared before the skolem vector, which
			// dies at the end of this block.
			out, err := m.NewRef(ast.ASTHandle{})
			if err != nil {
				return false, err
			}
			iteration = append(iteration, out)
			var skolemRefs []ast.ASTRef
			skolem := make([]ast.ASTHandle, n)
			for j := uint32(0); j < n; j++ {
				sort, err := m.GetDeclarationSort(t, j)
				if err != nil {
					return false, errors.Join(err, release(&skolemRefs))
				}
				h, err := m.MkFreshConst("@cex", sort)
				if err == nil {
					var ref ast.ASTRef
					if ref, err = m.NewRef(h); err == nil {
						skolemRefs = append(skolemRefs, ref)
					}
				}
				if err != nil {
					return false, errors.Join(err, release(&skolemRefs))
				}
				skolem[n-1-j] = h
			}
			substitution, err := rewriter.NewVarSubstitution(m, false)
			if err != nil {
				return false, errors.Join(err, release(&skolemRefs))
			}
			err = errors.Join(substitution.Substitute(body, skolem, out), substitution.Close())
			// ~var_subst, then ~expr_ref_vector skolem (forward)
			for j := range skolemRefs {
				err = errors.Join(err, m.DelRef(&skolemRefs[j]))
			}
			if err != nil {
				return false, err
			}
			if inst, err = m.RefGet(out); err != nil {
				return false, err
			}
		}
		// the clause shape FromClauses gives it
		var body, head ast.ASTHandle
		fact, err := v.isUninterpretedApp(inst)
		if err != nil {
			return false, err
		}
		canonical := false
		if !fact {
			implies, err := v.isBasic(inst, ast.ASTOpImplies)
			if err != nil {
				return false, err
			}
			if implies {
				if head, err = m.GetArg(inst, 1); err != nil {
					return false, err
				}
				uninterpreted, err := v.isUninterpretedApp(head)
				if err != nil {
					return false, err
				}
				canonical = m.IsFalse(head) || uninterpreted
			}
		}
		switch {
		case fact:
			if body, err = m.MkTrue(); err != nil {
				return false, err
			}
			head = inst
		case canonical:
			if body, err = m.GetArg(inst, 0); err != nil {
				return false, err
			}
		default:
			if body, err = local(m.MkNot(inst)); err != nil {
				return false, err
			}
			if head, err = m.MkFalse(); err != nil {
				return false, err
			}
		}
		// the query step is the root; every other step derives a fact
		if m.IsFalse(head) != (i+1 == len(steps)) {
			return false, nil
		}
		// cex_replace_atoms' own locals die when it returns; the condition
		// is held by constraints.
		mark := len(v.refs)
		condition, ok, err := v.replaceAtoms(facts, body)
		if err == nil && ok {
			condition, err = keep(&constraintRefs, condition, nil)
		}
		for j := mark; j < len(v.refs); j++ {
			err = errors.Join(err, m.DelRef(&v.refs[j]))
		}
		v.refs = v.refs[:mark]
		if err != nil || !ok {
			return false, err
		}
		constraints = append(constraints, condition)
		if !m.IsFalse(head) {
			f, err := m.GetDecl(head)
			if err != nil {
				return false, err
			}
			arity, err := m.GetArity(f)
			if err != nil {
				return false, err
			}
			args := make([]ast.ASTHandle, arity)
			for j := uint32(0); j < arity; j++ {
				domain, err := m.GetDomain(f, j)
				if err != nil {
					return false, err
				}
				// expr_ref_vector args: a block local
				if args[j], err = local(m.MkFreshConst("@cex", domain)); err != nil {
					return false, err
				}
				arg, err := m.GetArg(head, j)
				if err != nil {
					return false, err
				}
				eq, err := m.MkEq(arg, args[j])
				eq, err = keep(&constraintRefs, eq, err)
				if err != nil {
					return false, err
				}
				constraints = append(constraints, eq)
			}
			application, err := m.MkApp(f, args)
			application, err = keep(&factRefs, application, err)
			if err != nil {
				return false, err
			}
			facts = append(facts, application)
		}
	}
	// the last iteration's locals die before the solver is made
	if err := release(&iteration); err != nil {
		return false, err
	}
	// query runs with proofs on; MBQI with proofs can loop on a valid
	// forall-exists body, so check without them (scoped_no_proof).
	noProof, err := ast.NewScopedNoProof(m)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, noProof.Close()) }()
	// mk_smt_solver(m, params_ref(), symbol::null)
	native, err := factory.New(m, nil, false, true, false, "")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, native.Close()) }()
	for _, axiom := range axioms {
		if err := native.Assert(axiom); err != nil {
			return false, err
		}
	}
	for _, constraint := range constraints {
		if err := native.Assert(constraint); err != nil {
			return false, err
		}
	}
	status, err := native.CheckSat(nil)
	if err != nil {
		return false, err
	}
	return status == lbool.True, nil
}
