package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, nonlinear::compile.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

// ruleOwnerCleanupFailure keeps concrete retained rule owners reachable when
// native AST cleanup is refused. It implements memory.ConstructionFailure
// structurally without introducing a generic ownership adapter.
type ruleOwnerCleanupFailure struct {
	manager    *ast.ASTManager
	cause      error
	cleanupErr error
	reported   error
	reportText string
	rules      []*base.Rule
	child      memory.ConstructionFailure
}

type astRefCleanupFailure struct {
	manager    *ast.ASTManager
	refs       []ast.ASTRef
	cause      error
	cleanupErr error
	reported   error
	reportText string
	child      memory.ConstructionFailure
}

func (f *astRefCleanupFailure) Error() string { return f.reportText }
func (f *astRefCleanupFailure) Unwrap() error { return f.reported }
func (f *astRefCleanupFailure) Cleanup() error {
	if f.child != nil {
		if f.cleanupErr = f.child.Cleanup(); f.cleanupErr != nil {
			return f.cleanupErr
		}
		f.child = nil
	}
	f.cleanupErr = closeASTRefs(f.manager, f.refs)
	return f.cleanupErr
}
func (f *astRefCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.manager.NumeralService().RuntimeOwner()
}
func (f *astRefCleanupFailure) OriginalCause() error { return f.cause }
func (f *astRefCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func closeASTRefs(manager *ast.ASTManager, refs []ast.ASTRef) (result error) {
	for i := len(refs) - 1; i >= 0; i-- {
		if refs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := manager.DelRef(&refs[i]); err != nil {
			return errors.Join(result, err)
		}
	}
	return result
}

func disposeASTRefs(manager *ast.ASTManager, refs []ast.ASTRef, cause error) error {
	failure := &astRefCleanupFailure{manager: manager, refs: refs, cause: cause}
	if cause != nil {
		// These refs are constructed after the nested operation's roots. Drain
		// its retry receipt before touching the dependent outer frontier.
		errors.As(cause, &failure.child)
	}
	if cleanup := failure.Cleanup(); cleanup != nil {
		reported := cleanup
		if cause == nil {
			// Keep the refusal as the stable originating cause after a later
			// successful retry clears cleanupErr.
			failure.cause = cleanup
		} else {
			reported = errors.Join(cause, cleanup)
		}
		failure.reported = reported
		failure.reportText = reported.Error()
		return failure
	}
	return cause
}

func (f *ruleOwnerCleanupFailure) Error() string { return f.reportText }
func (f *ruleOwnerCleanupFailure) Unwrap() error { return f.reported }
func (f *ruleOwnerCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.manager.NumeralService().RuntimeOwner()
}
func (f *ruleOwnerCleanupFailure) OriginalCause() error { return f.cause }
func (f *ruleOwnerCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (f *ruleOwnerCleanupFailure) Cleanup() error {
	if f == nil {
		return nil
	}
	if f.child != nil {
		if f.cleanupErr = f.child.Cleanup(); f.cleanupErr != nil {
			return f.cleanupErr
		}
		f.child = nil
	}
	var pending []*base.Rule
	var cause error
	for i := len(f.rules) - 1; i >= 0; i-- {
		if err := f.rules[i].Close(); err != nil {
			pending = append(pending, f.rules[i])
			cause = errors.Join(cause, err)
		}
	}
	// The scan above is newest-to-oldest. Store pending owners in their
	// original order so every retry again closes newest-to-oldest.
	for left, right := 0, len(pending)-1; left < right; left, right = left+1, right-1 {
		pending[left], pending[right] = pending[right], pending[left]
	}
	f.rules = pending
	f.cleanupErr = cause
	if cause != nil {
		return cause
	}
	return nil
}

func closeRuleOwners(manager *ast.ASTManager, rules []*base.Rule) error {
	return disposeRuleOwners(manager, rules, nil)
}

func disposeRuleOwners(manager *ast.ASTManager, rules []*base.Rule, cause error) error {
	failure := &ruleOwnerCleanupFailure{manager: manager, rules: rules, cause: cause}
	if cause != nil {
		// Rule owners may depend on AST roots held by the nested operation.
		// Adopt that receipt before the first cleanup attempt so dependencies
		// always drain before retained rules.
		errors.As(cause, &failure.child)
	}
	if cleanup := failure.Cleanup(); cleanup != nil {
		reported := cleanup
		if cause == nil {
			failure.cause = cleanup
		} else {
			reported = errors.Join(cause, cleanup)
		}
		failure.cleanupErr = cleanup
		failure.reported = reported
		failure.reportText = reported.Error()
		return failure
	}
	return cause
}

func (c *Compiler) simplifyBoolean(and bool, args []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	m := c.manager
	var decl ast.ASTHandle
	var err error
	if and {
		decl, err = m.MkAndDecl()
	} else {
		decl, err = m.MkOrDecl()
	}
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = rewriter.SimplifyBooleanApplicationDefault(m, decl, args, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}

func (c *Compiler) applyVariables(decl ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	m := c.manager
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = disposeASTRefs(m, []ast.ASTRef{result}, resultErr)
			result = ast.ASTRef{}
		}
	}()
	count, err := m.GetArity(decl)
	if err != nil {
		return result, err
	}
	args := make([]ast.ASTHandle, count)
	var refs []ast.ASTRef
	defer func() {
		resultErr = disposeASTRefs(m, refs, resultErr)
	}()
	for i := range args {
		sort, err := m.GetDomain(decl, uint32(i))
		if err != nil {
			return result, err
		}
		args[i], err = m.MkVar(uint32(i), sort)
		if err != nil {
			return result, err
		}
		ref, err := m.NewRef(args[i])
		if err != nil {
			return result, err
		}
		refs = append(refs, ref)
	}
	value, err := m.MkApp(decl, args)
	if err != nil {
		return result, err
	}
	return m.NewRef(value)
}

// assignRef is expr_ref assignment: target takes value (the new term is
// retained before the old one is released).
func assignRef(m *ast.ASTManager, target ast.ASTRef, value ast.ASTHandle) error {
	return m.SetRef(target, value)
}

// assignFrom is target = <temporary expr_ref>: the assignment, then the
// temporary's destruction.
func assignFrom(m *ast.ASTManager, target ast.ASTRef, temporary *ast.ASTRef) error {
	value, err := m.RefGet(*temporary)
	if err == nil {
		err = m.SetRef(target, value)
	}
	return errors.Join(err, m.DelRef(temporary))
}

// releaseForward is ~expr_ref_vector: its elements are released first to
// last.
func releaseForward(m *ast.ASTManager, refs []ast.ASTRef, cause error) error {
	reversed := make([]ast.ASTRef, len(refs))
	for i, ref := range refs {
		reversed[len(refs)-1-i] = ref
	}
	return disposeASTRefs(m, reversed, cause)
}

// compileRule is one iteration of nonlinear::compile's rule loop. head and
// body are the loop's expr_refs (declared outside it): each assignment
// releases their previous terms. rule_i (ruleRef, owned) and the
// iteration's args and conjs vectors die at the iteration's end, conjs
// first. The result is the formula result.push_back(body) retains.
func (c *Compiler) compileRule(manager *base.RuleManager, predicate ast.ASTHandle, rule *base.Rule, ruleRef ast.ASTRef, level uint32, head, body ast.ASTRef) (result ast.ASTRef, resultErr error) {
	m := c.manager
	var args, conjs []ast.ASTRef
	defer func() {
		resultErr = releaseForward(m, conjs, resultErr)
		resultErr = releaseForward(m, args, resultErr)
		resultErr = disposeASTRefs(m, []ast.ASTRef{ruleRef}, resultErr)
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = disposeASTRefs(m, []ast.ASTRef{result}, resultErr)
			result = ast.ASTRef{}
		}
	}()
	push := func(vector *[]ast.ASTRef, value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		*vector = append(*vector, ref)
		return value, nil
	}
	ruleDecl, err := m.RefGet(ruleRef)
	if err != nil {
		return result, err
	}
	ruleHead, err := rule.Head()
	if err != nil {
		return result, err
	}
	variableSorts, err := rule.VariableSorts()
	if err != nil {
		return result, err
	}
	arity, err := m.GetArity(predicate)
	if err != nil {
		return result, err
	}
	argValues := make([]ast.ASTHandle, arity)
	seen := make(map[ast.ASTHandle]struct{})
	var conjValues []ast.ASTHandle
	var fresh uint32
	for i := range argValues {
		arg, err := m.GetArg(ruleHead, uint32(i))
		if err != nil {
			return result, err
		}
		kind, err := m.GetKind(arg)
		if err != nil {
			return result, err
		}
		_, duplicate := seen[arg]
		if kind == ast.ASTVar && !duplicate {
			seen[arg] = struct{}{}
			if argValues[i], err = push(&args, arg, nil); err != nil {
				return result, err
			}
			position, err := m.GetVarIndex(arg)
			if err != nil || int(position) >= len(variableSorts) {
				return result, errors.Join(ast.ErrASTDomain, err)
			}
			variableSorts[position] = ast.ASTHandle{}
		} else {
			sort, err := m.GetSort(arg)
			if err != nil {
				return result, err
			}
			variable, err := m.MkVar(uint32(len(variableSorts))+fresh, sort)
			if argValues[i], err = push(&args, variable, err); err != nil {
				return result, err
			}
			fresh++
			eq, err := m.MkEq(argValues[i], arg)
			eq, err = push(&conjs, eq, err)
			if err != nil {
				return result, err
			}
			conjValues = append(conjValues, eq)
		}
	}
	// head = m.mk_app(rule_i, args)
	application, err := m.MkApp(ruleDecl, argValues)
	if err == nil {
		err = assignRef(m, head, application)
	}
	if err != nil {
		return result, err
	}
	tailCount, err := rule.TailSize()
	if err != nil {
		return result, err
	}
	for i := 0; i < tailCount; i++ {
		item, _, err := rule.Tail(i)
		if err != nil {
			return result, err
		}
		if _, err = push(&conjs, item, nil); err != nil {
			return result, err
		}
		conjValues = append(conjValues, item)
	}
	// br.mk_and(conjs.size(), conjs.c_ptr(), body)
	temporary, err := c.simplifyBoolean(true, conjValues)
	if err != nil {
		return result, err
	}
	if err = assignFrom(m, body, &temporary); err != nil {
		return result, err
	}
	// replace_by_level_predicates(level, body): fml = tmp.
	value, err := m.RefGet(body)
	if err != nil {
		return result, err
	}
	if temporary, err = c.ReplacePredicates(manager, value, level); err != nil {
		return result, err
	}
	if err = assignFrom(m, body, &temporary); err != nil {
		return result, err
	}
	// body = skolemize_vars(r, args, rule_vars, body)
	if value, err = m.RefGet(body); err != nil {
		return result, err
	}
	if temporary, err = c.skolemizeVariables(predicate, argValues, variableSorts, value); err != nil {
		return result, err
	}
	if err = assignFrom(m, body, &temporary); err != nil {
		return result, err
	}
	// body = m.mk_implies(head, body)
	headValue, err := m.RefGet(head)
	if err != nil {
		return result, err
	}
	if value, err = m.RefGet(body); err != nil {
		return result, err
	}
	implication, err := m.MkImplies(headValue, value)
	if err == nil {
		err = assignRef(m, body, implication)
	}
	if err != nil {
		return result, err
	}
	// body = bind_vars(body, head)
	if temporary, err = c.BindVariables(implication, headValue); err != nil {
		return result, err
	}
	if err = assignFrom(m, body, &temporary); err != nil {
		return result, err
	}
	// result.push_back(body)
	return m.CopyRef(body)
}

