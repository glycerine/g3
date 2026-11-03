package fp

// Source: src/muz/rel/rel_context.cpp similarity-compressor registration.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (e *PropositionalEngine) applyDatalogSimilarity(outputs []ast.ASTHandle) (resultErr error) {
	if e.backend != "datalog" {
		return nil
	}
	enabled, threshold := true, uint32(11)
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return err
		}
		enabled, err = view.Bool("datalog.similarity_compressor", true)
		if err == nil && enabled {
			threshold, err = view.UInt("datalog.similarity_compressor_threshold", 11)
		}
		err = errors.Join(err, view.Close())
		if err != nil {
			return err
		}
	}
	if !enabled {
		return nil
	}
	var requested []ast.ASTHandle
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state != nil && state.explicitFiniteRepresentation {
			requested = append(requested, declaration)
		}
	}
	plugin, err := rel.NewFiniteSimilarityCompressorForRulesWithRequestedKinds(e.rules, e.relations, threshold, outputs, requested)
	if err != nil {
		return err
	}
	transformer := base.NewRuleTransformer()
	if err = transformer.Register(plugin); err != nil {
		return errors.Join(err, plugin.Close(), transformer.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, transformer.Close()) }()
	modified, err := transformer.Apply(e.set)
	if err != nil || !modified {
		return err
	}
	for _, decl := range plugin.FreshPredicates() {
		state, err := e.predicateState(decl)
		if err != nil {
			return err
		}
		if e.predicates[decl] == nil {
			e.predicateOrder = append(e.predicateOrder, decl)
		}
		if origin, found := plugin.FreshPredicateOrigin(decl); found {
			if original := e.predicates[origin]; original != nil {
				state.explicitFiniteRepresentation = original.explicitFiniteRepresentation
				state.requestedRepresentations = append([]string(nil), original.requestedRepresentations...)
			}
		}
		e.predicates[decl] = state
		e.auxiliary[decl] = true
	}
	e.revision++
	return nil
}
