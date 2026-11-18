package duality

// Source: src/muz/duality/duality_dl_interface.cpp (FORK FIX #63, AUDIT
// duality_01-F4): witness_clause_shape, witness_node_fact,
// witness_positions and find_step_witness.
//
// print_proof and extract_proof take each rule variable's value from the
// original rule, solved for its variables: the head must equal the step's
// proved fact, each used body predicate must equal its child's proved fact,
// and the rest of the body must hold together with the raw background
// axioms. Without a solution they keep the counterexample model's values.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// witnessClauseShape is witness_clause_shape: a clause (quantifier body)
// split into its head and body conjuncts. body is null for a fact.
func witnessClauseShape(m *ast.ASTManager, clause ast.ASTHandle) (head, body ast.ASTHandle, atoms []ast.ASTHandle, err error) {
	implies, err := isBasicOp(m, clause, ast.ASTOpImplies)
	if err != nil || !implies {
		return clause, ast.ASTHandle{}, nil, err
	}
	if body, err = m.GetArg(clause, 0); err != nil {
		return
	}
	and, err := isBasicOp(m, body, ast.ASTOpAnd)
	if err != nil {
		return
	}
	if and {
		n, e := m.GetNumArgs(body)
		if e != nil {
			return head, body, nil, e
		}
		for i := uint32(0); i < n; i++ {
			a, e := m.GetArg(body, i)
			if e != nil {
				return head, body, nil, e
			}
			atoms = append(atoms, a)
		}
	} else {
		atoms = append(atoms, body)
	}
	head, err = m.GetArg(clause, 1)
	return
}

func isBasicOp(m *ast.ASTManager, h ast.ASTHandle, op int32) (bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := m.GetFamilyID(h)
	if err != nil || family != m.BasicFamilyID() {
		return false, err
	}
	decl, err := m.GetDeclKind(h)
	return decl == op, err
}

