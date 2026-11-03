package fp

// Source: acyclic compiler components execute in dependency order. Missing
// target registers acquire the incoming product kind; original seed owners
// remain separate from each query's actual product result snapshot.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

func (p *udocProductContext) evaluate(plan *udocProductRulePlan, docs map[ast.ASTHandle]*rel.TypedUDocRelation, finite map[ast.ASTHandle]rel.FiniteTable) (resultErr error) {
	a := p.attempt()
	defer func() { resultErr = p.finish(a, resultErr) }()
	var working *rel.UDocTableProduct
	var err error
	if !plan.source.IsNull() {
		source := p.result[plan.source]
		if source == nil {
			return nil
		}
		working, err = source.Clone()
	} else {
		doc, table := docs[plan.doc], finite[plan.finite]
		if doc == nil || table == nil {
			return ErrUDocDatalogFragment
		}
		empty, emptyErr := doc.Empty()
		if emptyErr != nil || empty {
			return emptyErr
		}
		rows, rowsErr := table.Len()
		if rowsErr != nil || rows == 0 {
			return rowsErr
		}
		// The finite source declaration owns the sort signature; it is the
		// unchanged segment of the joined outer columns supplied by the plan.
		inputSorts := p.program.finiteSorts[plan.finite]
		if plan.docFirst {
			working, err = rel.JoinUDocFinite(p.manager, doc, inputSorts, table, plan.docColumns, plan.finiteColumns)
		} else {
			working, err = rel.JoinFiniteUDoc(p.manager, inputSorts, table, doc, plan.finiteColumns, plan.docColumns)
		}
	}
	if working != nil {
		a.values = append(a.values, working)
	}
	if err != nil {
		return err
	}
	for _, filter := range plan.filters {
		if err = working.FilterEqual(filter.column, filter.value); err != nil {
			return err
		}
	}
	for _, columns := range plan.identical {
		if err = working.FilterIdentical(columns); err != nil {
			return err
		}
	}
	empty, err := working.Empty()
	if err != nil || empty {
		return err
	}
	if len(plan.removed) != 0 {
		next, err := working.Project(plan.removed)
		if next != nil {
			a.values = append(a.values, next)
		}
		if err != nil {
			return err
		}
		working = next
	}
	for _, cycle := range plan.cycles {
		next, err := working.Rename(cycle)
		if next != nil {
			a.values = append(a.values, next)
		}
		if err != nil {
			return err
		}
		working = next
	}
	target := p.result[plan.head]
	if target == nil && finite[plan.head] != nil {
		count, cause := finite[plan.head].Len()
		if cause != nil {
			return cause
		}
		// Source cannot union a two-component product into a nonempty raw
		// table. Never silently upgrade an ordinary head with seed facts.
		if count != 0 {
			return ErrUDocDatalogFragment
		}
	}
	if p.seeds[plan.head] == nil {
		// Source relation kind survives invalidation caused by another
		// predicate's mutation, even before this head has explicit seed facts.
		seed, cause := working.EmptyLike()
		if seed != nil {
			p.seeds[plan.head] = seed
		}
		if cause != nil {
			return cause
		}
	}
	if target == nil {
		target, err = working.EmptyLike()
		if target != nil {
			p.result[plan.head] = target
		}
		if err != nil {
			return err
		}
	}
	delta, err := working.EmptyLike()
	if delta != nil {
		a.values = append(a.values, delta)
	}
	if err != nil {
		return err
	}
	// Keep the source componentwise delta, including its deliberately coarse
	// Empty result. Acyclic dependency order requires no fixed-point shortcut.
	return target.UnionFrom(working, delta)
}