// skolemizeVariables is skolemize_vars: mk_skolem_binding makes the
// binding vector (each body function's application), var_subst applies it,
// then the var_subst and the binding vector die, in that order.
func (c *Compiler) skolemizeVariables(predicate ast.ASTHandle, args []ast.ASTHandle, variableSorts []ast.ASTHandle, expression ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	m := c.manager
	var binding []ast.ASTRef
	defer func() {
		resultErr = releaseForward(m, binding, resultErr)
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = disposeASTRefs(m, []ast.ASTRef{result}, resultErr)
			result = ast.ASTRef{}
		}
	}()
	predicateName, err := m.GetASTName(predicate)
	if err != nil {
		return result, err
	}
	predicateText, err := m.Symbols.Display(predicateName)
	if err != nil {
		return result, err
	}
	argSorts := make([]ast.ASTHandle, len(args))
	for i, arg := range args {
		if argSorts[i], err = m.GetSort(arg); err != nil {
			return result, err
		}
	}
	bindings := make([]ast.ASTHandle, len(variableSorts))
	for i, sort := range variableSorts {
		if sort.IsNull() {
			continue
		}
		name, err := m.Symbols.Intern(fmt.Sprintf("%s@%d", predicateText, i))
		if err != nil {
			return result, err
		}
		decl, err := m.MkFuncDecl(name, argSorts, sort, ast.ASTFuncDeclInfo{})
		if err != nil {
			return result, err
		}
		application, err := m.MkApp(decl, args)
		if err != nil {
			return result, err
		}
		ref, err := m.NewRef(application)
		if err != nil {
			return result, err
		}
		binding = append(binding, ref)
		bindings[i] = application
	}
	sub, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		if sub != nil {
			err = errors.Join(err, sub.Close())
		}
		return result, err
	}
	if result, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return result, errors.Join(err, sub.Close())
	}
	err = sub.Substitute(expression, bindings, result)
	return result, errors.Join(err, sub.Close())
}

