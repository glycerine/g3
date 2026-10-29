package api

// Source: src/api/api_context.cpp:142-151,358-363.

// Interrupt sets the context AST manager's resource cancellation flag. The
// manager propagates it to each registered child resource limit, matching the
// source context's interrupt operation.
func (c *SMTContext) Interrupt() error {
	manager, err := c.Manager()
	if err != nil {
		return err
	}
	manager.ResourceLimit().Cancel()
	return nil
}
