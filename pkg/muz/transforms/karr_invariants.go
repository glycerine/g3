package transforms

// Source: src/muz/transforms/dl_mk_karr_invariants.cpp/.h. The invariants
// come from rel_query on the inner datalog context (KarrInnerContext).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// KarrInnerContext is mk_karr_invariants' m_inner_ctx, a datalog context
// (engine datalog, the default_relation and karr parameters) over the
// outer context's AST manager; package fp provides it.
type KarrInnerContext interface {
	// Query is get_invariants up to rel_query: m_inner_ctx.reset(), the
	// outer context's predicates registered, replace_rules(src), close()
	// and rel_query(heads).
	Query(source *base.RuleSet, heads []ast.ASTHandle) error
	// Formula is rel_context::try_get_formula(p) after Query; a null
	// reference when p has no relation. The caller owns the result.
	Formula(declaration ast.ASTHandle) (ast.ASTRef, error)
	// Close is ~context with the plugin.
	Close() error
}

// ErrKarrInvariant reports an invariant from the inner context. Every
// loop-counter head has an Int column, so the inner datalog context's
// check_rules rejects every non-empty rule set: z3 4.7.1 never records an
// invariant (m_fun2inv stays empty) and its model converter stays empty.
var ErrKarrInvariant = errors.New("muz/transforms: unexpected Karr invariant from the inner datalog context")

type KarrInvariantsPlugin struct {
	manager   *ast.ASTManager
	rules     *base.RuleManager
	inner     KarrInnerContext
	priority  uint32
	cancelled bool
	closed    bool
}

func NewKarrInvariantsPlugin(manager *ast.ASTManager, rules *base.RuleManager, inner KarrInnerContext, priority uint32) *KarrInvariantsPlugin {
	if priority == 0 {
		priority = 36010
	}
	return &KarrInvariantsPlugin{manager: manager, rules: rules, inner: inner, priority: priority}
}

func (p *KarrInvariantsPlugin) Priority() uint32            { return p.priority }
func (p *KarrInvariantsPlugin) CanDestratifyNegation() bool { return false }
func (p *KarrInvariantsPlugin) Cancel()                     { p.cancelled = true }

func (p *KarrInvariantsPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil || p.inner == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	p.cancelled = false
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		negated, negationErr := rule.HasNegation()
		getErr = errors.Join(negationErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
		if negated {
			return nil, nil
		}
	}
	// mk_loop_counter lc(m_ctx); mk_backwards bwd(m_ctx); then the
	// scoped rule sets, destroyed in reverse order.
	loop := NewLoopCounterPlugin(p.manager, p.rules, 0)
	defer func() { resultErr = errors.Join(resultErr, loop.Close()) }()
	backwards := NewBackwardsPlugin(p.manager, p.rules, 0)
	defer func() { resultErr = errors.Join(resultErr, backwards.Close()) }()
	loopRules, err := loop.Transform(source)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, loopRules.Close()) }()
	if err = p.innerInvariants(loopRules); err != nil {
		return nil, err
	}
	canceled, err := p.manager.Canceled()
	if err != nil || canceled {
		return nil, err
	}
	reversed, err := backwards.Transform(loopRules)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, reversed.Close()) }()
	if err = p.innerInvariants(reversed); err != nil {
		return nil, err
	}
	annotated, err := p.annotateRules(loopRules)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, annotated.Close()) }()
	result, err := loop.Revert(annotated)
	if err != nil {
		return nil, err
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, errors.Join(err, result.Close())
	}
	return result, nil
}

// innerInvariants is get_invariants: rel_query on the inner context for the
// heads of src (begin_grouped_rules order), then try_get_formula for each
// head; a formula other than true would be conjoined into m_fun2inv.
func (p *KarrInvariantsPlugin) innerInvariants(source *base.RuleSet) error {
	heads, err := source.GroupedHeads()
	if err != nil {
		return err
	}
	if err = p.inner.Query(source, heads); err != nil {
		return err
	}
	for _, head := range heads {
		formula, err := p.inner.Formula(head)
		if err != nil {
			return err
		}
		if formula == (ast.ASTRef{}) {
			continue
		}
		value, err := p.manager.RefGet(formula)
		truth := err == nil && p.manager.IsTrue(value)
		err = errors.Join(err, p.manager.DelRef(&formula))
		if err != nil {
			return err
		}
		if !truth {
			return ErrKarrInvariant
		}
	}
	return nil
}

// annotateRules is update_rules with m_fun2inv empty: update_body adds
// every rule unchanged (new_rule = &r) with mk_rule_rewrite_proof(r, r),
// and the model converter gets no invariants.
func (p *KarrInvariantsPlugin) annotateRules(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, err := source.Rule(i)
		if err != nil {
			return nil, err
		}
		err = result.Add(rule)
		if err == nil {
			err = p.rules.MkRuleRewriteProof(rule, rule)
		}
		if err = errors.Join(err, rule.Close()); err != nil {
			return nil, err
		}
	}
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	keep = true
	return result, nil
}

func (p *KarrInvariantsPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	if p.inner == nil {
		return nil
	}
	return p.inner.Close()
}

// SourceTypeName is typeid(datalog::mk_karr_invariants).name() for the IF_VERBOSE
// transform lines of rule_transformer::operator().
func (p *KarrInvariantsPlugin) SourceTypeName() string {
	return base.DatalogPluginTypeName("mk_karr_invariants")
}
