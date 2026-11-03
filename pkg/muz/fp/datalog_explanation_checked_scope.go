package fp

// Source: src/muz/rel/rel_context.cpp checked favourite table selection for
// explanation transform/compiler temporaries.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/muz/rel"
)

// datalogExplanationCheckedScope owns only the temporary factory installation.
// The relation manager owns every table created through the borrowed factory.
type datalogExplanationCheckedScope struct {
	engine    *PropositionalEngine
	primary   string
	checker   string
	installed bool
}

func (e *PropositionalEngine) beginDatalogExplanationCheckedScope() (_ *datalogExplanationCheckedScope, enabled bool, resultErr error) {
	checker, enabled, err := e.datalogDefaultTableCheck()
	if err != nil || !enabled {
		return nil, false, err
	}
	primary, err := e.datalogDefaultCheckedPrimary()
	if err != nil {
		return nil, false, err
	}
	factory, err := rel.NewFiniteCheckedFactory(primary, checker)
	if err != nil {
		return nil, false, err
	}
	scope := &datalogExplanationCheckedScope{engine: e, primary: primary, checker: checker}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, scope.Close())
		}
	}()
	if err = e.relations.SetCheckedFiniteFactory(factory); err != nil {
		return nil, false, err
	}
	scope.installed = true
	return scope, true, nil
}

type datalogExplanationCheckedProduct interface {
	CheckedConcreteKinds() (primary, checker string, checked bool, err error)
}

// requireProduct proves the explanation product's finite/index component was
// actually constructed by this scope before a checked query may succeed.
func (s *datalogExplanationCheckedScope) requireProduct(product datalogExplanationCheckedProduct) error {
	if s == nil || !s.installed {
		return nil
	}
	primary, checker, checked, err := product.CheckedConcreteKinds()
	if err != nil {
		return err
	}
	if !checked {
		return fmt.Errorf("%w: explanation product kinds %s/%s checked=false", rel.ErrCheckTable, primary, checker)
	}
	return nil
}

// Close is retryable and restores ordinary favourite-table construction.
func (s *datalogExplanationCheckedScope) Close() error {
	if s == nil || !s.installed {
		return nil
	}
	if s.engine == nil || s.engine.relations == nil {
		return rel.ErrTable
	}
	if err := s.engine.relations.SetCheckedFiniteFactory(nil); err != nil {
		return err
	}
	s.installed = false
	return nil
}
