package base

import "fmt"

type datalogContextScope struct {
	rules      int
	background int
}

func (c *DatalogContext) Push() error {
	if err := c.open(); err != nil {
		return err
	}
	n, err := c.rules.Len()
	if err != nil {
		return err
	}
	c.scopes = append(c.scopes, datalogContextScope{rules: n, background: len(c.background)})
	return nil
}
func (c *DatalogContext) Pop(count uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	if int(count) > len(c.scopes) {
		return fmt.Errorf("muz/base: datalog scope underflow")
	}
	for ; count > 0; count-- {
		target := c.scopes[len(c.scopes)-1]
		c.scopes = c.scopes[:len(c.scopes)-1]
		for i := len(c.background) - 1; i >= target.background; i-- {
			if err := c.manager.DelRef(&c.background[i]); err != nil {
				return err
			}
		}
		c.background = c.background[:target.background]
		finalized, err := c.rules.IsFinalized()
		if err != nil {
			return err
		}
		if finalized {
			if err = c.rules.Reopen(); err != nil {
				return err
			}
		}
		for {
			n, err := c.rules.Len()
			if err != nil {
				return err
			}
			if n <= target.rules {
				break
			}
			rule, err := c.rules.Rule(n - 1)
			if err != nil {
				return err
			}
			removeErr := c.rules.Remove(rule)
			closeErr := rule.Close()
			if removeErr != nil {
				return removeErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	return nil
}
