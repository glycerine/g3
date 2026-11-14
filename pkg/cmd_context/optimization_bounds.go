package cmd_context

// Source: src/opt/opt_context.cpp:1299-1341 get_lower/get_upper.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

type OptimizationBound struct {
	Value   simplex.Infinitesimal
	Integer bool
}

func (c *Context) ObjectiveBound(index uint32, lower bool) (OptimizationBound, error) {
	if err := c.valid(); err != nil {
		return OptimizationBound{}, err
	}
	if uint64(index) >= uint64(len(c.objectives)) {
		return OptimizationBound{}, ast.ErrASTDomain
	}
	source := &c.objectives[index]
	if !source.valueReady && source.displayReady {
		// get_lower/get_upper are to_expr(get_lower_as_num(idx)) and
		// to_expr(get_upper_as_num(idx)), the bounds display_assignment
		// prints (AUDIT.md FUZZ_A-F1).
		value := source.displayUpper.Clone()
		if lower {
			value = source.displayLower.Clone()
		}
		return OptimizationBound{Value: value, Integer: toExprInteger(&value)}, nil
	}
	if !source.valueReady && !source.boundsReady {
		return OptimizationBound{}, fmt.Errorf("objective value is not available")
	}
	var value simplex.Infinitesimal
	if source.boundsReady {
		if lower {
			value.Main.Set(&source.boundLower)
		} else {
			value.Main.Set(&source.boundUpper)
		}
	} else if source.openBounds {
		if lower {
			value.Infinity.SetInt64(-1)
		} else {
			value.Infinity.SetInt64(1)
		}
	} else if source.soft || source.bitVector {
		value.Main.Set(&source.softValue)
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
			value.Infinity.SetInt64(sign)
		} else {
			value.Set(selected)
		}
	}
	return OptimizationBound{Value: value, Integer: value.Epsilon.Sign() == 0 && value.Main.IsInt()}, nil
}
