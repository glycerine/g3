package cmd_context

// SelectDatalogFrontendEngine follows read_datalog's local engine=datalog
// selection before parsing, without replacing module or other local options.
func (c *Context) SelectDatalogFrontendEngine() error {
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	if err = engine.SelectEngine("datalog"); err != nil {
		return err
	}
	// read_datalog compiles and runs the rules itself rather than through
	// rel_context::saturate, so fixedpoint.print_aig does not apply.
	engine.SetDatalogFrontend(true)
	return nil
}
