package fp

// Source: rel_context::transform_rules registers bit blasting at 22000 and
// another interpreted-tail simplifier at 21000. Predicate expansion comes from
// dl_mk_bit_blast.cpp. The relational answer uses the transformed Boolean
// coordinates (rel_context::query reads get_relation(rules.get_pred(q))), so
// publish records it from the derived relation. Go stores ground rules as
// table rows, so install imports them into the derived tables (the C++
// transforms those rules). The query's emptiness and answer are read from
// the derived relations before the query scope releases them; the original
// relations keep their own tuples, as in the source. The original relations'
// facts are copied into the derived ones (FORK FIX #120 for tables, #121 for
// every other relation kind).

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type datalogBitBlastPredicate struct {
	original, derived ast.ASTHandle
	widths            []uint32
}

// This query-local descriptor borrows plugin and table owners. The ordered
// transformer and outer Datalog query scope own every generated resource.
type datalogBitBlastQuery struct {
	engine  *PropositionalEngine
	planner *transforms.BitBlastPlugin
	columns []datalogBitBlastPredicate
}

func (q *datalogBitBlastQuery) register(transformer *base.RuleTransformer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) error {
	if q == nil || q.engine == nil {
		return nil
	}
	e := q.engine
	enabled := false
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return err
		}
		enabled, err = view.Bool("xform.bit_blast", false)
		err = errors.Join(err, view.Close())
		if err != nil {
			return err
		}
	}
	if !enabled {
		return nil
	}
	normalization, err := e.ruleNormalizationParams()
	if err != nil {
		return err
	}
	planner, err := transforms.NewBitBlastPlugin(e.manager, e.rules, e.contextRewriter(), context, globals, 22000)
	if err != nil {
		return err
	}
	planner.SetNormalizationParams(normalization)
	planner.SetRuleMaker(e.addFormulaWithProof)
	planner.SetBlastedFacts(q.addBitBlastedFacts)
	if err = transformer.Register(planner); err != nil {
		return errors.Join(err, planner.Close())
	}
	q.planner = planner
	simplifier, err := transforms.NewInterpTailSimplifierPluginShared(e.manager, e.rules, e.contextRewriter(), context, globals, 21000)
	if err != nil {
		return err
	}
	simplifier.SetNormalizationParams(normalization)
	if err = transformer.Register(simplifier); err != nil {
		return errors.Join(err, simplifier.Close())
	}
	return nil
}

// addBitBlastedFacts is rel_context::add_bit_blasted_facts(old, fresh),
// called by mk_bit_blast at the end of its transformation: the derived
// predicate gets its finite owner and the original relation's facts.
func (q *datalogBitBlastQuery) addBitBlastedFacts(original, derived ast.ASTHandle) error {
	e := q.engine
	source := e.predicates[original]
	if source == nil || source.table == nil {
		return fmt.Errorf("%w: bit-blast result requires an original finite table", ErrPropositionalHorn)
	}
	columns := datalogBitBlastPredicate{original: original, derived: derived, widths: make([]uint32, len(source.sorts))}
	var bits uint64
	for i, sort := range source.sorts {
		width, err := e.manager.BitVectorSize(sort)
		if err != nil {
			return err
		}
		if width == 0 || width > 63 {
			return fmt.Errorf("%w: bit-blast finite column width %d", ErrPropositionalHorn, width)
		}
		columns.widths[i] = width
		bits += uint64(width)
	}
	arity, err := e.manager.GetArity(derived)
	if err != nil {
		return err
	}
	if bits != uint64(arity) {
		return ast.ErrASTDomain
	}
	target, err := e.predicateState(derived)
	if err != nil {
		return err
	}
	if target.table == nil {
		return ErrPropositionalHorn
	}
	if e.predicates[derived] == nil {
		e.predicateOrder = append(e.predicateOrder, derived)
	}
	e.predicates[derived] = target
	e.auxiliary[derived] = true
	if err = e.inheritDatalogFiniteRepresentation(derived, original); err != nil {
		return err
	}
	q.columns = append(q.columns, columns)
	// FORK FIX #120/#121: the facts of the original relation, of any
	// kind, go to the derived predicate. try_get_relation skips a relation
	// the source has not created yet (a placeholder); a table's rows are
	// read; a relation that is not a table is asked contains_fact of every
	// tuple of its domains.
	var facts []rel.Fact
	placeholder := e.relations.IsPlaceholderRelation(original)
	if relation, ok := rel.AsRelation(source.table); ok && !placeholder {
		empty, err := relation.Empty()
		if err != nil {
			return err
		}
		if !empty {
			if facts, err = datalogBitBlastRelationTuples(e.manager, relation, source.sorts, columns.widths); err != nil {
				return err
			}
		}
	} else if !ok && !placeholder {
		if facts, err = source.table.Facts(); err != nil {
			return err
		}
	}
	for _, fact := range facts {
		encoded, err := encodeDatalogBitBlastFact(columns.widths, fact)
		if err != nil {
			return err
		}
		// rel_context::add_fact(new_pred, g): reset_saturated_marks,
		// then get_relation(pred) (made on first use) and add_fact.
		if err = e.relations.ResetSaturatedMarks(); err != nil {
			return err
		}
		if err = e.relations.MaterializeFiniteRelation(derived); err != nil {
			return err
		}
		if _, err = target.table.Add(encoded); err != nil {
			return err
		}
	}
	return nil
}

