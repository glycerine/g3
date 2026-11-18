package cmd_context

// Source: src/opt/opt_context.cpp:1299-1341 get_lower/get_upper.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// OptimizationBound's Value is a fresh inf_eps of the AST manager's
// NumeralService().Manager owned by the bound (Release).
type OptimizationBound struct {
	Value   simplex.Infinitesimal
	Integer bool
}

// Release destroys the bound's Value.
func (b *OptimizationBound) Release(q *numeral.RationalManager) error {
	return b.Value.Del(q)
}

func (c *Context) ObjectiveBound(index uint32, lower bool) (OptimizationBound, error) {
	empty := OptimizationBound{Value: simplex.ZeroInfinitesimal()}
	if err := c.valid(); err != nil {
		return empty, err
	}
	if uint64(index) >= uint64(len(c.objectives)) {
		return empty, ast.ErrASTDomain
	}
	q := c.optQ()
	source := &c.objectives[index]
	if !source.valueReady && source.displayReady {
		// get_lower/get_upper are to_expr(get_lower_as_num(idx)) and
		// to_expr(get_upper_as_num(idx)), the bounds display_assignment
		// prints (AUDIT.md FUZZ_A-F1).
		selected := &source.displayUpper
		if lower {
			selected = &source.displayLower
		}
		value, err := selected.Clone(q)
		return c.optimizationBound(value, err)
	}
	if !source.valueReady && !source.boundsReady {
		return empty, fmt.Errorf("objective value is not available")
	}
	var value simplex.Infinitesimal
	var err error
	if source.boundsReady {
		bound := &source.boundUpper
		if lower {
			bound = &source.boundLower
		}
		value, err = c.optInfinitesimal(bound)
	} else if source.openBounds {
		if lower {
			value, err = c.optInfinite(-1)
		} else {
			value, err = c.optInfinite(1)
		}
	} else if source.soft || source.bitVector {
		value, err = c.optInfinitesimal(&source.softValue)
	} else {
		selected, unbounded := &source.value.Finite, source.value.Unbounded
		if source.interval && lower == source.maximize {
			selected, unbounded = &source.modelValue.Finite, source.modelValue.Unbounded
		}
		if unbounded {
			sign := int64(-1)
			if source.interval {
				if !lower {
					sign = 1
				}
			} else if source.maximize {
				sign = 1
			}
			value, err = c.optInfinite(sign)
		} else {
			value, err = selected.Clone(q)
		}
	}
	return c.optimizationBound(value, err)
}

// optimizationBound wraps a fresh value with to_expr's is_int, freeing it on
// failure.
func (c *Context) optimizationBound(value simplex.Infinitesimal, err error) (OptimizationBound, error) {
	bound := OptimizationBound{Value: value}
	if err == nil {
		bound.Integer, err = c.toExprInteger(&value)
	}
	if err != nil {
		return OptimizationBound{Value: simplex.ZeroInfinitesimal()}, errors.Join(err, value.Del(c.optQ()))
	}
	return bound, nil
}
