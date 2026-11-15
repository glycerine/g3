package fp

// Source: src/muz/rel/rel_context.cpp rel_context::transform_rules (297-317)
// and setup_default_relation (519-523), for the doc relation owner.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

// transformUDocRules is rel_context::transform_rules on the query-scoped
// e.set: mk_coi_filter, mk_filter_rules, mk_simple_joins, mk_rule_inliner,
// mk_interp_tail_simplifier and mk_separate_negated_tails, ordered by their
// priorities. The doc similarity compressor runs afterwards on the result
// (queryUDocDatalog). setup_default_relation disables mk_unbound_compressor
// for the doc favourite. Predicates the transformation creates receive doc
// relations; the caller releases them with restrictUDocSeeds.
func (e *PropositionalEngine) transformUDocRules(owner *datalogUDocContext) (fresh []ast.ASTHandle, resultErr error) {
	if err := e.clearDatalogTransformCleanup(); err != nil {
		return nil, err
	}
	owners := &datalogTransformOwners{transformer: base.NewRuleTransformer()}
	transformer := owners.transformer
	conversionBase := len(e.datalogConversions)
	var coneSteps []transforms.ConeModelStep
	var inlineRules []transforms.InlineModelRule
	defer func() {
		e.datalogTransformCleanup = owners
		resultErr = errors.Join(resultErr, e.clearDatalogTransformCleanup())
		if resultErr != nil {
			cleanupErr := closeHornModelConversions(e.datalogConversions[conversionBase:])
			resultErr = errors.Join(resultErr, cleanupErr)
			if cleanupErr == nil {
				e.datalogConversions = e.datalogConversions[:conversionBase]
			}
		}
		cleanupErr := transforms.CloseConeModelSteps(coneSteps)
		resultErr = errors.Join(resultErr, cleanupErr)
		if cleanupErr != nil {
			for _, step := range coneSteps {
				e.datalogConversions = append(e.datalogConversions, hornModelConversion{kind: hornConvertCone, cone: step})
			}
		}
		cleanupErr = transforms.CloseInlineModelRules(inlineRules)
		resultErr = errors.Join(resultErr, cleanupErr)
		if cleanupErr != nil {
			e.datalogConversions = append(e.datalogConversions, hornModelConversion{kind: hornConvertInline, inline: inlineRules})
		}
	}()
	context, globals, err := owners.services(e)
	if err != nil {
		return nil, err
	}
	normalization, err := e.ruleNormalizationParams()
	if err != nil {
		return nil, err
	}
	simplifier, err := transforms.NewInterpTailSimplifierPluginShared(e.manager, e.rules, e.contextRewriter(), context, globals, 0)
	if err != nil {
		return nil, err
	}
	simplifier.SetNormalizationParams(normalization)
	inliner, err := e.datalogInlinerWithFacts(context, globals, owner.hasFacts)
	if err != nil {
		return nil, errors.Join(err, simplifier.Close())
	}
	cone := transforms.NewConePluginWithRelations(e.relations, 0)
	// rel_context::has_facts sees the doc relations, which the Go UDoc
	// owner keeps outside the relation manager.
	cone.SetFactProbe(owner.hasFacts)
	cone.SetModelConverter(e.contextModelConverter)
	negative := transforms.NewSeparateNegatedTailsPlugin(e.manager, e.rules, 0)
	filter := transforms.NewFilterRulesPlugin(e.manager, e.rules, 0)
	joins := transforms.NewSimpleJoinsPlugin(e.manager, e.rules, 0, e.relations)
	if err := transformer.Register(cone); err != nil {
		return nil, errors.Join(err, cone.Close(), simplifier.Close(), inliner.Close(), negative.Close(), filter.Close(), joins.Close())
	}
	if err := transformer.Register(simplifier); err != nil {
		return nil, errors.Join(err, simplifier.Close(), inliner.Close(), negative.Close(), filter.Close(), joins.Close())
	}
	if err := transformer.Register(inliner); err != nil {
		return nil, errors.Join(err, inliner.Close(), negative.Close(), filter.Close(), joins.Close())
	}
	if err := transformer.Register(negative); err != nil {
		return nil, errors.Join(err, negative.Close(), filter.Close(), joins.Close())
	}
	if err := transformer.Register(filter); err != nil {
		return nil, errors.Join(err, filter.Close(), joins.Close())
	}
	if err := transformer.Register(joins); err != nil {
		return nil, errors.Join(err, joins.Close())
	}
	var source *base.RuleSet
	if owner.products != nil {
		// The product executor plans a narrower program shape; keep the
		// untransformed rules for the fallback below.
		if source, err = base.NewRuleSet(e.rules); err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, source.Close()) }()
		if err = source.AddRules(e.set); err != nil {
			return nil, err
		}
	}
	modified, err := transformer.Apply(e.set)
	if err != nil {
		return nil, err
	}
	coneSteps, err = cone.TakeModelSteps()
	if err != nil {
		return nil, err
	}
	inlineRules, err = inliner.TakeModelRules()
	if err != nil {
		return nil, err
	}
	if !modified {
		return nil, nil
	}
	fresh = append(negative.FreshPredicates(), filter.FreshPredicates()...)
	fresh = append(fresh, joins.FreshPredicates()...)
	if owner.products != nil {
		// Rebuild the product plans over the transformed rules: plans name
		// rules by their index in the set they were planned for.
		var program *udocProductProgram
		if len(fresh) == 0 {
			program, err = e.planUDocProductsForSet(owner.factory, true, owner.products.program.sorts, e.set, nil, nil)
			if err != nil {
				return nil, err
			}
		}
		if program == nil {
			// KNOWN GAP (muz_fp_03-F2): the product executor has no plan for
			// the transformed program (its auxiliary predicates, or inlined
			// rule shapes). Evaluate the untransformed rules as before.
			if err = e.set.ReplaceRules(source); err != nil {
				return nil, err
			}
			valid, err := e.set.Finalize()
			if err != nil {
				return nil, err
			}
			if !valid {
				return nil, ErrUnstratifiedNegation
			}
			return nil, nil
		}
		if err = owner.syncProducts(e, program); err != nil {
			return nil, err
		}
	}
	for _, declaration := range fresh {
		state, x := e.predicateState(declaration)
		if x != nil {
			return nil, x
		}
		if e.predicates[declaration] == nil {
			e.predicateOrder = append(e.predicateOrder, declaration)
		}
		e.predicates[declaration] = state
		e.auxiliary[declaration] = true
	}
	// Source mk_fresh_head_predicate copies an explicit requested kind.
	for _, declaration := range filter.FreshPredicates() {
		if origin, found := filter.FreshPredicateOrigin(declaration); found {
			if err = e.inheritDatalogFiniteRepresentation(declaration, origin); err != nil {
				return fresh, err
			}
		}
	}
	for _, declaration := range joins.FreshPredicates() {
		if origin, found := joins.FreshPredicateOrigin(declaration); found {
			if err = e.inheritDatalogFiniteRepresentation(declaration, origin); err != nil {
				return fresh, err
			}
		}
	}
	for _, step := range coneSteps {
		e.datalogConversions = append(e.datalogConversions, hornModelConversion{kind: hornConvertCone, cone: step})
	}
	coneSteps = nil
	if len(inlineRules) != 0 {
		e.datalogConversions = append(e.datalogConversions, hornModelConversion{kind: hornConvertInline, inline: inlineRules})
		inlineRules = nil
	}
	e.revision++
	if owner.products != nil {
		return fresh, nil
	}
	// The relation manager creates the doc relations of the new predicates
	// when the compiled code first touches them; publish their owners.
	return fresh, owner.sync(e)
}

