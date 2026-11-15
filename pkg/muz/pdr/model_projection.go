package pdr

// Source: context::create_children model substitution and predecessor-literal
// partitioning in src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/qe"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

// modelSubstitution is a create_children block's expr_substitution sub(m)
// and expr_ref tmp(m): each source maps to tmp = mev.eval(M, source).
type modelSubstitution struct {
	c   *Context
	sub ast.ASTExprSubstitution
	tmp ast.ASTRef
}

func (c *Context) newModelSubstitution() (*modelSubstitution, error) {
	sub, err := c.manager.NewExprSubstitution()
	if err != nil {
		return nil, err
	}
	tmp, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, errors.Join(err, c.manager.DelExprSubstitution(&sub))
	}
	return &modelSubstitution{c: c, sub: sub, tmp: tmp}, nil
}

// add is tmp = mev.eval(M, source); sub.insert(source, tmp, pr).
func (s *modelSubstitution) add(modelCore *model.ModelCore, source ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.c.manager
	evaluated, err := s.c.implicant.EvalExpr(modelCore.DerivedModel(), source)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := m.RefGet(evaluated)
	if err == nil {
		err = m.SetRef(s.tmp, value)
	}
	if err = errors.Join(err, m.DelRef(&evaluated)); err != nil {
		return ast.ASTHandle{}, err
	}
	if value, err = m.RefGet(s.tmp); err != nil {
		return ast.ASTHandle{}, err
	}
	return value, m.ExprSubstitutionInsert(s.sub, source, value, ast.ASTHandle{}, ast.ASTDependency{})
}

// apply is if (!rep) rep = mk_expr_simp_replacer(m); rep->set_substitution(&sub);
// (*rep)(target): target is rewritten in place. rep is create_children's
// scoped_ptr, so its cache outlives this block's sub.
func (s *modelSubstitution) apply(rep **rewriter.SimplifyingExprReplacer, target ast.ASTRef) error {
	c := s.c
	if *rep == nil {
		config := c.owner.contexts.config
		replacer, err := rewriter.NewSimplifyingExprReplacer(c.manager, config.Params, config.Globals, c.owner.contexts.params)
		if err != nil {
			if replacer != nil {
				err = errors.Join(err, replacer.Close())
			}
			return err
		}
		*rep = replacer
	}
	if err := (*rep).SetSubstitution(&s.sub); err != nil {
		return err
	}
	term, err := c.manager.RefGet(target)
	if err != nil {
		return err
	}
	return (*rep).Rewrite(term, target)
}

// close ends the block: tmp dies, then sub.
func (s *modelSubstitution) close() error {
	err := s.c.owner.releaseNotAndRef(&s.tmp)
	return errors.Join(err, s.c.manager.DelExprSubstitution(&s.sub))
}

