package fp

// Source: src/muz/transforms/dl_mk_karr_invariants.cpp mk_karr_invariants
// (its m_inner_ctx member) and mk_karr_invariants::get_invariants.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// karrInnerContext is mk_karr_invariants' m_inner_ctx: a datalog::context
// over the outer context's AST manager whose parameters are
// default_relation=karr_relation, engine=datalog and karr=false. Only
// "engine" names a fixedpoint parameter ("default_relation" and "karr" are
// not datalog.default_relation and xform.karr), so the inner context keeps
// the module's relation and Karr settings, as in the source. The inner
// context shares the outer rule manager, which holds the rules it is given.
type karrInnerContext struct {
	outer *PropositionalEngine
	inner *PropositionalEngine
}

func (e *PropositionalEngine) newKarrInnerContext() *karrInnerContext {
	return &karrInnerContext{outer: e}
}

// reset is m_inner_ctx.reset(): the rules, predicates and engine go.
func (c *karrInnerContext) reset() error {
	if c.inner == nil {
		return nil
	}
	err := c.inner.Close()
	c.inner = nil
	return err
}

// Close is ~context when the plugin is deallocated.
func (c *karrInnerContext) Close() error {
	if c == nil {
		return nil
	}
	return c.reset()
}

func (c *karrInnerContext) open() (_ *PropositionalEngine, resultErr error) {
	o := c.outer
	inner, err := newPropositionalEngineWithServices(o.manager, o.params, o.globals, o.rulesShare)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, inner.Close())
		}
	}()
	inner.softTimeout = o.softTimeout
	if o.params == nil {
		return inner, nil
	}
	// mk_karr_invariants::mk_karr_invariants: m_inner_ctx.updt_params.
	params := o.params.EmptyValue()
	defer func() { resultErr = errors.Join(resultErr, o.params.Del(&params)) }()
	set := func(key, value string) error {
		name, err := o.manager.Symbols.Intern(value)
		if err != nil {
			return err
		}
		return o.params.SetSymbolName(&params, &key, name)
	}
	if err = set("default_relation", "karr_relation"); err != nil {
		return nil, err
	}
	if err = set("engine", "datalog"); err != nil {
		return nil, err
	}
	karr := "karr"
	if err = o.params.SetBoolName(&params, &karr, false); err != nil {
		return nil, err
	}
	if err = inner.SetQueryParameters(&params); err != nil {
		return nil, err
	}
	return inner, nil
}

// Query is get_invariants up to its rel_query: m_inner_ctx.reset(), the
// outer context's predicates registered, replace_rules(src), close() and
// rel_query(heads). The inner context is a datalog engine, so the query's
// check_rules rejects a rule whose head has an Int column; the loop
// counter gives every head one, so in practice only an empty rule set
// reaches saturation.
func (c *karrInnerContext) Query(source *base.RuleSet, heads []ast.ASTHandle) error {
	if err := c.reset(); err != nil {
		return err
	}
	inner, err := c.open()
	if err != nil {
		return err
	}
	c.inner = inner
	predicates, err := c.outer.rules.ContextPredicates()
	if err != nil {
		return err
	}
	for _, declaration := range predicates {
		if err = inner.RegisterPredicate(declaration); err != nil {
			return err
		}
	}
	for _, declaration := range heads {
		if err = inner.RegisterPredicate(declaration); err != nil {
			return err
		}
	}
	finalized, err := inner.set.IsFinalized()
	if err != nil {
		return err
	}
	if finalized {
		if err = inner.set.Reopen(); err != nil {
			return err
		}
	}
	if err = inner.set.ReplaceRules(source); err != nil {
		return err
	}
	inner.advanceRuleRevision()
	_, err = inner.QueryPredicates(heads)
	return err
}

// Formula is rel_context::try_get_formula(p) after Query: the formula of
// p's relation, or a null reference when it has none.
func (c *karrInnerContext) Formula(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if c.inner == nil {
		return ast.ASTRef{}, nil
	}
	state := c.inner.predicates[declaration]
	if state == nil || state.table == nil {
		return ast.ASTRef{}, nil
	}
	relation, ok := rel.AsRelation(state.table)
	if !ok {
		return ast.ASTRef{}, nil
	}
	return relation.ToFormula()
}
