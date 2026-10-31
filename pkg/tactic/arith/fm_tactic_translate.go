package arith

// Source: src/tactic/arith/fm_tactic.cpp:264-283,1646-1648.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// The source borrows process services across managers. Parameter and numeral
// owners are explicit in Go; only the process cooperation service is shared.
func fmTargetServices(m *ast.ASTManager, source model.ModelEvaluatorConfig) (model.ModelEvaluatorConfig, error) {
	if m == nil {
		return model.ModelEvaluatorConfig{}, ast.ErrASTHandle
	}
	c, g, err := m.RewriterParameterServices()
	if err != nil {
		return model.ModelEvaluatorConfig{}, err
	}
	if c == nil {
		c, g = source.Context, source.Globals
	}
	if c == nil || g == nil || g.Context != c {
		return model.ModelEvaluatorConfig{}, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return model.ModelEvaluatorConfig{}, err
	}
	return model.ModelEvaluatorConfig{Context: c, Globals: g, Memory: m.NumeralService().RuntimeOwner(), Cooperation: source.Cooperation, Trace: m.BorrowSourceTraceTags()}, nil
}

func (t *FMTactic) Translate(m *ast.ASTManager) (result tactic.Tactic, err error) {
	defer func() {
		if err != nil {
			result = nil
		}
	}()
	if err = t.open(); err != nil {
		return nil, err
	}
	services, err := fmTargetServices(m, t.services)
	if err != nil {
		return nil, err
	}
	c, g := services.Context, services.Globals
	if c == t.context {
		return NewFMTactic(m, c, g, &t.params, services)
	}
	p, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, c.Del(&p)) }()
	// Only these concrete parameter types are consumed by the raw FM tactic.
	// Unused caller parameters do not require a universal cross-owner adapter.
	for _, field := range []struct {
		name     string
		fallback uint32
	}{
		{"max_memory", math.MaxUint32}, {"fm_limit", 5000000}, {"fm_cutoff1", 8}, {"fm_cutoff2", 256}, {"fm_extra", 0},
	} {
		name := field.name
		value, e := t.context.GetUIntName(&t.params, &name, field.fallback)
		if e != nil {
			return nil, e
		}
		if e = c.SetUIntName(&p, &name, value); e != nil {
			return nil, e
		}
	}
	for _, field := range []struct {
		name     string
		fallback bool
	}{{"fm_real_only", true}, {"fm_occ", false}} {
		name := field.name
		value, e := t.context.GetBoolName(&t.params, &name, field.fallback)
		if e != nil {
			return nil, e
		}
		if e = c.SetBoolName(&p, &name, value); e != nil {
			return nil, e
		}
	}
	return NewFMTactic(m, c, g, &p, services)
}
