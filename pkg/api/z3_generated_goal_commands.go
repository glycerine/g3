package api

// Source: generated api_commands.cpp and src/api/api_goal.cpp.

import (
	"errors"
	"reflect"
)

type replayGoal struct {
	object apiObject
	owner  *replayContextOwner
	value  *APIGoal
}

func (g *replayGoal) apiDestroy() error { return g.value.Close() }

func registerZ3GeneratedGoalCommands(r *Z3Replayer) error {
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{348, "Z3_mk_goal", replayMkGoal}, {349, "Z3_goal_inc_ref", replayGoalIncRef}, {350, "Z3_goal_dec_ref", replayGoalDecRef},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return nil
}
func replayStoreGoal(r *Z3Replayer, owner *replayContextOwner, value *APIGoal) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayGoal{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	g := &replayGoal{owner: owner, value: value}
	g.object = apiObject{context: owner.results, destructor: g, ownerBytes: n}
	g.object.self = &g.object
	id, err := owner.results.addObject(&g.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	g.object.id = id
	if err = owner.results.saveObject(&g.object); err != nil {
		return err
	}
	if r.goals == nil {
		r.goals = make(map[uint64]*replayGoal)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.goals[replayID] = g
	owner.goalIDs = append(owner.goalIDs, replayID)
	return r.StoreResult(replayID)
}
func replayGetGoal(r *Z3Replayer) (*replayGoal, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	id, err := r.Object(1)
	if err != nil {
		return nil, err
	}
	g := r.goals[id]
	if g == nil || g.owner != owner || g.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return g, nil
}
func replayMkGoal(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	models, err := r.Bool(1)
	if err != nil {
		return err
	}
	cores, err := r.Bool(2)
	if err != nil {
		return err
	}
	proofs, err := r.Bool(3)
	if err != nil {
		return err
	}
	value, err := owner.context.NewGoal(models, cores, proofs)
	if err != nil {
		return err
	}
	return replayStoreGoal(r, owner, value)
}
func replayGoalIncRef(r *Z3Replayer) error {
	if _, err := replayContext(r, 0); err != nil {
		return err
	}
	if id, err := r.Object(1); err != nil || id == 0 {
		return err
	}
	g, err := replayGetGoal(r)
	if err != nil {
		return err
	}
	return g.object.incRef()
}
func replayGoalDecRef(r *Z3Replayer) error {
	if _, err := replayContext(r, 0); err != nil {
		return err
	}
	if id, err := r.Object(1); err != nil || id == 0 {
		return err
	}
	g, err := replayGetGoal(r)
	if err != nil {
		return err
	}
	return g.object.decRef()
}
