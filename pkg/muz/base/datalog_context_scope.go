package base

import "fmt"

func (c *DatalogContext) Push() error {
	if err := c.open(); err != nil {
		return err
	}
	n, err := c.rules.Len()
	if err != nil {
		return err
	}
	c.scopes = append(c.scopes, n)
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
			if n <= target {
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
