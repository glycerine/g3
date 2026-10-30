package arith

// Source: src/tactic/arith/fm_tactic.cpp:1445-1516.
func (e *FMEliminator) discard(cs []*FMConstraint) error {
	for _, c := range cs {
		if err := e.Store.Delete(c); err != nil {
			return err
		}
	}
	return nil
}

func (e *FMEliminator) Try(x uint32) (bool, error) {
	if e.Inconsistent || e.Variables.IsForbidden(x) {
		return false, nil
	}
	e.Variables.Cleanup(x)
	lowers := e.Variables.Index.Lower[x]
	uppers := e.Variables.Index.Upper[x]
	if len(lowers) == 0 || len(uppers) == 0 {
		e.markDead(x)
		return true, nil
	}
	if !e.Config.permits(len(lowers), len(uppers)) || !e.Variables.CanEliminate(x) {
		return false, nil
	}
	limit := len(lowers) + len(uppers) + int(e.Config.Extra)
	created := make([]*FMConstraint, 0, len(lowers)*len(uppers))
	for _, lower := range lowers {
		for _, upper := range uppers {
			if len(created) > limit {
				return false, e.discard(created)
			}
			resolution, err := FMResolve(e.Store, e.Variables, lower, upper, x)
			if err != nil {
				_ = e.discard(created)
				return false, err
			}
			if resolution.Inconsistent {
				if err := e.setInconsistent(resolution.Dependency); err != nil {
					_ = e.discard(created)
					return false, err
				}
				return false, e.discard(created)
			}
			if resolution.Constraint != nil {
				created = append(created, resolution.Constraint)
			}
		}
	}
	e.markDead(x)
	for _, c := range created {
		e.Variables.Index.Backward(c)
		e.Variables.Register(c)
	}
	return true, nil
}
