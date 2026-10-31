package api

// Source: src/api/api_solver.cpp and generated api_commands.cpp, solver replay.

import (
	"errors"
	"reflect"
)

type replaySolver struct {
	object        apiObject
	owner         *replayContextOwner
	value         *APISimpleSolver
	kind          uint8
	logic         string
	params        *ParameterSet
	retiredParams []*ParameterSet
}

func (s *replaySolver) apiDestroy() error {
	if err := s.value.Close(); err != nil {
		return err
	}
	if s.params != nil {
		if err := s.params.Close(); err != nil {
			return err
		}
		s.params = nil
	}
	for len(s.retiredParams) != 0 {
		i := len(s.retiredParams) - 1
		if err := s.retiredParams[i].Close(); err != nil {
			return err
		}
		s.retiredParams = s.retiredParams[:i]
	}
	return nil
}

func RegisterZ3GeneratedSolverCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{408, "Z3_mk_solver", replayMkSolver}, {409, "Z3_mk_simple_solver", replayMkSimpleSolver}, {410, "Z3_mk_solver_for_logic", replayMkSolverForLogic},
		{414, "Z3_solver_get_param_descrs", replaySolverDescriptions}, {415, "Z3_solver_set_params", replaySolverSetParams}, {416, "Z3_solver_inc_ref", replaySolverIncRef}, {417, "Z3_solver_dec_ref", replaySolverDecRef},
		{418, "Z3_solver_push", replaySolverPush}, {419, "Z3_solver_pop", replaySolverPop}, {420, "Z3_solver_reset", replaySolverReset}, {421, "Z3_solver_get_num_scopes", replaySolverScopes},
		{422, "Z3_solver_assert", replaySolverAssert}, {423, "Z3_solver_assert_and_track", replaySolverAssertTracked},
		{426, "Z3_solver_from_string", replaySolverFromString}, {427, "Z3_solver_check", replaySolverCheck},
		{428, "Z3_solver_check_assumptions", replaySolverCheckAssumptions}, {434, "Z3_solver_get_reason_unknown", replaySolverReasonUnknown},
		{436, "Z3_solver_to_string", replaySolverString},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return nil
}
func replayMkSolver(r *Z3Replayer) error         { return replayCreateSolver(r, 0) }
func replayMkSimpleSolver(r *Z3Replayer) error   { return replayCreateSolver(r, 1) }
func replayMkSolverForLogic(r *Z3Replayer) error { return replayCreateSolver(r, 2) }
func replayCreateSolver(r *Z3Replayer, kind uint8) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if r.nextObjectID == ^uint64(0) {
		return ErrZ3ReplayState
	}
	var value *APISimpleSolver
	var logic string
	switch kind {
	case 0:
		value, err = owner.context.NewSolver()
	case 1:
		value, err = owner.context.NewSimpleSolver()
	case 2:
		name, e := r.value(1, replaySymbol)
		if e != nil {
			return e
		}
		if name.numericSymbol || name.nullSymbol {
			return ErrZ3ReplayReference
		}
		logic = name.s
		value, err = owner.context.NewSolverForLogic(logic)
	default:
		return ErrZ3ReplayState
	}
	if err != nil {
		return err
	}
	n := uint64(reflect.TypeOf(replaySolver{}).Size())
	if err = owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	s := &replaySolver{owner: owner, value: value, kind: kind, logic: logic}
	s.object = apiObject{context: owner.results, destructor: s, ownerBytes: n}
	s.object.self = &s.object
	id, err := owner.results.addObject(&s.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	s.object.id = id
	if err = owner.results.saveObject(&s.object); err != nil {
		return err
	}
	if r.solvers == nil {
		r.solvers = make(map[uint64]*replaySolver)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.solvers[replayID] = s
	owner.solverIDs = append(owner.solverIDs, replayID)
	return r.StoreResult(replayID)
}
func replayGetSolver(r *Z3Replayer) (*replaySolver, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	id, err := r.Object(1)
	if err != nil {
		return nil, err
	}
	s := r.solvers[id]
	if s == nil || s.owner != owner || s.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return s, nil
}
func replaySolverSetParams(r *Z3Replayer) error {
	s, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	id, err := r.Object(2)
	if err != nil {
		return err
	}
	p := r.parameterSets[id]
	if p == nil || p.owner != s.owner || p.object.open() != nil {
		return ErrZ3ReplayReference
	}
	return s.setParameters(p.value)
}
func replaySolverIncRef(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	return s.object.incRef()
}
func replaySolverDecRef(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	return s.object.decRef()
}
func replaySolverPush(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	return s.value.Push()
}
func replaySolverPop(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	n, e := r.Uint(2)
	if e != nil {
		return e
	}
	return s.value.Pop(n)
}
func replaySolverScopes(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	_, e = s.value.ScopeLevel()
	return e
}
func replaySolverAssert(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	a, e := r.astObject(s.owner, 2)
	if e != nil {
		return e
	}
	return s.value.Assert(a.handle)
}
func replaySolverAssertTracked(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	a, e := r.astObject(s.owner, 2)
	if e != nil {
		return e
	}
	b, e := r.astObject(s.owner, 3)
	if e != nil {
		return e
	}
	return s.value.AssertTracked(a.handle, b.handle)
}
func replaySolverFromString(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	text, e := r.String(2)
	if e != nil {
		return e
	}
	return s.value.LoadSMT2String(text)
}
func replaySolverCheck(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	_, e = s.value.Check()
	return e
}
func replaySolverCheckAssumptions(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	args, e := replayASTArray(r, s.owner, 2, 3)
	if e != nil {
		return e
	}
	_, e = s.value.Check(args...)
	return e
}
func replaySolverReasonUnknown(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	_, e = s.value.ReasonUnknown()
	return e
}
func replaySolverString(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	_, e = s.value.String()
	return e
}
