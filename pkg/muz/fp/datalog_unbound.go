package fp

// Source: rel_context.cpp transform_rules/setup_default_relation and scoped_query.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type datalogUnboundScope struct {
	engine     *PropositionalEngine
	set        *base.RuleSet
	predicates map[ast.ASTHandle]bool
	// keepRelations leaves the relation manager to an enclosing query
	// scope.
	keepRelations bool
}

// A refused native cleanup keeps its concrete owner reachable through the
// standard memory.ConstructionFailure cleanup contract.
type datalogUnboundCleanupFailure struct {
	cause error
	scope *datalogUnboundScope
}

func (f *datalogUnboundCleanupFailure) Error() string  { return f.cause.Error() }
func (f *datalogUnboundCleanupFailure) Unwrap() error  { return f.cause }
func (f *datalogUnboundCleanupFailure) Cleanup() error { return f.scope.Close() }

func (s *datalogUnboundScope) dispose() error {
	if err := s.Close(); err != nil {
		return &datalogUnboundCleanupFailure{cause: err, scope: s}
	}
	return nil
}

func (s *datalogUnboundScope) Close() error {
	if s == nil {
		return nil
	}
	if err := s.set.Close(); err != nil {
		return err
	}
	// scoped_query's destructor restricts the relations to the original
	// predicates; read_datalog runs no query, so its auxiliary relations stay.
	if !s.engine.datalogFrontend && !s.keepRelations {
		if err := s.engine.relations.RestrictPredicates(s.predicates); err != nil {
			return err
		}
	}
	for decl := range s.engine.predicates {
		if !s.predicates[decl] {
			delete(s.engine.predicates, decl)
			delete(s.engine.auxiliary, decl)
		}
	}
	order := s.engine.predicateOrder[:0]
	for _, decl := range s.engine.predicateOrder {
		if s.predicates[decl] {
			order = append(order, decl)
		}
	}
	s.engine.predicateOrder = order
	return nil
}

// datalogUnboundCompression reads the unbound compressor's switches:
// enabled is context::unbound_compressor (datalog.unbound_compressor,
// cleared by rel_context for doc relations), compress is
// context::compress_unbound (xform.compress_unbound), the plugin's own gate.
func (e *PropositionalEngine) datalogUnboundCompression() (enabled, compress bool, resultErr error) {
	enabled, compress = true, true
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return false, false, err
		}
		enabled, err = view.Bool("datalog.unbound_compressor", true)
		if err == nil && enabled {
			// The outer registration switch and the transform's own gate
			// are distinct source parameters, with the same local precedence.
			compress, err = view.Bool("xform.compress_unbound", true)
		}
		if err == nil && enabled {
			relation, cause := view.Symbol("datalog.default_relation", symbol.Symbol{})
			err = cause
			if err == nil && relation.Value != 0 {
				name, _, cause := e.manager.Symbols.BareString(relation)
				err = cause
				enabled = name != "doc"
			}
		}
		err = errors.Join(err, view.Close())
		if err != nil {
			return false, false, err
		}
	}
	if !enabled {
		return false, compress, nil
	}
	checkRelation, checking, err := e.datalogCheckRelationName()
	if err != nil {
		return false, false, err
	}
	if checking && checkRelation == "doc" {
		return false, compress, nil
	}
	return true, compress, nil
}

// datalogUnboundCompressor is rel_context::transform_rules'
// register_plugin(alloc(mk_unbound_compressor, m_context)) when
// m_context.unbound_compressor(); nil otherwise.
func (e *PropositionalEngine) datalogUnboundCompressor() (*transforms.UnboundCompressorPlugin, error) {
	enabled, compress, err := e.datalogUnboundCompression()
	if err != nil || !enabled {
		return nil, err
	}
	plugin := transforms.NewUnboundCompressorPlugin(e.manager, e.rules, compress, 0)
	// mk_unbound_compressor::operator(): collect_non_empty_predicates
	// (m_non_empty_rels only gates can_remove_orig_rule, whose
	// arg_indices.empty() conjunct is false inside its loop; the calls'
	// fast_empty are observable for external relations).
	plugin.SetNonEmptyCollector(func() error {
		_, err := e.relations.CollectNonEmptyPredicates()
		return err
	})
	return plugin, nil
}

// registerDatalogUnboundPredicates gives the compressed predicates of the
// transformed rules their predicate states (mk_fresh_head_predicate). It
// scans the resulting program rather than the plugin's one-value-per-source
// summary: different rules can compress different columns of one predicate.
func (e *PropositionalEngine) registerDatalogUnboundPredicates() error {
	count, err := e.set.Len()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		rule, cause := e.set.Rule(i)
		if cause != nil {
			return cause
		}
		head, cause := rule.HeadDeclaration()
		declarations := []ast.ASTHandle{head}
		tails := 0
		if cause == nil {
			tails, cause = rule.UninterpretedTailSize()
		}
		for j := 0; cause == nil && j < tails; j++ {
			var decl ast.ASTHandle
			decl, cause = rule.TailDeclaration(j)
			declarations = append(declarations, decl)
		}
		cause = errors.Join(cause, rule.Close())
		if cause != nil {
			return cause
		}
		for _, decl := range declarations {
			if e.predicates[decl] != nil {
				continue
			}
			state, cause := e.predicateState(decl)
			if cause != nil {
				return cause
			}
			e.predicates[decl] = state
			e.predicateOrder = append(e.predicateOrder, decl)
			e.auxiliary[decl] = true
		}
	}
	return nil
}