func (c *Context) createChildrenProjected(node *ModelNode) (resultErr error) {
	proofsEnabled, err := c.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofsEnabled {
		if err = c.manager.ToggleProofMode(ast.ASTProofDisabled); err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, c.manager.ToggleProofMode(ast.ASTProofEnabled)) }()
	}
	modelCore, found, err := node.Model()
	if err != nil {
		return err
	}
	if !found {
		return ast.ErrASTDomain
	}
	rule, err := node.transformer.selectRule(modelCore)
	if err != nil {
		return err
	}
	if err = node.SetRule(rule.rule); err != nil {
		return err
	}
	node.selected = rule

	signatures, err := node.transformer.sig()
	if err != nil {
		return err
	}
	eliminated := make([]ast.ASTHandle, 0, len(signatures)+len(rule.auxiliaries))
	for _, signature := range signatures {
		declaration, x := c.owner.OldToNew(signature, 0)
		if x != nil {
			return x
		}
		application, x := c.manager.MkApp(declaration, nil)
		if x != nil {
			return x
		}
		eliminated = append(eliminated, application)
	}
	eliminated = append(eliminated, rule.auxiliaries...)

	forms, err := c.manager.FlattenAnd([]ast.ASTHandle{rule.transition, node.state})
	if err != nil {
		return err
	}
	references := make([]ast.ASTRef, 0, len(forms))
	defer func() {
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&references[index]))
		}
	}()
	// flatten_and owns every entry, including newly constructed negations.
	pinForms := func(forms []ast.ASTHandle) error {
		for _, form := range forms {
			ref, err := c.manager.NewRef(form)
			if err != nil {
				return err
			}
			references = append(references, ref)
		}
		return nil
	}
	if err = pinForms(forms); err != nil {
		return err
	}
	// Phi's entries (the minimized literals) within references.
	phiStart, phiEnd := 0, 0
	if derived := modelCore.DerivedModel(); c.implicant != nil && derived != nil {
		var minimized []ast.ASTRef
		var x error
		if c.useModel {
			minimized, x = c.implicant.MinimizeModel(forms, derived)
		} else {
			minimized, x = c.implicant.MinimizeLiterals(forms, derived)
		}
		if x != nil {
			return fmt.Errorf("muz/pdr: implicant minimization: %w", x)
		}
		phiStart = len(references)
		references = append(references, minimized...)
		phiEnd = len(references)
		forms = forms[:0]
		for _, reference := range minimized {
			formula, y := c.manager.RefGet(reference)
			if y != nil {
				return y
			}
			forms = append(forms, formula)
		}
	}
	variableRefs := make([]ast.ASTRef, 0, len(eliminated))
	defer func() {
		for index := len(variableRefs) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&variableRefs[index]))
		}
	}()
	for _, variable := range eliminated {
		ref, x := c.manager.NewRef(variable)
		if x != nil {
			return x
		}
		variableRefs = append(variableRefs, ref)
	}
	// scoped_ptr<expr_replacer> rep is declared after vars and before qe:
	// made on first use, it dies after qe and before vars.
	var rep *rewriter.SimplifyingExprReplacer
	defer func() {
		if rep != nil {
			resultErr = errors.Join(resultErr, rep.Close())
		}
	}()
	// qe_lite qe(m, m_params.p) is declared after vars and before phi1: it
	// dies at create_children's end, after phi1 and before vars.
	lite, err := c.newLite()
	if err != nil {
		return err
	}
	defer func() {
		if x := lite.Close(); x != nil {
			c.pendingLites = append(c.pendingLites, lite)
			resultErr = errors.Join(resultErr, x)
		}
	}()
	formulaRef, err := c.normalizeProjectedPhiWith(lite, forms, &variableRefs, modelCore.DerivedModel())
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&formulaRef)) }()
	eliminated = eliminated[:0]
	for _, reference := range variableRefs {
		variable, x := c.manager.RefGet(reference)
		if x != nil {
			return x
		}
		eliminated = append(eliminated, variable)
	}
	if len(eliminated) != 0 {
		// expr_substitution sub(m); sub.insert(vars[i], mev.eval(M, vars[i]));
		// rep->set_substitution(&sub); (*rep)(phi1).
		sub, x := c.newModelSubstitution()
		if x != nil {
			return x
		}
		for _, variable := range eliminated {
			if _, x = sub.add(modelCore, variable); x != nil {
				break
			}
		}
		if x == nil {
			x = sub.apply(&rep, formulaRef)
		}
		if x = errors.Join(x, sub.close()); x != nil {
			return fmt.Errorf("muz/pdr: projected model substitution: %w", x)
		}
	}
	// Phi.reset(); flatten_and(phi1, Phi).
	if phiEnd > phiStart {
		for index := phiStart; index < phiEnd; index++ {
			if x := c.owner.releaseNotAndRef(&references[index]); x != nil {
				return x
			}
		}
		references = append(references[:phiStart], references[phiEnd:]...)
	}
	formula, err := c.manager.RefGet(formulaRef)
	if err != nil {
		return err
	}
	projected, err := c.manager.FlattenAnd([]ast.ASTHandle{formula})
	if err != nil {
		return err
	}
	if err = pinForms(projected); err != nil {
		return err
	}

	// vector<expr_ref_vector> child_states is declared after phi1: it dies
	// before phi1, each expr_ref_vector front to back.
	childStates := make([][]ast.ASTHandle, len(rule.predecessors))
	childRefs := make([][]ast.ASTRef, len(rule.predecessors))
	defer func() {
		for _, refs := range childRefs {
			for index := range refs {
				resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&refs[index]))
			}
		}
	}()
	retainChild := func(index uint32, formula ast.ASTHandle) error {
		if index >= uint32(len(childStates)) {
			return ast.ErrASTDomain
		}
		// child_states[index].push_back(...): an expr_ref_vector of the
		// old-name literals; formula_o2n renames their conjunction below.
		reference, x := c.manager.NewRef(formula)
		if x != nil {
			return x
		}
		childRefs[index] = append(childRefs[index], reference)
		childStates[index] = append(childStates[index], formula)
		return nil
	}
	for _, literal := range projected {
		indices, x := c.owner.mux.CollectIndices(literal)
		if x != nil {
			return fmt.Errorf("muz/pdr: projected state-index collection: %w", x)
		}
		if len(indices) == 0 {
			if x = verbosePP(c.manager, 3, "Skipping ", literal, "\n"); x != nil {
				return x
			}
			continue
		}
		if len(indices) == 1 {
			if x = retainChild(indices[0], literal); x != nil {
				return x
			}
			continue
		}
		variables, x := c.owner.mux.CollectVariables(literal)
		if x != nil {
			return fmt.Errorf("muz/pdr: projected state-variable collection: %w", x)
		}
		// A block with its own expr_substitution sub(m) and expr_ref tmp(m).
		sub, x := c.newModelSubstitution()
		if x != nil {
			return x
		}
		x = c.projectCoupledLiteral(sub, &rep, modelCore, rule, indices, variables, literal, retainChild)
		if x = errors.Join(x, sub.close()); x != nil {
			return fmt.Errorf("muz/pdr: coupled-state model substitution: %w", x)
		}
	}

	// expr_ref n_cube(m) is declared before the loop, after child_states:
	// each formula_o2n replaces it, and it dies first.
	var nCube ast.ASTRef
	defer func() {
		if nCube != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&nCube))
		}
	}()
	for predecessorIndex, predecessor := range rule.predecessors {
		if err = c.addProjectedChild(node, predecessor, uint32(predecessorIndex), childStates[predecessorIndex], &nCube); err != nil {
			return err
		}
	}
	return node.CheckPreClosed()
}

