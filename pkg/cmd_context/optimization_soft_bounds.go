package cmd_context

import "github.com/glycerine/g3/pkg/opt"

// publishOptimizationBounds copies available search bounds even when the
// search stops before finding an optimum. The result owns its values.
func (c *Context) publishOptimizationBounds(result *opt.ArithmeticResult) {
	var err error
	for i := range c.objectives {
		objective := &c.objectives[i]
		objective.boundsReady = false
		if result == nil || i >= len(result.Values) {
			continue
		}
		value := &result.Values[i]
		if objective.soft && value.SoftBoundsValid {
			c.optSetBig(&objective.boundLower, &value.SoftLower, &err)
			c.optSetBig(&objective.boundUpper, &value.SoftUpper, &err)
		} else if !objective.soft && value.SourceBoundsValid {
			c.optSetBig(&objective.boundLower, &value.SourceLower, &err)
			c.optSetBig(&objective.boundUpper, &value.SourceUpper, &err)
		} else {
			continue
		}
		objective.boundsReady = err == nil
	}
}
