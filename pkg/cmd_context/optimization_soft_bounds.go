package cmd_context

import (
	"errors"

	"github.com/glycerine/g3/pkg/opt"
)

// publishOptimizationBounds copies available search bounds even when the
// search stops before finding an optimum. The result owns its values.
func (c *Context) publishOptimizationBounds(result *opt.ArithmeticResult) {
	q := c.optQ()
	var err error
	for i := range c.objectives {
		objective := &c.objectives[i]
		objective.boundsReady = false
		if result == nil || i >= len(result.Values) {
			continue
		}
		value := &result.Values[i]
		if objective.soft && value.SoftBoundsValid {
			err = errors.Join(err, q.Copy(&objective.boundLower, &value.SoftLower), q.Copy(&objective.boundUpper, &value.SoftUpper))
		} else if !objective.soft && value.SourceBoundsValid {
			err = errors.Join(err, q.Copy(&objective.boundLower, &value.SourceLower), q.Copy(&objective.boundUpper, &value.SourceUpper))
		} else {
			continue
		}
		objective.boundsReady = err == nil
	}
}
