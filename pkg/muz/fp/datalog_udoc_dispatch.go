package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) ensureUDocDatalog() (selected bool, resultErr error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	requested := false
	for _, state := range e.predicates {
		requested = requested || len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc"
	}
	if e.datalogRelationFavourite != "doc" && !requested {
		return false, nil
	}
	owner := e.udocDatalog
	if owner == nil {
		var err error
		owner, err = newDatalogUDocContext(e.manager)
		if err != nil {
			return false, err
		}
		defer func() {
			if owner != nil && e.udocDatalog != owner {
				resultErr = errors.Join(resultErr, owner.close())
			}
		}()
	}
	any, other, finiteGuards := false, false, false
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		supported, err := owner.factory.CanHandleSignature(state.sorts)
		if err != nil {
			return false, err
		}
		hasDoc := len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc"
		candidate := supported && !state.explicitFiniteRepresentation && state.external == nil && (hasDoc || e.datalogRelationFavourite == "doc" && len(state.requestedRepresentations) == 0 || e.auxiliary[d])
		any = any || candidate
		guard := udocFiniteNullary(state) && owner.seeds[d] == nil
		finiteGuards = finiteGuards || guard
		// The existing finite setter rejects an adopted UDoc owner. Keep that
		// boundary rather than allowing a second owner to mask its facts.
		changedOwner := udocFiniteNullary(state) && owner.seeds[d] != nil
		other = other || !candidate && !guard || changedOwner
	}
	if !any {
		return false, nil
	}
	if finiteGuards {
		allowed, err := e.udocNullaryGuardsOnly()
		if err != nil {
			return false, err
		}
		other = other || !allowed
	}
	if other {
		// The source generic compiler may mix the favourite doc plugin with a
		// fallback relation plugin. The specialized Go UDoc executor cannot,
		// so leave an initially mixed program to the ordinary finite executor.
		// Once UDoc has adopted facts, falling back would silently abandon its
		// seed owners, so preserve the explicit unsupported-fragment result.
		if e.udocDatalog != nil {
			return true, ErrUDocDatalogFragment
		}
		return false, nil
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return true, errors.Join(err, ErrUDocDatalogFragment)
	}
	if _, enabled, err := e.datalogCheckRelationName(); err != nil || enabled {
		return true, errors.Join(err, ErrUDocDatalogFragment)
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return true, errors.Join(err, ErrUDocDatalogFragment)
	}
	e.udocDatalog = owner
	return true, owner.sync(e)
}

func (e *PropositionalEngine) closeUDocDatalog() error {
	if e.udocDatalog == nil {
		return nil
	}
	if err := e.udocDatalog.close(); err != nil {
		return err
	}
	e.udocDatalog = nil
	return nil
}

func (e *PropositionalEngine) udocAnswer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if e.queryClone != nil && e.queryClone.udocDatalog != nil {
		return e.queryClone.udocDatalog.answer(declaration)
	}
	if e.udocDatalog == nil {
		return ast.ASTRef{}, ErrUDocDatalogFragment
	}
	return e.udocDatalog.answer(declaration)
}

// Query preparation and status are separate from positive-rule evaluation so
// errors never expose an earlier result through the current source snapshot.
func (e *PropositionalEngine) prepareUDocQuery() error {
	if err := e.udocDatalog.snapshot(); err != nil {
		return err
	}
	e.compiled = false
	e.datalogStats.Reset()
	return nil
}

func (e *PropositionalEngine) finishUDocQuery(declarations []ast.ASTHandle) (lbool.LBool, error) {
	return e.udocDatalog.status(declarations)
}

func (e *PropositionalEngine) queryUDocDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if err := e.udocDatalog.checkpoint(); err != nil {
		return lbool.Undef, err
	}
	if err := e.prepareUDocQuery(); err != nil {
		return lbool.Undef, err
	}
	if err := e.udocDatalog.saturate(e.set); err != nil {
		return lbool.Undef, err
	}
	return e.finishUDocQuery(declarations)
}