// projectCoupledLiteral is create_children's case for a literal over several
// predecessors: for each later index's variable, tmp = mev.eval(M, v);
// sub.insert(v, tmp); child_states[j].push_back(m.mk_eq(v, tmp)); then
// tmp = Phi[i]; (*rep)(tmp); child_states[indices[0]].push_back(tmp).
func (c *Context) projectCoupledLiteral(sub *modelSubstitution, rep **rewriter.SimplifyingExprReplacer, modelCore *model.ModelCore, rule *normalizedRule, indices []uint32, variables [][]ast.ASTHandle, literal ast.ASTHandle, retainChild func(uint32, ast.ASTHandle) error) error {
	for _, index := range indices[1:] {
		if index >= uint32(len(variables)) || index >= uint32(len(rule.predecessors)) {
			return ast.ErrASTDomain
		}
		for _, variable := range variables[index] {
			value, err := sub.add(modelCore, variable)
			if err != nil {
				return err
			}
			equality, err := c.manager.MkEq(variable, value)
			if err != nil {
				return err
			}
			if err = retainChild(index, equality); err != nil {
				return err
			}
		}
	}
	if err := c.manager.SetRef(sub.tmp, literal); err != nil {
		return err
	}
	if err := sub.apply(rep, sub.tmp); err != nil {
		return err
	}
	reduced, err := c.manager.RefGet(sub.tmp)
	if err != nil {
		return err
	}
	return retainChild(indices[0], reduced)
}

