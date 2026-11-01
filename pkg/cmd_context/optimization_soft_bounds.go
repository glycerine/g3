package cmd_context

import "github.com/glycerine/g3/pkg/opt"

// publishOptimizationSoftBounds copies available MaxSMT search bounds even
// when the search stops before finding an optimum. The result owns its values.
func (c *Context) publishOptimizationSoftBounds(result *opt.ArithmeticResult) {
	for i := range c.objectives {
		objective := &c.objectives[i]
		if !objective.soft {
			continue
		}
		objective.boundsReady = false
		if result == nil || i >= len(result.Values) || !result.Values[i].SoftBoundsValid {
			continue
		}
		objective.softLower.Set(&result.Values[i].SoftLower)
		objective.softUpper.Set(&result.Values[i].SoftUpper)
		objective.boundsReady = true
	}
}
