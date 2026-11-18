package opt

// Source: src/opt/opt_context.cpp from_fmls/internalize and optsmt result flow.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/numeral"

	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type ObjectivePipelineValue struct {
	SourceKind FormulaObjectiveKind
	Soft       bool
	Exact      numeral.Rational
	Arithmetic smtcore.SMTArithmeticObjectiveValue
	ModelValue smtcore.SMTArithmeticObjectiveValue
	ModelValid bool
	Interval   bool
	OpenBounds bool
}

type ObjectivePipelineStatistics struct {
	Arithmetic bool
	MaxSMT     []MaxSMTStatistics
	Objectives uint32
	Hard       uint32
}

type ObjectivePipelineResult struct {
	Status     lbool.LBool
	Optimal    bool
	Values     []ObjectivePipelineValue
	Model      *model.ModelCore
	Statistics ObjectivePipelineStatistics
	// Q owns the Values' Exact rationals; Close releases them.
	Q *numeral.RationalManager
}

func (r *ObjectivePipelineResult) Close() error {
	if r == nil {
		return nil
	}
	var err error
	if r.Q != nil {
		for i := range r.Values {
			err = errors.Join(err, r.Q.Del(&r.Values[i].Exact))
			r.Values[i].Exact = numeral.NewRational()
		}
	}
	if r.Model == nil {
		return err
	}
	err = errors.Join(err, r.Model.DecRef())
	if err == nil {
		r.Model = nil
	}
	return err
}