// witnessNodeFact is witness_node_fact: the fact a counterexample node
// proves, its original rule's head symbol applied to the evaluated edge
// parameters. The fact is pinned in call.
func (g *RPFP) witnessNodeFact(call *proofWitnessCall, node *Node) (ast.ASTHandle, bool, error) {
	m := g.context.manager
	edge := node.Outgoing
	t, err := stepClause(node)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if t, _, err = stripForall(m, t); err != nil {
		return ast.ASTHandle{}, false, err
	}
	head, _, _, err := witnessClauseShape(m, t)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	kind, err := m.GetKind(head)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	f, err := m.GetDecl(head)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	arity, err := m.GetArity(f)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if int(arity) > len(edge.F.individualParams) {
		return ast.ASTHandle{}, false, nil
	}
	args := make([]ast.ASTHandle, arity)
	for i := range args {
		h, err := m.RefGet(edge.F.individualParams[i])
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		param, err := NewExpr(g.context, h)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		call.terms = append(call.terms, param)
		value, err := g.Eval(edge, param)
		if value != nil {
			call.terms = append(call.terms, value)
		}
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if args[i], err = value.Raw(); err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	fact, err := call.pin(m.MkApp(f, args))
	return fact, err == nil, err
}

// witnessPositions is witness_positions: the 1-based body positions of each
// distinct predicate atom of a clause (the quantifier body), one group per
// edge child. ok is false on a malformed program.
func (g *RPFP) witnessPositions(source *RPFP, clause ast.ASTHandle) (positions [][]uint32, ok bool, err error) {
	m := g.context.manager
	// the predicate symbols are the uninterpreted clause heads
	preds := make(map[ast.ASTHandle]bool)
	for _, ref := range source.sourceClauses {
		c, err := m.RefGet(ref)
		if err != nil {
			return nil, false, err
		}
		kind, err := m.GetKind(c)
		if err != nil {
			return nil, false, err
		}
		if kind == ast.ASTQuantifier {
			if c, err = m.GetQuantifierBody(c); err != nil {
				return nil, false, err
			}
		}
		implies, err := isBasicOp(m, c, ast.ASTOpImplies)
		if err != nil {
			return nil, false, err
		}
		if implies {
			if c, err = m.GetArg(c, 1); err != nil {
				return nil, false, err
			}
		}
		if kind, err = m.GetKind(c); err != nil {
			return nil, false, err
		}
		if kind != ast.ASTApp {
			return nil, false, nil
		}
		family, err := m.GetFamilyID(c)
		if err != nil {
			return nil, false, err
		}
		if family == ast.NullFamilyID {
			decl, err := m.GetDecl(c)
			if err != nil {
				return nil, false, err
			}
			preds[decl] = true
		}
	}
	_, _, atoms, err := witnessClauseShape(m, clause)
	if err != nil {
		return nil, false, err
	}
	seen := make(map[ast.ASTHandle]int)
	for i, a := range atoms {
		kind, err := m.GetKind(a)
		if err != nil {
			return nil, false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		decl, err := m.GetDecl(a)
		if err != nil {
			return nil, false, err
		}
		if !preds[decl] {
			continue
		}
		index, found := seen[a]
		if !found {
			index = len(positions)
			seen[a] = index
			positions = append(positions, nil)
		}
		positions[index] = append(positions[index], uint32(i+1))
	}
	return positions, true, nil
}

// findStepWitness is find_step_witness: the values of the original rule's
// bound variables for this step, indexed by declaration, owned by the
// caller. ok is false if none are found; the caller then keeps the
// counterexample model's values. axioms are the raw background axioms.
func (g *RPFP) findStepWitness(factory solver.Factory, root *Node, axioms []*Expr) (witness []ast.ASTRef, ok bool, resultErr error) {
	m := g.context.manager
	if factory == nil {
		return nil, false, ErrWrapperOwner
	}
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return nil, false, err
	}
	call := &proofWitnessCall{manager: m, proofScope: scope}
	defer func() {
		if err := call.close(); err != nil {
			g.pendingProofWitnesses = append(g.pendingProofWitnesses, call)
			resultErr = errors.Join(resultErr, err)
		}
		if resultErr != nil || !ok {
			for i := range witness {
				resultErr = errors.Join(resultErr, m.DelRef(&witness[i]))
			}
			witness = nil
		}
	}()
	edge := root.Outgoing
	t, err := stepClause(root)
	if err != nil {
		return nil, false, err
	}
	clause, forall, err := stripForall(m, t)
	if err != nil || !forall {
		return nil, false, err
	}
	// body positions of each distinct predicate atom, one group per edge child
	positions, shaped, err := g.witnessPositions(edge.Map.Owner, clause)
	if err != nil || !shaped || len(positions) != len(edge.Children) {
		return nil, false, err
	}
	// instantiate the rule with fresh constants
	n, err := m.GetNumDeclarations(t)
	if err != nil {
		return nil, false, err
	}
	skolem := make([]ast.ASTHandle, n)
	for j := uint32(0); j < n; j++ {
		sort, err := m.GetDeclarationSort(t, j)
		if err != nil {
			return nil, false, err
		}
		if skolem[n-1-j], err = call.pin(m.MkFreshConst("@proof", sort)); err != nil {
			return nil, false, err
		}
	}
	if call.substitution, err = rewriter.NewVarSubstitution(m, false); err != nil {
		return nil, false, err
	}
	instantiated, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, err
	}
	call.refs = append(call.refs, instantiated)
	if err = call.substitution.Substitute(clause, skolem, instantiated); err != nil {
		return nil, false, err
	}
	inst, err := m.RefGet(instantiated)
	if err != nil {
		return nil, false, err
	}
	ihead, ibody, iatoms, err := witnessClauseShape(m, inst)
	if err != nil {
		return nil, false, err
	}
	if ibody.IsNull() {
		if ibody, err = m.MkTrue(); err != nil {
			return nil, false, err
		}
		iatoms = append(iatoms, ibody)
	}
	// predicate atoms of unused children are false, used ones are true and
	// their arguments equal the child's proved fact
	replacements := make(map[ast.ASTHandle]ast.ASTHandle)
	var replaced []ast.ASTHandle
	replace := func(a, b ast.ASTHandle) {
		if _, found := replacements[a]; !found {
			replaced = append(replaced, a)
		}
		replacements[a] = b
	}
	falsity, err := m.MkFalse()
	if err != nil {
		return nil, false, err
	}
	truth, err := m.MkTrue()
	if err != nil {
		return nil, false, err
	}
	for _, group := range positions {
		for _, position := range group {
			if int(position) > len(iatoms) {
				return nil, false, nil
			}
			replace(iatoms[position-1], falsity)
		}
	}
	var constraints []ast.ASTHandle
	equal := func(application, fact ast.ASTHandle) (bool, error) {
		kind, err := m.GetKind(application)
		if err != nil || kind != ast.ASTApp {
			return false, err
		}
		d1, err := m.GetDecl(application)
		if err != nil {
			return false, err
		}
		d2, err := m.GetDecl(fact)
		if err != nil || d1 != d2 {
			return false, err
		}
		count, err := m.GetNumArgs(application)
		if err != nil {
			return false, err
		}
		for j := uint32(0); j < count; j++ {
			a, err := m.GetArg(application, j)
			if err != nil {
				return false, err
			}
			b, err := m.GetArg(fact, j)
			if err != nil {
				return false, err
			}
			eq, err := call.pin(m.MkEq(a, b))
			if err != nil {
				return false, err
			}
			constraints = append(constraints, eq)
		}
		return true, nil
	}
	for i, child := range edge.Children {
		empty, err := g.Empty(child)
		if err != nil {
			return nil, false, err
		}
		if empty {
			continue
		}
		fact, found, err := g.witnessNodeFact(call, child)
		if err != nil || !found {
			return nil, false, err
		}
		for _, position := range positions[i] {
			a := iatoms[position-1]
			matched, err := equal(a, fact)
			if err != nil || !matched {
				return nil, false, err
			}
			replace(a, truth)
		}
	}
	conclusion, found, err := g.witnessNodeFact(call, root)
	if err != nil || !found {
		return nil, false, err
	}
	if matched, err := equal(ihead, conclusion); err != nil || !matched {
		return nil, false, err
	}
	rep, err := rewriter.NewExprSafeReplace(m)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, rep.Close()) }()
	for _, a := range replaced {
		if err := rep.Insert(a, replacements[a]); err != nil {
			return nil, false, err
		}
	}
	condition, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, false, err
	}
	call.refs = append(call.refs, condition)
	if err = rep.Apply(ibody, condition); err != nil {
		return nil, false, err
	}
	c, err := m.RefGet(condition)
	if err != nil {
		return nil, false, err
	}
	constraints = append(constraints, c)
	// solve in a fresh solver, with the background axioms
	if call.native, err = factory.New(m, nil, false, true, false, ""); err != nil {
		return nil, false, err
	}
	for _, axiom := range axioms {
		if err := axiom.open(); err != nil {
			return nil, false, err
		}
		if axiom.value.object.context != g.context {
			return nil, false, ErrContextMismatch
		}
		h, err := axiom.Raw()
		if err != nil {
			return nil, false, err
		}
		if err := call.native.Assert(h); err != nil {
			return nil, false, err
		}
	}
	for _, constraint := range constraints {
		if err := call.native.Assert(constraint); err != nil {
			return nil, false, err
		}
	}
	status, err := call.native.CheckSat(nil)
	if err != nil || status != lbool.True {
		return nil, false, err
	}
	if call.model, err = call.native.AcquireModel(); err != nil || call.model == nil {
		return nil, false, err
	}
	if call.evaluator, err = model.NewModelEvaluator(call.model, g.evaluatorConfig); err != nil {
		return nil, false, err
	}
	if err = call.evaluator.SetModelCompletion(true); err != nil {
		return nil, false, err
	}
	for j := uint32(0); j < n; j++ {
		value, err := call.evaluator.EvaluateValue(skolem[n-1-j])
		if err != nil {
			return nil, false, err
		}
		witness = append(witness, value)
	}
	return witness, true, nil
}
