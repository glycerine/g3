package fp

// Source: src/muz/rel/rel_context.cpp rel_context::updt_params and
// src/muz/rel/check_relation.cpp checked fact insertion.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/util/symbol"
)

func (e *PropositionalEngine) datalogCheckRelationName() (name string, enabled bool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return "", false, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return "", false, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	selection, err := view.Symbol("datalog.check_relation", symbol.Symbol{})
	if err != nil {
		return "", false, err
	}
	if selection.Value == 0 {
		return "", false, nil
	}
	name, _, err = e.manager.Symbols.BareString(selection)
	if err != nil {
		return "", false, err
	}
	if name == "" || name == "null" {
		return name, false, nil
	}
	// Source table_relation_plugin::create_plugin_name registers these
	// relation names. Keep the existing bare Go convenience names too.
	switch name {
	case "tr_hashtable":
		name = "hashtable"
	case "tr_sparse":
		name = "sparse"
	case "tr_bitvector":
		name = "bitvector"
	case "tr_lazy_sparse":
		name = "lazy_sparse"
	}
	// rel_context::updt_params accepts any name: one no relation plugin
	// has leaves check_relation without a base.
	return name, true, nil
}

// datalogCheckRelationBase is context::check_relation() as
// rel_context::updt_params reads it: enabled unless null or "null"; name is
// the relation plugin name it looks up (tr_hashtable for the hashtable
// table's table_relation plugin; a name no relation plugin has gives
// check_relation no base).
func (e *PropositionalEngine) datalogCheckRelationBase() (name string, enabled bool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return "", false, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return "", false, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	selection, err := view.Symbol("datalog.check_relation", symbol.Symbol{})
	if err != nil || selection.Value == 0 {
		return "", false, err
	}
	name, _, err = e.manager.Symbols.BareString(selection)
	if err != nil || name == "" || name == "null" {
		return "", false, err
	}
	return name, true, nil
}

// datalogDefaultTableCheck returns the exact configured checker plugin. The
// selected primary is the predicate's current concrete storage, so an explicit
// SetPredicateRepresentation remains authoritative.
func (e *PropositionalEngine) datalogDefaultTableCheck() (checker string, enabled bool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return "", false, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return "", false, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	enabled, err = view.Bool("datalog.default_table_checked", false)
	if err != nil || !enabled {
		return "", false, err
	}
	readName := func(key, fallback string) (string, error) {
		defaultValue := symbol.Symbol{}
		if fallback != "" {
			var err error
			defaultValue, err = e.manager.Symbols.Intern(fallback)
			if err != nil {
				return "", err
			}
		}
		value, err := view.Symbol(key, defaultValue)
		if err != nil || value.Value == 0 {
			return "", err
		}
		name, _, err := e.manager.Symbols.BareString(value)
		return name, err
	}
	base, err := readName("datalog.default_table", "sparse")
	if err != nil {
		return "", false, err
	}
	checker, err = readName("datalog.default_table_checker", "null")
	if err != nil {
		return "", false, err
	}
	if checker == "" || checker == "null" {
		// Source registration leaves checking disabled when the named checker
		// plugin does not exist; null is its catalog default.
		return "", false, nil
	}
	normalize := func(name string) string {
		switch name {
		case "tr_hashtable":
			return "hashtable"
		case "tr_sparse":
			return "sparse"
		case "tr_bitvector":
			return "bitvector"
		case "tr_lazy_sparse":
			return "lazy_sparse"
		default:
			return name
		}
	}
	base, checker = normalize(base), normalize(checker)
	available := func(name string) bool {
		return name == "hashtable" || name == "sparse" || name == "bitvector" || name == "lazy_sparse"
	}
	if !available(base) || !available(checker) {
		return "", false, fmt.Errorf("muz/fp: checked default table pair %s/%s is not available", base, checker)
	}
	return checker, true, nil
}
