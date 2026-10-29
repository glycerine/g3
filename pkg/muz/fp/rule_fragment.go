package fp

// Source: src/muz/base/dl_context.h; src/muz/base/dl_context.cpp check_rules,
// src/muz/fp/dl_register_engine.h; src/muz/fp/dl_register_engine.cpp retained
// Datalog/PDR/Spacer engine selection.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/muz/base"
)

// validateRuleSetProperties applies the source check_rules ordering. Ordering
// is observable: the first failing property is the diagnostic returned to the
// fixedpoint caller.
func validateRuleSetProperties(rules *base.RuleManager, set *base.RuleSet, backend string) (resultErr error) {
	properties, err := base.NewRuleProperties(rules)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPropositionalHorn, err)
	}
	defer func() {
		if closeErr := properties.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("%w: close rule properties: %w", ErrPropositionalHorn, closeErr))
		}
	}()
	if err = properties.Collect(set); err != nil {
		return fmt.Errorf("%w: collect rule properties: %w", ErrPropositionalHorn, err)
	}
	switch backend {
	case "datalog":
		err = properties.CheckQuantifierFree()
		if err == nil {
			err = properties.CheckUninterpretedFree()
		}
		if err == nil {
			err = properties.CheckNestedFree()
		}
		if err == nil {
			err = properties.CheckInfiniteSorts()
		}
	case "pdr", "spacer":
		err = properties.CheckExistentialTail()
		if err == nil {
			err = properties.CheckNegativePredicates()
		}
		if err == nil {
			err = properties.CheckUninterpretedFree()
		}
	default:
		err = fmt.Errorf("unsupported fixedpoint engine %q", backend)
	}
	if err != nil {
		return fmt.Errorf("%w: validate %s rule fragment: %w", ErrPropositionalHorn, backend, err)
	}
	return nil
}