// install checks that every blasted predicate got its finite owner.
func (q *datalogBitBlastQuery) install() error {
	if q == nil || q.planner == nil {
		return nil
	}
	if len(q.columns) != len(q.planner.FreshPredicates()) {
		return fmt.Errorf("%w: bit-blasted predicate has no finite owner", ErrPropositionalHorn)
	}
	return nil
}

// publish prepares every inverse table before replacing original owners, so
// conversion failure cannot publish a partially decoded relation. Bit-blast
// conversion precedes the earlier inliner/COI model ledger: those converters
// read original-domain interpretations built from these completed tables.
//
// Only the queried predicates' relations are read (rel_context::query reads
// get_relation(rules.get_pred(q)) of its query predicates alone).
func (q *datalogBitBlastQuery) publish(queried []ast.ASTHandle) error {
	if q == nil || len(q.columns) == 0 {
		return nil
	}
	e := q.engine
	read := make(map[ast.ASTHandle]bool, len(queried))
	for _, declaration := range queried {
		read[declaration] = true
	}
	for _, columns := range q.columns {
		if !read[columns.original] {
			continue
		}
		derived := e.predicates[columns.derived]
		if derived == nil || derived.table == nil {
			return ErrPropositionalEngine
		}
		// rel_context::query: rel = get_relation(rules.get_pred(q)),
		// rel.empty() and is_precise(), then rel.to_formula.
		empty, precise := false, true
		if relation, ok := rel.AsRelation(derived.table); ok {
			var err error
			if empty, err = relation.Empty(); err != nil {
				return err
			}
			precise = relation.IsPrecise()
		} else {
			count, err := derived.table.Len()
			if err != nil {
				return err
			}
			empty = count == 0
		}
		if e.bitBlastEmpty == nil {
			e.bitBlastEmpty = make(map[ast.ASTHandle][2]bool)
		}
		e.bitBlastEmpty[columns.original] = [2]bool{empty, precise}
		answer, err := e.finiteRelationAnswer(derived)
		if err != nil {
			return err
		}
		if e.bitBlastAnswers == nil {
			e.bitBlastAnswers = make(map[ast.ASTHandle]ast.ASTRef)
		}
		if old, found := e.bitBlastAnswers[columns.original]; found {
			err = e.manager.DelRef(&old)
		} else {
			e.bitBlastAnswerOrder = append(e.bitBlastAnswerOrder, columns.original)
		}
		e.bitBlastAnswers[columns.original] = answer
		if err != nil {
			return err
		}
	}
	return nil
}

// releaseBitBlastAnswers drops the previous query's Boolean-relation answers.
func (e *PropositionalEngine) releaseBitBlastAnswers() error {
	var err error
	kept := e.bitBlastAnswerOrder[:0]
	for _, declaration := range e.bitBlastAnswerOrder {
		answer, found := e.bitBlastAnswers[declaration]
		if !found {
			continue
		}
		if cause := e.manager.DelRef(&answer); cause != nil {
			err = errors.Join(err, cause)
			kept = append(kept, declaration)
			continue
		}
		delete(e.bitBlastAnswers, declaration)
	}
	e.bitBlastAnswerOrder = kept
	e.bitBlastEmpty = nil
	return err
}
