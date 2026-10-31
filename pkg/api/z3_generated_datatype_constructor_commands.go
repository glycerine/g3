package api

// Source: generated api_commands.cpp and src/api/api_datatype.cpp.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type replayDatatypeConstructor struct {
	owner  *replayContextOwner
	value  *APIDatatypeConstructorObject
	budget *arena.Budget
	bytes  uint64
}

func (c *replayDatatypeConstructor) close() error {
	if err := c.value.Close(); err != nil {
		return err
	}
	c.budget.Release(c.bytes)
	return nil
}

type replayDatatypeList struct {
	owner  *replayContextOwner
	value  *APIDatatypeConstructorList
	budget *arena.Budget
	bytes  uint64
}

func (l *replayDatatypeList) close() error {
	if err := l.value.Close(); err != nil {
		return err
	}
	l.budget.Release(l.bytes)
	return nil
}

func registerZ3GeneratedDatatypeConstructorCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{42, "Z3_mk_constructor"}, {43, "Z3_del_constructor"},
		{45, "Z3_mk_constructor_list"}, {46, "Z3_del_constructor_list"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayDatatypeConstructorCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}

func replayDatatypeConstructorCommand(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 43 || id == 46 {
		object, e := r.Object(1)
		if e != nil || object == 0 {
			return e
		}
		if id == 43 {
			v := r.datatypeConstructors[object]
			if v == nil || v.owner != owner {
				return ErrZ3ReplayReference
			}
			if e = v.close(); e != nil {
				return e
			}
			delete(r.datatypeConstructors, object)
		} else {
			v := r.datatypeLists[object]
			if v == nil || v.owner != owner {
				return ErrZ3ReplayReference
			}
			if e = v.close(); e != nil {
				return e
			}
			delete(r.datatypeLists, object)
		}
		return nil
	}
	if r.nextObjectID == ^uint64(0) {
		return ErrZ3ReplayState
	}
	budget := owner.results.manager.Budget()
	if id == 42 {
		name, e := replayTypedSymbol(r, owner, 1)
		if e != nil {
			return e
		}
		tester, e := replayTypedSymbol(r, owner, 2)
		if e != nil {
			return e
		}
		count, e := r.Uint(3)
		if e != nil {
			return e
		}
		namesValue, e := r.value(4, replaySymbolArray)
		if e != nil {
			return e
		}
		names := r.symbolArrays[namesValue.u]
		sorts, e := r.ObjectArray(5)
		if e != nil {
			return e
		}
		indices, e := r.UintArray(6)
		if e != nil {
			return e
		}
		if uint64(count) > uint64(len(names)) || uint64(count) > uint64(len(sorts)) || uint64(count) > uint64(len(indices)) {
			return ErrZ3ReplayReference
		}
		definition := ast.DatatypeConstructorDefinition{Name: name, Recognizer: tester}
		for i := uint32(0); i < count; i++ {
			field, e := replaySymbolValue(owner, names[i])
			if e != nil {
				return e
			}
			accessor := ast.DatatypeAccessorDefinition{Name: field, Recursive: sorts[i] == 0, RecursiveIndex: indices[i]}
			if sorts[i] != 0 {
				v, e := r.astByID(owner, sorts[i])
				if e != nil {
					return e
				}
				accessor.Range = v.handle
			}
			definition.Accessors = append(definition.Accessors, accessor)
		}
		value, e := owner.context.NewDatatypeConstructorObjectWithSymbols(definition)
		if e != nil {
			return e
		}
		n := uint64(reflect.TypeOf(replayDatatypeConstructor{}).Size())
		if e = budget.Reserve(n); e != nil {
			return errors.Join(e, value.Close())
		}
		if r.datatypeConstructors == nil {
			r.datatypeConstructors = make(map[uint64]*replayDatatypeConstructor)
		}
		r.nextObjectID++
		object := r.nextObjectID
		r.datatypeConstructors[object] = &replayDatatypeConstructor{owner: owner, value: value, budget: budget, bytes: n}
		owner.datatypeConstructorIDs = append(owner.datatypeConstructorIDs, object)
		return r.StoreResult(object)
	}
	count, err := r.Uint(1)
	if err != nil {
		return err
	}
	objects, err := r.ObjectArray(2)
	if err != nil {
		return err
	}
	if uint64(count) > uint64(len(objects)) {
		return ErrZ3ReplayReference
	}
	values := make([]*APIDatatypeConstructorObject, count)
	for i := range values {
		v := r.datatypeConstructors[objects[i]]
		if v == nil || v.owner != owner {
			return ErrZ3ReplayReference
		}
		values[i] = v.value
	}
	value, err := owner.context.NewDatatypeConstructorList(values...)
	if err != nil {
		return err
	}
	n := uint64(reflect.TypeOf(replayDatatypeList{}).Size())
	if err = budget.Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	if r.datatypeLists == nil {
		r.datatypeLists = make(map[uint64]*replayDatatypeList)
	}
	r.nextObjectID++
	object := r.nextObjectID
	r.datatypeLists[object] = &replayDatatypeList{owner: owner, value: value, budget: budget, bytes: n}
	owner.datatypeListIDs = append(owner.datatypeListIDs, object)
	return r.StoreResult(object)
}
