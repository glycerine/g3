package duality

// profileScope restores the current timer on every search return, including
// cancellation and inference errors. The context lends its active profiler
// only for the duration of Engine.Solve.
type profileScope struct {
	profiler *Profiler
	name     string
}

func startProfile(context *Context, name string) (*profileScope, error) {
	if context == nil || context.activeProfiler == nil {
		return nil, nil
	}
	if err := context.activeProfiler.TimerStart(name); err != nil {
		return nil, err
	}
	return &profileScope{profiler: context.activeProfiler, name: name}, nil
}
func (s *profileScope) Close() error {
	if s == nil || s.profiler == nil {
		return nil
	}
	p := s.profiler
	s.profiler = nil
	return p.TimerStop(s.name)
}
