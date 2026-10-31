package api

// Source: generated api_commands.cpp and src/api/api_model.cpp.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type replayFunctionInterpretation struct {
	object apiObject
	owner  *replayContextOwner
	value  *APIFunctionInterpretation
}

func (f *replayFunctionInterpretation) apiDestroy() error { return f.value.Close() }
func registerZ3GeneratedFunctionInterpretation(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{301, "Z3_model_get_func_interp"}, {312, "Z3_add_func_interp"}, {314, "Z3_func_interp_inc_ref"}, {315, "Z3_func_interp_dec_ref"},
		{316, "Z3_func_interp_get_num_entries"}, {318, "Z3_func_interp_get_else"}, {319, "Z3_func_interp_set_else"},
		{320, "Z3_func_interp_get_arity"}, {321, "Z3_func_interp_add_entry"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayFunctionInterpretationCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayStoreFunctionInterpretation(r *Z3Replayer, owner *replayContextOwner, value *APIFunctionInterpretation) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayFunctionInterpretation{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	f := &replayFunctionInterpretation{owner: owner, value: value}
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
	if r.functions == nil {
		r.functions = make(map[uint64]*replayFunctionInterpretation)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.functions[replayID] = f
	owner.functionIDs = append(owner.functionIDs, replayID)
	return r.StoreResult(replayID)
}
func replayGetFunctionInterpretation(r *Z3Replayer) (*replayFunctionInterpretation, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	id, err := r.Object(1)
	if err != nil {
		return nil, err
	}
	f := r.functions[id]
	if f == nil || f.owner != owner || f.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return f, nil
}
func replayFunctionInterpretationCommand(r *Z3Replayer, id uint32) error {
	if id == 301 || id == 312 {
		m, err := replayGetModel(r)
		if err != nil {
			return err
		}
		decl, err := r.astObject(m.owner, 2)
		if err != nil {
			return err
		}
		var f *APIFunctionInterpretation
		if id == 301 {
			f, err = m.value.FunctionInterpretation(decl.handle)
		} else {
			value, e := r.astObject(m.owner, 3)
			if e != nil {
				return e
			}
			f, err = m.value.AddFunctionInterpretation(decl.handle, value.handle)
		}
		if err != nil {
			return err
		}
		return replayStoreFunctionInterpretation(r, m.owner, f)
	}
	if id == 314 || id == 315 {
		if _, err := replayContext(r, 0); err != nil {
			return err
		}
		if object, err := r.Object(1); err != nil || object == 0 {
			return err
		}
	}
	f, err := replayGetFunctionInterpretation(r)
	if err != nil {
		return err
	}
	switch id {
	case 314:
		return f.object.incRef()
	case 315:
		return f.object.decRef()
	case 316:
		_, err = f.value.NumEntries()
	case 318:
		value, e := f.value.Else()
		if e != nil {
			return e
		}
		if value.IsNull() {
			return r.StoreResult(0)
		}
		return r.storeAST(f.owner, value)
	case 319:
		var value *replayASTOwner
		value, err = r.astObject(f.owner, 2)
		if err == nil {
			err = f.value.SetElse(value.handle)
		}
	case 320:
		_, err = f.value.Arity()
	case 321:
		var args *replayASTVector
		args, err = replayVectorAt(r, f.owner, 2)
		if err != nil {
			return err
		}
		var value *replayASTOwner
		value, err = r.astObject(f.owner, 3)
		if err != nil {
			return err
		}
		var values []ast.ASTHandle
		values, err = args.value.Values()
		if err == nil {
			err = f.value.AddEntry(values, value.handle)
		}
	}
	return err
}
