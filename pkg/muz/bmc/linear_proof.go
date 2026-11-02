package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, linear::get_model proof assembly.
// Default-transformation proof conversion is a separate stage.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

// LinearProof constructs the unconverted hyper-resolution proof for a winning
// linear model, before transformation proof conversion. The caller owns the
// returned proof reference. The rule set must be the one used for the model.
func (c *Compiler) LinearProof(rules *base.RuleSet, query ast.ASTHandle, level uint32, md *model.ModelCore, services model.ModelEvaluatorConfig) (result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	rm, err := rules.RuleManager()
	if err != nil {
		return result, err
	}
	scope, err := ast.NewScopedProof(m)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, scope.Close()) }()
	if services.Context == nil || services.Globals == nil {
		return result, ast.ErrASTHandle
	}
	normalizer := transforms.NewRuleInlinerPluginWithServices(m, rm, nil, true, true, services.Context, services.Globals, 0)
	defer func() { resultErr = errors.Join(resultErr, normalizer.Close()) }()
	var previous *base.Rule
	defer func() {
		if previous != nil {
			resultErr = errors.Join(resultErr, previous.Close())
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	predicate := query
	for {
		canceled, err := m.Canceled()
		if err != nil {
			return result, err
		}
		if canceled {
			return result, fmt.Errorf("bmc: %s", m.ResourceLimit().CancelMessage())
		}
		rule, index, err := c.selectedLinearRule(rules, predicate, level, md)
		if err != nil {
			return result, err
		}
		nextRule, nextProof, err := c.linearProofStep(rm, rule, index, level, md, previous, result, normalizer)
		tails, tailErr := rule.UninterpretedTailSize()
		var nextPredicate ast.ASTHandle
		if tailErr == nil && tails > 0 {
			nextPredicate, tailErr = rule.TailDeclaration(0)
		}
		err = errors.Join(err, tailErr, rule.Close())
		if err != nil {
			if nextRule != nil {
				err = errors.Join(err, nextRule.Close())
			}
			if nextProof != (ast.ASTRef{}) {
				err = errors.Join(err, m.DelRef(&nextProof))
			}
			return result, err
		}
		if previous != nil {
			if err = previous.Close(); err != nil {
				return result, errors.Join(err, nextRule.Close(), m.DelRef(&nextProof))
			}
		}
		previous = nextRule
		if result != (ast.ASTRef{}) {
			if err = m.DelRef(&result); err != nil {
				return result, errors.Join(err, m.DelRef(&nextProof))
			}
		}
		result = nextProof
		if tails == 0 {
			return result, nil
		}
		if tails != 1 || level == 0 {
			return result, ast.ErrASTDomain
		}
		predicate = nextPredicate
		level--
	}
}

func (c *Compiler) linearProofStep(rm *base.RuleManager, rule *base.Rule, index, level uint32, md *model.ModelCore, previous *base.Rule, previousProof ast.ASTRef, normalizer *transforms.RuleInlinerPlugin) (next *base.Rule, proofRef ast.ASTRef, resultErr error) {
	bindings, err := c.LinearModelBindings(rule, index, level, md)
	if err != nil {
		return nil, ast.ASTRef{}, err
	}
	return c.linearProofStepWithBindings(rm, rule, bindings, previous, previousProof, normalizer, ast.ASTProofPosition{First: 1, Second: 0})
}

// linearProofStepWithBindings takes ownership of all supplied references.
func (c *Compiler) linearProofStepWithBindings(rm *base.RuleManager, rule *base.Rule, bindingRefs []ast.ASTRef, previous *base.Rule, previousProof ast.ASTRef, normalizer *transforms.RuleInlinerPlugin, position ast.ASTProofPosition) (next *base.Rule, proofRef ast.ASTRef, resultErr error) {
	m := c.manager
	refs := append([]ast.ASTRef(nil), bindingRefs...)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			if refs[i] != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
			}
		}
		if resultErr != nil {
			if next != nil {
				resultErr = errors.Join(resultErr, next.Close())
				next = nil
			}
			if proofRef != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, m.DelRef(&proofRef))
				proofRef = ast.ASTRef{}
			}
		}
	}()
	var err error
	bindings := make([]ast.ASTHandle, len(bindingRefs))
	for i, ref := range bindingRefs {
		if ref != (ast.ASTRef{}) {
			bindings[i], err = m.RefGet(ref)
			if err != nil {
				return nil, proofRef, err
			}
		}
	}
	instantiated, err := rm.SubstituteRule(rule, bindings)
	if err != nil {
		return nil, proofRef, err
	}
	defer func() { resultErr = errors.Join(resultErr, instantiated.Close()) }()
	proof, found, err := rule.Proof()
	if err != nil {
		return nil, proofRef, err
	}
	if !found {
		formula, err := rule.ToFormula()
		if err != nil {
			return nil, proofRef, err
		}
		refs = append(refs, formula)
		value, err := m.RefGet(formula)
		if err != nil {
			return nil, proofRef, err
		}
		proof, err = m.MkProofAsserted(value)
		if err != nil {
			return nil, proofRef, err
		}
	}
	pinned, err := m.NewRef(proof)
	if err != nil {
		return nil, proofRef, err
	}
	refs = append(refs, pinned)
	premises := []ast.ASTHandle{proof}
	var positions []ast.ASTProofPosition
	substitutions := [][]ast.ASTHandle{bindings}
	if previous == nil {
		next, err = instantiated.Retain()
	} else {
		unifier, e := transforms.NewRuleUnifier(m)
		if e != nil {
			return nil, proofRef, e
		}
		defer func() { resultErr = errors.Join(resultErr, unifier.Close()) }()
		ok, e := unifier.UnifyRules(previous, 0, instantiated)
		if e != nil || !ok {
			return nil, proofRef, errors.Join(ast.ErrASTDomain, e)
		}
		firstRefs, e := unifier.RuleSubstitution(previous, true)
		if e != nil {
			return nil, proofRef, e
		}
		refs = append(refs, firstRefs...)
		secondRefs, e := unifier.RuleSubstitution(instantiated, false)
		if e != nil {
			return nil, proofRef, e
		}
		refs = append(refs, secondRefs...)
		first := make([]ast.ASTHandle, len(firstRefs))
		second := make([]ast.ASTHandle, len(secondRefs))
		for i := range first {
			first[i], e = m.RefGet(firstRefs[i])
			if e != nil {
				return nil, proofRef, e
			}
		}
		for i := range second {
			second[i], e = m.RefGet(secondRefs[i])
			if e != nil {
				return nil, proofRef, e
			}
		}
		sub, e := rewriter.NewVarSubstitution(m, false)
		if e != nil {
			if sub != nil {
				e = errors.Join(e, sub.Close())
			}
			return nil, proofRef, e
		}
		defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
		for i, binding := range bindings {
			if binding.IsNull() {
				continue
			}
			ref, e := m.NewRef(ast.ASTHandle{})
			if e != nil {
				return nil, proofRef, e
			}
			refs = append(refs, ref)
			if e = sub.Substitute(binding, second, ref); e != nil {
				return nil, proofRef, e
			}
			bindings[i], e = m.RefGet(ref)
			if e != nil {
				return nil, proofRef, e
			}
		}
		var renaming []ast.ASTRef
		next, renaming, err = unifier.ResolveRawWithRenaming(rm, previous, 0, instantiated)
		refs = append(refs, renaming...)
		if err != nil {
			return nil, proofRef, err
		}
		// Rule construction compacts the resolvent variable bank. Compose
		// that compression with both premise substitutions before proving it.
		mapping := make([]ast.ASTHandle, len(renaming))
		for i, ref := range renaming {
			if ref != (ast.ASTRef{}) {
				mapping[i], err = m.RefGet(ref)
				if err != nil {
					return next, proofRef, err
				}
			}
		}
		if len(mapping) != 0 {
			for _, bank := range [][]ast.ASTHandle{first, bindings} {
				for i, term := range bank {
					if term.IsNull() {
						continue
					}
					ref, e := m.NewRef(ast.ASTHandle{})
					if e != nil {
						return next, proofRef, e
					}
					refs = append(refs, ref)
					if e = sub.Substitute(term, mapping, ref); e != nil {
						return next, proofRef, e
					}
					bank[i], e = m.RefGet(ref)
					if e != nil {
						return next, proofRef, e
					}
				}
			}
		}
		prior, e := m.RefGet(previousProof)
		if e != nil {
			return next, proofRef, e
		}
		premises = []ast.ASTHandle{prior, proof}
		positions = []ast.ASTProofPosition{position}
		substitutions = [][]ast.ASTHandle{first, bindings}
	}
	if err != nil {
		return nil, proofRef, err
	}
	if previous == nil && len(bindings) == 0 {
		proofRef, err = m.NewRef(proof)
		return next, proofRef, err
	}
	formula, err := next.ToFormula()
	if err != nil {
		return next, proofRef, err
	}
	refs = append(refs, formula)
	conclusion, err := m.RefGet(formula)
	if err != nil {
		return next, proofRef, err
	}
	resolvedProof, err := m.MkProofHyperResolve(premises, conclusion, positions, substitutions)
	if err != nil {
		return next, proofRef, err
	}
	if previous != nil {
		// Prove the raw resolvent first so interpreted-tail normalization can
		// preserve its proof through explicit rewrite/modus-ponens steps.
		if err = next.SetProof(resolvedProof); err != nil {
			return next, proofRef, err
		}
		normalized, e := normalizer.NormalizeResolvedRule(next)
		if e != nil {
			return next, proofRef, e
		}
		if e = next.Close(); e != nil {
			if normalized != nil {
				e = errors.Join(e, normalized.Close())
			}
			return nil, proofRef, e
		}
		next = normalized
		if next == nil {
			return nil, proofRef, ast.ErrASTDomain
		}
		var found bool
		resolvedProof, found, err = next.Proof()
		if err != nil || !found {
			return next, proofRef, errors.Join(err, ast.ErrASTDomain)
		}
	}
	proofRef, err = m.NewRef(resolvedProof)
	return next, proofRef, err
}