// addProjectedChild is one iteration of create_children's predecessor loop:
// expr_ref o_cube = m_pm.mk_and(child_states[i]); m_pm.formula_o2n(o_cube,
// n_cube, i). The returned n_cube is the caller's, o_cube dies here.
func (c *Context) addProjectedChild(node *ModelNode, predecessor *PredicateTransformer, index uint32, states []ast.ASTHandle, nCube *ast.ASTRef) (resultErr error) {
	oCube, err := c.owner.AndRef(states)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&oCube)) }()
	old, err := c.manager.RefGet(oCube)
	if err != nil {
		return err
	}
	renamed, err := c.owner.FormulaOldToNewRef(old, index, true)
	if renamed != (ast.ASTRef{}) {
		// m_pm.formula_o2n(o_cube, n_cube, i) replaces the previous n_cube.
		if *nCube != (ast.ASTRef{}) {
			err = errors.Join(err, c.owner.releaseNotAndRef(nCube))
		}
		*nCube = renamed
	}
	if err != nil {
		return fmt.Errorf("muz/pdr: child-state renaming: %w", err)
	}
	cube, err := c.manager.RefGet(*nCube)
	if err != nil {
		return err
	}
	child, err := NewModelNode(node, cube, predecessor, node.level-1)
	if err != nil {
		return fmt.Errorf("muz/pdr: projected model-node construction: %w", err)
	}
	c.nodes++
	if err = c.search.AddLeaf(child); err != nil {
		return err
	}
	closed := "open"
	if child.IsClosed() {
		closed = "closed"
	}
	if err = verbosePP(c.manager, 2, "Predecessor: ", cube, " "+closed+"\n"); err != nil {
		return err
	}
	if child.depth > c.maxDepth {
		c.maxDepth = child.depth
	}
	return nil
}

// normalizeProjectedPhi follows create_children: Boolean conjunction, QE,
// optional disequality reduction, then the fixedpoint theory rewriter.
func (c *Context) normalizeProjectedPhi(forms []ast.ASTHandle, vars *[]ast.ASTRef, md *model.Model) (result ast.ASTRef, resultErr error) {
	return c.normalizeProjectedPhiWith(c.lite, forms, vars, md)
}

func (c *Context) normalizeProjectedPhiWith(lite *qe.Lite, forms []ast.ASTHandle, vars *[]ast.ASTRef, md *model.Model) (result ast.ASTRef, resultErr error) {
	result, resultErr = c.owner.AndRef(forms)
	if resultErr != nil {
		return result, resultErr
	}
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.owner.releaseNotAndRef(&result))
		}
	}()
	if lite != nil {
		if resultErr = lite.EliminateConstants(vars, result); resultErr != nil {
			return result, fmt.Errorf("muz/pdr: QE-lite projection: %w", resultErr)
		}
	}
	if !c.useModel && c.implicant != nil && md != nil {
		formula, err := c.manager.RefGet(result)
		if err != nil {
			return result, err
		}
		reduced, err := c.reduceDisequalities(md, 3, formula)
		if err != nil {
			return result, fmt.Errorf("muz/pdr: disequality reduction: %w", err)
		}
		formula, err = c.manager.RefGet(reduced)
		if err == nil {
			err = c.manager.SetRef(result, formula)
		}
		err = errors.Join(err, c.owner.releaseNotAndRef(&reduced))
		if err != nil {
			return result, err
		}
	}
	// Nil-Globals contexts are a supported partial Go configuration and have
	// no parameter service from which to construct the source rewriter.
	if err := c.ensureProjectionTheory(); err != nil {
		return result, err
	}
	if c.projectionTheory == nil {
		return result, nil
	}
	formula, err := c.manager.RefGet(result)
	if err != nil {
		return result, err
	}
	if err = c.projectionTheory.Rewrite(formula, result); err != nil {
		return result, err
	}
	return result, nil
}

// ensureProjectionTheory also permits retry after a partially constructed
// rewriter refused cleanup during an earlier attempt.
func (c *Context) ensureProjectionTheory() error {
	config := c.owner.contexts.config
	if config.Globals == nil {
		return nil
	}
	if c.projectionTheory != nil && !c.projectionTheoryReady {
		if err := c.projectionTheory.Close(); err != nil {
			return err
		}
		c.projectionTheory = nil
	}
	if c.projectionTheory == nil {
		var err error
		c.projectionTheory, err = rewriter.NewTheoryRewriter(c.manager, config.Params, config.Globals, nil)
		if err != nil {
			return err
		}
		c.projectionTheoryReady = true
	}
	return nil
}
