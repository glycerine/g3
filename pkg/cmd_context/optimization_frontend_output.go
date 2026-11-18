package cmd_context

// Source: src/shell/opt_frontend.cpp display_results.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/opt"
)

// WriteOptimizationFrontendValues prints the optimization shell's objective
// bounds in the source shell format.
func (c *Context) WriteOptimizationFrontendValues(out io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if out == nil {
		return fmt.Errorf("optimization: nil output")
	}
	for index := range c.objectives {
		if err := c.writeOptimizationFrontendValue(out, uint32(index)); err != nil {
			return err
		}
	}
	return nil
}

// writeOptimizationFrontendValue writes one objective's bounds, which are
// temporaries.
func (c *Context) writeOptimizationFrontendValue(out io.Writer, index uint32) (resultErr error) {
	q := c.optQ()
	{
		lower, err := c.ObjectiveBound(index, true)
		defer func() { resultErr = errors.Join(resultErr, lower.Release(q)) }()
		if err != nil {
			return err
		}
		upper, err := c.ObjectiveBound(index, false)
		defer func() { resultErr = errors.Join(resultErr, upper.Release(q)) }()
		if err != nil {
			return err
		}
		cmp, err := lower.Value.Cmp(q, &upper.Value)
		if err != nil {
			return err
		}
		equal := cmp == 0
		prefix := "  ["
		if equal {
			prefix = "   "
		}
		if _, err = io.WriteString(out, prefix); err != nil {
			return err
		}
		if err = c.writeOptimizationFrontendBound(out, &lower); err != nil {
			return err
		}
		if !equal {
			if _, err = io.WriteString(out, ":"); err != nil {
				return err
			}
			if err = c.writeOptimizationFrontendBound(out, &upper); err != nil {
				return err
			}
			if _, err = io.WriteString(out, "]"); err != nil {
				return err
			}
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (c *Context) writeOptimizationFrontendBound(out io.Writer, bound *OptimizationBound) error {
	ref, err := opt.ArithmeticValueExpression(c.manager, &bound.Value, bound.Integer)
	if err != nil {
		return err
	}
	h, err := c.manager.RefGet(ref)
	if err == nil {
		err = c.WriteExpression(out, h)
	}
	return errors.Join(err, c.manager.DelRef(&ref))
}
