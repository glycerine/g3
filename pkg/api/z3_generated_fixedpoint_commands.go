package api

// Source: generated api_commands.cpp and src/api/api_datalog.cpp fixedpoint objects.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type replayFixedpoint struct {
	object apiObject
	owner  *replayContextOwner
	value  *APIFixedpoint
}

func (f *replayFixedpoint) apiDestroy() error { return f.value.Close() }
func registerZ3GeneratedFixedpointCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{511, "Z3_mk_fixedpoint"}, {512, "Z3_fixedpoint_inc_ref"}, {513, "Z3_fixedpoint_dec_ref"},
		{515, "Z3_fixedpoint_add_fact"}, {516, "Z3_fixedpoint_assert"}, {518, "Z3_fixedpoint_query_relations"},
		{520, "Z3_fixedpoint_get_reason_unknown"}, {525, "Z3_fixedpoint_get_statistics"}, {526, "Z3_fixedpoint_register_relation"},
		{528, "Z3_fixedpoint_get_rules"}, {529, "Z3_fixedpoint_get_assertions"}, {536, "Z3_fixedpoint_push"}, {537, "Z3_fixedpoint_pop"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayFixedpointCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayStoreFixedpoint(r *Z3Replayer, owner *replayContextOwner, value *APIFixedpoint) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayFixedpoint{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	f := &replayFixedpoint{owner: owner, value: value}
	f.object = apiObject{context: owner.results, destructor: f, ownerBytes: n}
	f.object.self = &f.object
	id, err := owner.results.addObject(&f.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	f.object.id = id
	if err = owner.results.saveObject(&f.object); err != nil {
		return err
	}
	if r.fixedpoints == nil {
		r.fixedpoints = make(map[uint64]*replayFixedpoint)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.fixedpoints[replayID] = f
	owner.fixedpointIDs = append(owner.fixedpointIDs, replayID)
	return r.StoreResult(replayID)
}
func replayGetFixedpoint(r *Z3Replayer) (*replayFixedpoint, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	id, err := r.Object(1)
	if err != nil {
		return nil, err
	}
	f := r.fixedpoints[id]
	if f == nil || f.owner != owner || f.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return f, nil
}
func replayFixedpointCommand(r *Z3Replayer, id uint32) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 511 {
		value, e := owner.context.NewFixedpoint()
		if e != nil {
			return e
		}
		return replayStoreFixedpoint(r, owner, value)
	}
	if id == 512 || id == 513 {
		object, e := r.Object(1)
		if e != nil || object == 0 {
			return e
		}
	}
	f, err := replayGetFixedpoint(r)
	if err != nil {
		return err
	}
	switch id {
	case 512:
		return f.object.incRef()
	case 513:
		return f.object.decRef()
	case 515:
		decl, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		count, e := r.Uint(3)
		if e != nil {
			return e
		}
		values, e := r.UintArray(4)
		if e != nil {
			return e
		}
		if uint64(count) > uint64(len(values)) {
			return ErrZ3ReplayReference
		}
		err = f.value.AddTableFact(decl.handle, values[:count])
	case 516:
		value, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		err = f.value.Assert(value.handle)
	case 518:
		decls, e := replayASTArray(r, owner, 2, 3)
		if e != nil {
			return e
		}
		_, err = f.value.QueryRelations(decls)
	case 520:
		_, err = f.value.ReasonUnknown()
	case 525:
		value, e := f.value.StatisticsObject()
		if e != nil {
			return e
		}
		return replayStoreStatistics(r, owner, value)
	case 526:
		var decl *replayASTOwner
		decl, err = r.astObject(owner, 2)
		if err == nil {
			err = f.value.RegisterRelation(decl.handle)
		}
	case 528, 529:
		var refs []ast.ASTRef
		if id == 528 {
			refs, err = f.value.Rules()
		} else {
			refs, err = f.value.Assertions()
		}
		if err != nil {
			return err
		}
		manager := owner.results.manager
		defer func() {
			for i := range refs {
				err = errors.Join(err, manager.DelRef(&refs[i]))
			}
		}()
		values := make([]ast.ASTHandle, len(refs))
		for i := range refs {
			values[i], err = manager.RefGet(refs[i])
			if err != nil {
				return err
			}
		}
		vector, e := owner.context.NewASTVectorValues(values...)
		if e != nil {
			return e
		}
		return replayStoreVector(r, owner, vector)
	case 536:
		err = f.value.Push()
	case 537:
		err = f.value.Pop()
	}
	return err
}