// Compile builds the source nonlinear implications for one unfolding level.
// Returned references are owned by the caller.
func (c *Compiler) Compile(rules *base.RuleSet, level uint32) (result []ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rules == nil {
		return nil, ast.ErrASTHandle
	}
	m := c.manager
	defer func() {
		if resultErr != nil {
			resultErr = disposeASTRefs(m, result, resultErr)
			result = nil
		}
	}()
	manager, err := rules.RuleManager()
	if err != nil {
		return result, err
	}
	// nonlinear::compile walks begin_grouped_rules()..end_grouped_rules(),
	// the rule set's decl2rules table order.
	predicates, err := rules.GroupedPredicates()
	if err != nil {
		return result, err
	}
	for _, predicate := range predicates {
		if err := c.compileGroup(rules, manager, predicate, level, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (c *Compiler) compileGroup(rules *base.RuleSet, manager *base.RuleManager, predicate ast.ASTHandle, level uint32, result *[]ast.ASTRef) (resultErr error) {
	m := c.manager
	group, err := rules.PredicateRules(predicate)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, closeRuleOwners(m, group))
	}()
	// level_pred, then expr_ref_vector rules and expr_ref body, head; they
	// die in reverse order: head, body, rules, level_pred.
	var levelRef, body, head ast.ASTRef
	var alternativeRefs []ast.ASTRef
	defer func() {
		resultErr = disposeASTRefs(m, []ast.ASTRef{head, body}, resultErr)
		resultErr = releaseForward(m, alternativeRefs, resultErr)
		resultErr = disposeASTRefs(m, []ast.ASTRef{levelRef}, resultErr)
	}()
	if levelRef, err = c.levelDeclaration(predicate, level, nil); err != nil {
		return err
	}
	levelDecl, err := m.RefGet(levelRef)
	if err != nil {
		return err
	}
	if body, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return err
	}
	if head, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return err
	}
	var alternatives []ast.ASTHandle
	for i, rule := range group {
		index := uint32(i)
		ruleRef, err := c.levelDeclaration(predicate, level, &index)
		if err != nil {
			return err
		}
		ruleDecl, err := m.RefGet(ruleRef)
		if err != nil {
			return disposeASTRefs(m, []ast.ASTRef{ruleRef}, err)
		}
		// rules.push_back(apply_vars(rule_i))
		application, err := c.applyVariables(ruleDecl)
		if err != nil {
			return disposeASTRefs(m, []ast.ASTRef{ruleRef}, err)
		}
		alternativeRefs = append(alternativeRefs, application)
		value, err := m.RefGet(application)
		if err != nil {
			return disposeASTRefs(m, []ast.ASTRef{ruleRef}, err)
		}
		alternatives = append(alternatives, value)
		formula, err := c.compileRule(manager, predicate, rule, ruleRef, level, head, body)
		if err != nil {
			return err
		}
		*result = append(*result, formula)
	}
	// br.mk_or(rules.size(), rules.c_ptr(), body)
	temporary, err := c.simplifyBoolean(false, alternatives)
	if err != nil {
		return err
	}
	if err = assignFrom(m, body, &temporary); err != nil {
		return err
	}
	// head = apply_vars(level_pred)
	if temporary, err = c.applyVariables(levelDecl); err != nil {
		return err
	}
	if err = assignFrom(m, head, &temporary); err != nil {
		return err
	}
	headValue, err := m.RefGet(head)
	if err != nil {
		return err
	}
	bodyValue, err := m.RefGet(body)
	if err != nil {
		return err
	}
	// body = m.mk_implies(head, body)
	implication, err := m.MkImplies(headValue, bodyValue)
	if err == nil {
		err = assignRef(m, body, implication)
	}
	if err != nil {
		return err
	}
	// body = bind_vars(body, head)
	if temporary, err = c.BindVariables(implication, headValue); err != nil {
		return err
	}
	if err = assignFrom(m, body, &temporary); err != nil {
		return err
	}
	formula, err := m.CopyRef(body)
	if err != nil {
		return err
	}
	*result = append(*result, formula)
	return nil
}
