package fp

// Source: muz/base/dl_context.cpp display_smt2 and fixedpoint-local print
// parameters and pending closed input formulas.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (e *PropositionalEngine) contextSMT2Config(queries []ast.ASTHandle, local *uparams.ParamsRef) (config base.ContextSMT2Config, resultErr error) {
	config = base.DefaultContextSMT2Config()
	config.Queries = queries
	if e.params != nil && e.globals != nil {
		parameters, err := e.fixedpointParamView()
		if err != nil {
			return config, err
		}
		defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
		if local != nil {
			parameters.local = local
		}
		config.FixedpointExtensions, err = parameters.Bool("print_fixedpoint_extensions", true)
		if err != nil {
			return config, err
		}
		config.LowLevel, err = parameters.Bool("print_low_level_smt2", false)
		if err != nil {
			return config, err
		}
		config.DeclareVariables, err = parameters.Bool("print_with_variable_declarations", true)
		if err != nil {
			return config, err
		}
	}
	for _, ref := range e.assertions {
		h, err := e.manager.RefGet(ref)
		if err != nil {
			return config, err
		}
		config.Background = append(config.Background, h)
	}
	if len(e.assertions) == 0 {
		h, err := e.manager.RefGet(e.background)
		if err != nil {
			return config, err
		}
		if !e.manager.IsTrue(h) {
			config.Background = append(config.Background, h)
		}
	}
	return config, nil
}

// WriteContextSMT2 emits source print profiles without changing query options
// or invalidating a published result. Local parameters, when given, override
// engine/module print settings for this invocation only.
func (e *PropositionalEngine) WriteContextSMT2(out io.Writer, queries []ast.ASTHandle, local *uparams.ParamsRef) (resultErr error) {
	if err := e.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	config, err := e.contextSMT2Config(queries, local)
	if err != nil {
		return err
	}
	count, err := e.set.Len()
	if err != nil {
		return err
	}
	omit := make([]bool, count)
	for _, raw := range e.rawRules {
		if !raw.pending {
			continue
		}
		h, err := e.manager.RefGet(raw.formula)
		if err != nil {
			return err
		}
		free, err := e.manager.HasFreeVars(h)
		if err != nil {
			return err
		}
		// Source normalizes open formulas even during display. Closed inputs
		// stay in the pending list, preserving binders and annotations.
		if free {
			continue
		}
		if raw.first < 0 || raw.end > count || raw.first > raw.end {
			return ErrPropositionalEngine
		}
		for i := raw.first; i < raw.end; i++ {
			omit[i] = true
		}
		config.PendingRules = append(config.PendingRules, h)
		config.PendingNames = append(config.PendingNames, raw.name)
	}
	if len(config.PendingRules) == 0 {
		return e.set.WriteContextSMT2(out, config)
	}
	snapshot, err := base.NewRuleSet(e.rules)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, snapshot.Close()) }()
	if err = snapshot.InheritPredicates(e.set); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if omit[i] {
			continue
		}
		rule, err := e.set.Rule(i)
		if err != nil {
			return err
		}
		err = errors.Join(snapshot.Add(rule), rule.Close())
		if err != nil {
			return err
		}
	}
	return snapshot.WriteContextSMT2(out, config)
}
