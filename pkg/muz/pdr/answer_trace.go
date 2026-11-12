package pdr

// Source: model_search::get_trace in src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	astsubst "github.com/glycerine/g3/pkg/ast/substitution"
)

type traceObligation struct {
	node      *ModelNode
	predicate ast.ASTHandle
	ref       ast.ASTRef
}

func (c *Context) traceMaximumVariable(expressions ...[]ast.ASTHandle) (uint32, error) {
	used, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return 0, err
	}
	for _, group := range expressions {
		for _, expression := range group {
			if err = used.Process(expression); err != nil {
				return 0, err
			}
		}
	}
	return used.MaxFoundVarIndexPlusOne(), nil
}

// finish receives the final constraints while get_trace's locals are still
// alive: pm.mk_and(result) runs before they are destroyed.
func (c *Context) unifiedTraceConstraints(root *ModelNode, complete, includeRoot bool, finish func([]ast.ASTHandle) error) (result []ast.ASTRef, resultErr error) {
	phase := "update models"
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("pdr: trace %s: %w", phase, resultErr)
		}
	}()
	// get_trace: rule = n->get_rule() on the root, then update_models().
	if _, err := c.selectedTraceRule(root); err != nil {
		return nil, fmt.Errorf("pdr: select root trace rule: %w", err)
	}
	if err := c.updateTraceModels(root); err != nil {
		return nil, fmt.Errorf("pdr: update trace models: %w", err)
	}
	owned := make([]ast.ASTRef, 0)
	defer func() {
		if x := closeASTRefs(c.manager, owned); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: close trace temporaries: %w", x))
		}
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeASTRefs(c.manager, result))
			result = nil
		}
	}()
	rootRule, err := c.selectedTraceRule(root)
	if err != nil {
		return nil, fmt.Errorf("pdr: select root trace rule: %w", err)
	}
	head, err := rootRule.rule.Head()
	if err != nil {
		return nil, err
	}
	headRef, err := c.manager.NewRef(head) // predicates.push_back(rule->get_head())
	if err != nil {
		return nil, err
	}
	work := []traceObligation{{node: root, predicate: head, ref: headRef}}
	var anchor *PredicateTransformer
	if !complete {
		anchor, err = traceRecursiveAnchor(root)
		if err != nil {
			return nil, err
		}
	}
	theory, err := rewriter.NewTheoryRewriter(c.manager, c.owner.contexts.config.Params, c.owner.contexts.config.Globals, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if x := theory.Close(); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: close trace rewriter: %w", x))
		}
	}()
	unifier, err := astsubst.NewUnifier(c.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if x := unifier.Close(); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: close trace unifier: %w", x))
		}
	}()
	// get_trace's locals: expr_ref tmp, the substitution, and the
	// instantiated rule (rule_ref rule) outlive each iteration; the
	// constraint and predicate vectors own their entries.
	tmp, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	owned = append(owned, tmp)
	substitution, err := astsubst.New(c.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if x := substitution.Close(); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: close trace substitution: %w", x))
		}
	}()
	var instantiated []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(c.manager, instantiated)) }()
	var constraintRefs []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(c.manager, constraintRefs)) }()
	defer func() {
		for i := range work {
			if work[i].ref != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, c.manager.DelRef(&work[i].ref))
			}
		}
	}()
	// tmp = result; vec[i] = tmp.
	setTmp := func(reference ast.ASTRef) (ast.ASTHandle, error) {
		handle, y := c.manager.RefGet(reference)
		if y == nil {
			y = c.manager.SetRef(tmp, handle)
		}
		return handle, errors.Join(y, c.manager.DelRef(&reference))
	}
	instantiate := func(node *ModelNode, rule *normalizedRule, expression ast.ASTHandle) (ast.ASTHandle, error) {
		reference, y := c.instantiateTraceExpression(node, rule, expression)
		if y != nil {
			return ast.ASTHandle{}, y
		}
		instantiated = append(instantiated, reference)
		return c.manager.RefGet(reference)
	}
	iteration := 0
	for len(work) != 0 {
		phase = fmt.Sprintf("select rule %d", iteration)
		current := work[len(work)-1]
		rule, x := c.selectedTraceRule(current.node)
		if x != nil {
			return nil, fmt.Errorf("pdr: select trace rule: %w", x)
		}
		rawHead, x := rule.rule.Head()
		if x != nil {
			return nil, x
		}
		phase = fmt.Sprintf("instantiate rule %d", iteration)
		// r1 = r0 in mk_instantiate releases the previous instantiation.
		if x = closeASTRefs(c.manager, instantiated); x != nil {
			return nil, x
		}
		instantiated = instantiated[:0]
		instantiatedHead, x := instantiate(current.node, rule, rawHead)
		if x != nil {
			return nil, fmt.Errorf("pdr: instantiate trace head: %w", x)
		}
		tailSize, x := rule.rule.TailSize()
		if x != nil {
			return nil, x
		}
		uninterpreted, x := rule.rule.UninterpretedTailSize()
		if x != nil {
			return nil, x
		}
		tails := make([]ast.ASTHandle, tailSize)
		for index := 0; index < tailSize; index++ {
			tail, _, y := rule.rule.Tail(index)
			if y != nil {
				return nil, y
			}
			tails[index], y = instantiate(current.node, rule, tail)
			if y != nil {
				return nil, fmt.Errorf("pdr: instantiate trace tail %d: %w", index, y)
			}
		}
		// mk_instantiate's rule_manager::substitute rebuilds the rule with
		// rule_manager::mk, which fills the interpreted tails from the end:
		// the instantiated rule lists them in reverse.
		if len(rule.instances) != 0 {
			for i, j := uninterpreted, tailSize-1; i < j; i, j = i+1, j-1 {
				tails[i], tails[j] = tails[j], tails[i]
			}
		}
		constraints := make([]ast.ASTHandle, len(constraintRefs))
		for i := range constraintRefs {
			if constraints[i], x = c.manager.RefGet(constraintRefs[i]); x != nil {
				return nil, x
			}
		}
		pending := make([]ast.ASTHandle, len(work))
		for i := range work {
			pending[i] = work[i].predicate
		}
		phase = fmt.Sprintf("measure variables %d", iteration)
		maximum, x := c.traceMaximumVariable(constraints, pending, []ast.ASTHandle{instantiatedHead}, tails)
		if x != nil {
			return nil, x
		}
		// subst.reset(); subst.reserve(2, max_var+1)
		if x = substitution.Reset(); x != nil {
			return nil, x
		}
		if x = substitution.Reserve(2, maximum); x != nil {
			return nil, x
		}
		phase = fmt.Sprintf("unify predicates %d", iteration)
		unified, x := unifier.UnifyPair(current.predicate, instantiatedHead, substitution, true)
		if x != nil || !unified {
			if x != nil {
				return nil, x
			}
			return nil, fmt.Errorf("pdr: trace predicate does not unify: %w", ast.ErrASTDomain)
		}
		deltas := []uint32{0, maximum}
		apply := func(expression ast.ASTHandle, offset uint32, rewrite bool) (ast.ASTHandle, error) {
			applied, y := c.manager.NewRef(ast.ASTHandle{})
			if y != nil {
				return ast.ASTHandle{}, y
			}
			if y = substitution.Apply(2, deltas, astsubst.NewExprOffset(expression, offset), applied); y != nil {
				return ast.ASTHandle{}, errors.Join(y, c.manager.DelRef(&applied))
			}
			handle, y := setTmp(applied)
			if y != nil || !rewrite {
				return handle, y
			}
			// dctx.get_rewriter()(tmp)
			rewritten, y := c.manager.NewRef(ast.ASTHandle{})
			if y != nil {
				return ast.ASTHandle{}, y
			}
			if y = theory.Rewrite(handle, rewritten); y != nil {
				return ast.ASTHandle{}, errors.Join(y, c.manager.DelRef(&rewritten))
			}
			return setTmp(rewritten)
		}
		phase = "apply unifier"
		for i := range constraintRefs {
			value, y := apply(constraints[i], 0, true)
			if y == nil {
				y = c.manager.SetRef(constraintRefs[i], value)
			}
			if y != nil {
				return nil, y
			}
		}
		// predicates holds the pending obligations with the current one last.
		for i := range work {
			value, y := apply(work[i].predicate, 0, false)
			if y == nil {
				y = c.manager.SetRef(work[i].ref, value)
			}
			if y != nil {
				return nil, y
			}
			work[i].predicate = value
		}
		predicate := work[len(work)-1].predicate
		// get_trace's first flag: the root head (query!N) is never a constraint.
		if complete && (iteration != 0 || includeRoot) || iteration != 0 && (anchor == nil || current.node.transformer == anchor) {
			reference, y := c.manager.NewRef(predicate)
			if y != nil {
				return nil, y
			}
			constraintRefs = append(constraintRefs, reference)
		}
		// predicates.pop_back()
		popped := work[len(work)-1]
		work = work[:len(work)-1]
		if x = c.manager.DelRef(&popped.ref); x != nil {
			return nil, x
		}
		// The interpreted tails become constraints first; the uninterpreted
		// tails are instantiated as predicates only for a node with children.
		for i := uninterpreted; i < len(tails); i++ {
			value, y := apply(tails[i], 1, false)
			if y != nil {
				return nil, y
			}
			reference, y := c.manager.NewRef(value)
			if y != nil {
				return nil, y
			}
			constraintRefs = append(constraintRefs, reference)
		}
		children, x := current.node.Children()
		if x != nil {
			return nil, x
		}
		if len(children) == 0 {
			continue
		}
		if len(children) != uninterpreted {
			return nil, fmt.Errorf("pdr: trace has %d children for %d uninterpreted tails: %w", len(children), uninterpreted, ast.ErrASTDomain)
		}
		// get_trace appends children in rule-tail order and pops the last
		// child first. Keep its LIFO traversal and matching predicate pairing.
		for i := 0; i < uninterpreted; i++ {
			value, y := apply(tails[i], 1, false)
			if y != nil {
				return nil, y
			}
			reference, y := c.manager.NewRef(value)
			if y != nil {
				return nil, y
			}
			work = append(work, traceObligation{node: children[i], predicate: value, ref: reference})
		}
		iteration++
	}
	constraints := make([]ast.ASTHandle, len(constraintRefs))
	for i := range constraintRefs {
		if constraints[i], err = c.manager.RefGet(constraintRefs[i]); err != nil {
			return nil, err
		}
	}

	phase = "build ground replacement"
	replacement, err := c.manager.NewExprSubstitutionPolicy(false, false)
	if err != nil {
		return nil, err
	}
	defer func() {
		if x := c.manager.DelExprSubstitution(&replacement); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: delete trace replacement: %w", x))
		}
	}()
	for _, constraint := range constraints {
		if kind, x := c.manager.GetKind(constraint); x != nil {
			return nil, x
		} else if kind != ast.ASTApp {
			continue
		}
		family, x := c.manager.GetFamilyID(constraint)
		if x != nil {
			return nil, x
		}
		operation, x := c.manager.GetDeclKind(constraint)
		if x != nil {
			return nil, x
		}
		if family != c.manager.BasicFamilyID() || operation != ast.ASTOpEq {
			continue
		}
		left, x := c.manager.GetArg(constraint, 0)
		if x != nil {
			return nil, x
		}
		right, x := c.manager.GetArg(constraint, 1)
		if x != nil {
			return nil, x
		}
		leftKind, x := c.manager.GetKind(left)
		if x != nil {
			return nil, x
		}
		rightGround, x := c.manager.IsGround(right)
		if x != nil {
			return nil, x
		}
		if leftKind == ast.ASTVar && rightGround {
			if x = c.manager.ExprSubstitutionInsert(replacement, left, right, ast.ASTHandle{}, ast.ASTDependency{}); x != nil {
				return nil, x
			}
			continue
		}
		rightKind, x := c.manager.GetKind(right)
		if x != nil {
			return nil, x
		}
		leftGround, x := c.manager.IsGround(left)
		if x != nil {
			return nil, x
		}
		if rightKind == ast.ASTVar && leftGround {
			if x = c.manager.ExprSubstitutionInsert(replacement, right, left, ast.ASTHandle{}, ast.ASTDependency{}); x != nil {
				return nil, x
			}
		}
	}
	phase = "create trace replacer"
	replacer, err := rewriter.NewDefaultExprReplacer(c.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if x := replacer.Close(); x != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("pdr: close trace replacer: %w", x))
		}
	}()
	if err = replacer.SetSubstitution(&replacement); err != nil {
		return nil, err
	}
	truth, err := c.manager.MkTrue()
	if err != nil {
		return nil, err
	}
	phase = "finalize constraints"
	for _, constraint := range constraints {
		replaced, x := c.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return nil, x
		}
		if x = replacer.Rewrite(constraint, replaced); x != nil {
			return nil, errors.Join(x, c.manager.DelRef(&replaced))
		}
		replacedHandle, x := c.manager.RefGet(replaced)
		if x != nil {
			return nil, errors.Join(x, c.manager.DelRef(&replaced))
		}
		rewritten, x := c.manager.NewRef(ast.ASTHandle{})
		if x != nil {
			return nil, errors.Join(x, c.manager.DelRef(&replaced))
		}
		// tmp is rewritten in place: the replaced value dies here.
		x = theory.Rewrite(replacedHandle, rewritten)
		if x = errors.Join(x, c.manager.DelRef(&replaced)); x != nil {
			return nil, errors.Join(x, c.manager.DelRef(&rewritten))
		}
		handle, x := c.manager.RefGet(rewritten)
		if x != nil {
			return nil, errors.Join(x, c.manager.DelRef(&rewritten))
		}
		if handle == truth {
			if x = c.manager.DelRef(&rewritten); x != nil {
				return nil, x
			}
			continue
		}
		result = append(result, rewritten)
	}
	if finish != nil {
		handles := make([]ast.ASTHandle, len(result))
		for i := range result {
			if handles[i], err = c.manager.RefGet(result[i]); err != nil {
				return nil, err
			}
		}
		phase = "combine"
		if err = finish(handles); err != nil {
			return nil, err
		}
	}
	phase = "cleanup"
	return result, nil
}
