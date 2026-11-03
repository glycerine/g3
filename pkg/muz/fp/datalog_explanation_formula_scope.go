package fp

// Source: rel_context::init and relation_manager::mk_empty_relation select
// check_relation as the favourite only when no explicit relation kind applies.
// Explanation products keep their proof component outside this finite sieve.
import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/smtcore"
)

type datalogExplanationFormulaScope struct {
	engine   *PropositionalEngine
	eligible map[ast.ASTHandle]bool
	nested   bool
}

func (e *PropositionalEngine) beginDatalogExplanationFormulaScope() (*datalogExplanationFormulaScope, bool, error) {
	name, enabled, err := e.datalogCheckRelationName()
	if err != nil || !enabled {
		return nil, false, err
	}
	// The live formula checker currently owns a Hash TableRelation. Other
	// selected checker plugins must not silently fall back to that backing.
	if name != "hashtable" && name != "check" {
		return nil, false, fmt.Errorf("%w: explanation formula checker %s", rel.ErrCompilerUnsupported, name)
	}
	scope := &datalogExplanationFormulaScope{engine: e, eligible: make(map[ast.ASTHandle]bool), nested: name == "check"}
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state != nil {
			scope.eligible[declaration] = len(state.requestedRepresentations) == 0
		}
	}
	return scope, true, nil
}

func (s *datalogExplanationFormulaScope) transferFacts(transformer *rel.ExplanationTransformer, source, target *base.RuleSet) error {
	if s == nil || s.engine == nil {
		return rel.ErrCompilerUnsupported
	}
	e := s.engine
	return transformer.TransferRelationFactsFormulaChecked(source, target, e.relations, e.predicateOrder, s.eligible, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals})
}

func (s *datalogExplanationFormulaScope) installFreshProducts(predicates []ast.ASTHandle) error {
	if s == nil || s.engine == nil {
		return rel.ErrCompilerUnsupported
	}
	e := s.engine
	config := smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}
	for _, declaration := range predicates {
		_, found, err := e.relations.TryGetRelationExplanation(declaration)
		if err != nil {
			return err
		}
		if found {
			continue
		}
		product, eligible, err := rel.NewFormulaCheckedRelationExplanationForPredicate(e.relations, declaration, config)
		if err != nil {
			return fmt.Errorf("muz/fp: construct fresh explanation product %v: %w", declaration, err)
		}
		if !eligible {
			continue
		}
		if err = e.relations.StoreRelationExplanation(declaration, product); err != nil {
			return errors.Join(fmt.Errorf("muz/fp: store fresh explanation product %v: %w", declaration, err), product.Close())
		}
	}
	return nil
}

func (s *datalogExplanationFormulaScope) requireProduct(original ast.ASTHandle, product *rel.RelationExplanationProduct) error {
	if s == nil || s.engine == nil {
		return rel.ErrCompilerUnsupported
	}
	eligible, known := s.eligible[original]
	if !known {
		return fmt.Errorf("%w: predicate missing from explanation checker selection", rel.ErrRelationVerification)
	}
	if !eligible {
		return nil
	}
	return s.requireConcreteProduct(product)
}

func (s *datalogExplanationFormulaScope) requireConcreteProduct(product *rel.RelationExplanationProduct) error {
	if s == nil || s.engine == nil {
		return rel.ErrCompilerUnsupported
	}
	checked, err := product.FormulaChecked()
	if err != nil {
		return err
	}
	if !checked {
		return fmt.Errorf("%w: explanation finite sieve was not formula checked", rel.ErrRelationVerification)
	}
	return nil
}

// Close releases borrowed query-selection metadata. Products own their
// concrete checker and formula independently; no manager-global selection is
// changed by this scope.
func (s *datalogExplanationFormulaScope) Close() error {
	if s != nil {
		s.engine = nil
		s.eligible = nil
	}
	return nil
}
