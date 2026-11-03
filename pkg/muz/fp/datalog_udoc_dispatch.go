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
	checkerName, checkerEnabled, err := e.datalogCheckRelationName()
	if err != nil {
		return false, err
	}
	checkedDoc := checkerEnabled && checkerName == "doc"
	docFavourite := e.datalogRelationFavourite == "doc" || checkedDoc
	if e.udocDatalog != nil {
		// Source relation_manager keeps the existing concrete relation owner
		// when favourite/checker parameters change. Its migrated facts must
		// not fall back to the finite table whose contents it already adopted.
		docFavourite = true
		checkedDoc = e.udocDatalog.factory.FormulaChecked()
	}
	requested := false
	for _, state := range e.predicates {
		requested = requested || len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc"
	}
	if !docFavourite && !requested {
		return false, nil
	}
	owner := e.udocDatalog
	if owner == nil {
		var err error
		owner, err = e.newUDocDatalogOwner(checkedDoc)
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
	finiteInputs := make(map[ast.ASTHandle]*udocFiniteInput)
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		supported, err := owner.factory.CanHandleSignature(state.sorts)
		if err != nil {
			return false, err
		}
		hasDoc := len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "doc"
		if checkedDoc && hasDoc {
			return true, ErrUDocDatalogFragment
		}
		candidate := supported && !state.explicitFiniteRepresentation && state.external == nil && (hasDoc || docFavourite && len(state.requestedRepresentations) == 0 || e.auxiliary[d])
		any = any || candidate
		guard := udocFiniteNullary(state) && owner.seeds[d] == nil
		forward := !checkedDoc && supported && udocFiniteForwardingInput(state) && owner.seeds[d] == nil
		if forward {
			finiteInputs[d] = &udocFiniteInput{table: state.table, sorts: state.sorts, auxiliary: e.auxiliary[d]}
		}
		finiteGuards = finiteGuards || guard
		// The existing finite setter rejects an adopted UDoc owner. Keep that
		// boundary rather than allowing a second owner to mask its facts.
		changedOwner := udocFiniteNullary(state) && owner.seeds[d] != nil
		other = other || !candidate && !guard && !forward || changedOwner
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
	if len(finiteInputs) != 0 {
		allowed, err := e.udocFiniteForwardingOnly(finiteInputs)
		if err != nil {
			return false, err
		}
		other = other || !allowed
	}
	if other {
		if checkedDoc {
			// Preserve existing mixed finite execution until the generic
			// checked doc/table product compiler is integrated. Pure doc
			// programs take the concrete checked factory above; mixed Bool
			// signatures with only doc nullaries use the zero-column DOC
			// specialization in the finite scope.
			if e.udocDatalog == nil {
				return false, nil
			}
			return true, ErrUDocDatalogFragment
		}
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
	if checkerEnabled && !checkedDoc {
		return true, ErrUDocDatalogFragment
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
