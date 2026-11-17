package api

// Source: src/api/api_datalog.cpp:590-606.

func (f *APIFixedpoint) Push() error {
	if err := f.open(); err != nil {
		return err
	}
	return fixedpointException(f.command.PushFixedpoint())
}

func (f *APIFixedpoint) Pop() error {
	if err := f.open(); err != nil {
		return err
	}
	return fixedpointException(f.command.PopFixedpoint())
}