// hasFacts is rel_context::has_facts for the doc owner: the relation holds
// its pre-query facts and, as the source relation manager keeps the
// relations computed by a previous query, that query's result.
func (c *datalogUDocContext) hasFacts(declaration ast.ASTHandle) (bool, error) {
	if populated, err := c.hasSeedFacts(declaration); err != nil || populated {
		return populated, err
	}
	if c.products != nil {
		if value := c.products.result[declaration]; value != nil {
			empty, err := value.Empty()
			return !empty, err
		}
	}
	if table := c.finiteResult[declaration]; table != nil {
		length, err := table.Len()
		return length != 0, err
	}
	if present, found := c.nullaryResult[declaration]; found {
		return present, nil
	}
	if relation := c.result[declaration]; relation != nil {
		empty, err := relation.Empty()
		return !empty, err
	}
	return false, nil
}

// restrictUDocSeeds is scoped_query's restrict_predicates for the doc owner:
// the relations of predicates created by transform_rules leave with them.
func (c *datalogUDocContext) restrictUDocSeeds(fresh []ast.ASTHandle) error {
	if len(fresh) == 0 {
		return nil
	}
	removed := make(map[ast.ASTHandle]bool, len(fresh))
	for _, declaration := range fresh {
		relation := c.seeds[declaration]
		if relation == nil {
			continue
		}
		if err := relation.Close(); err != nil {
			return err
		}
		delete(c.seeds, declaration)
		removed[declaration] = true
	}
	order := c.order[:0]
	for _, declaration := range c.order {
		if !removed[declaration] {
			order = append(order, declaration)
		}
	}
	c.order = order
	return nil
}
