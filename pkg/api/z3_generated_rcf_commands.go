package api

// Source: generated api_commands.cpp and src/api/api_rcf.cpp. Only rational
// values are supported until the real-closure extensions are implemented.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

type replayRCFNumber struct {
	owner      *replayContextOwner
	value      *RCFNumber
	refs       uint32
	ownerBytes uint64
	budget     *arena.Budget
}

func (n *replayRCFNumber) close() error {
	if err := n.value.Close(); err != nil {
		return err
	}
	n.budget.Release(n.ownerBytes)
	return nil
}

func registerZ3GeneratedRCFCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{488, "Z3_rcf_del"}, {489, "Z3_rcf_mk_rational"}, {490, "Z3_rcf_mk_small_int"},
		{502, "Z3_rcf_lt"}, {503, "Z3_rcf_gt"}, {504, "Z3_rcf_le"}, {505, "Z3_rcf_ge"}, {506, "Z3_rcf_eq"}, {507, "Z3_rcf_neq"},
		{508, "Z3_rcf_num_to_string"}, {509, "Z3_rcf_num_to_decimal_string"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayRCFCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}

func replayStoreRCF(r *Z3Replayer, owner *replayContextOwner, value *RCFNumber) error {
	zero, err := value.service.IsZero(&value.value)
	if err != nil {
		return errors.Join(err, value.Close())
	}
	if zero {
		return errors.Join(value.Close(), r.StoreResult(0))
	}
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayRCFNumber{}).Size())
	if err = owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	if r.rcfNumbers == nil {
		r.rcfNumbers = make(map[uint64]*replayRCFNumber)
	}
	r.nextObjectID++
	id := r.nextObjectID
	r.rcfNumbers[id] = &replayRCFNumber{owner: owner, value: value, refs: 1, ownerBytes: n, budget: owner.results.manager.Budget()}
	owner.rcfNumberIDs = append(owner.rcfNumberIDs, id)
	return r.StoreResult(id)
}

// The source uses a null numeral pointer for zero. Temporaries give the Go
// rational API a valid zero owner without assigning an identity to it.
func replayRCFAt(r *Z3Replayer, owner *replayContextOwner, position uint32) (*RCFNumber, bool, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, false, err
	}
	if id == 0 {
		value, e := owner.context.RCFSmallInt(0)
		return value, true, e
	}
	n := r.rcfNumbers[id]
	if n == nil || n.owner != owner || n.refs == 0 {
		return nil, false, ErrZ3ReplayReference
	}
	return n.value, false, nil
}

func replayRCFCommand(r *Z3Replayer, id uint32) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 489 || id == 490 {
		var value *RCFNumber
		if id == 489 {
			text, e := r.String(1)
			if e != nil {
				return e
			}
			value, err = owner.context.RCFRational(text)
		} else {
			n, e := r.Int(1)
			if e != nil {
				return e
			}
			value, err = owner.context.RCFSmallInt(n)
		}
		if err != nil {
			return err
		}
		return replayStoreRCF(r, owner, value)
	}
	if id == 488 {
		object, e := r.Object(1)
		if e != nil || object == 0 {
			return e
		}
		n := r.rcfNumbers[object]
		if n == nil || n.owner != owner || n.refs == 0 {
			return ErrZ3ReplayReference
		}
		n.refs--
		if n.refs == 0 {
			if e = n.close(); e != nil {
				n.refs++
				return e
			}
			delete(r.rcfNumbers, object)
		}
		return nil
	}
	first, temporary, err := replayRCFAt(r, owner, 1)
	if err != nil {
		return err
	}
	if temporary {
		defer func() { err = errors.Join(err, first.Close()) }()
	}
	if id >= 502 && id <= 507 {
		second, secondTemporary, e := replayRCFAt(r, owner, 2)
		if e != nil {
			return e
		}
		if secondTemporary {
			defer func() { err = errors.Join(err, second.Close()) }()
		}
		switch id {
		case 502:
			_, err = owner.context.RCFLess(first, second)
		case 503:
			_, err = owner.context.RCFGreater(first, second)
		case 504:
			_, err = owner.context.RCFLessEqual(first, second)
		case 505:
			_, err = owner.context.RCFGreaterEqual(first, second)
		case 506:
			_, err = owner.context.RCFEqual(first, second)
		case 507:
			_, err = owner.context.RCFNotEqual(first, second)
		}
		return err
	}
	if id == 508 {
		if _, err = r.Bool(2); err != nil {
			return err
		}
		if _, err = r.Bool(3); err != nil {
			return err
		}
		_, err = owner.context.RCFString(first)
		return err
	}
	precision, err := r.Uint(2)
	if err != nil {
		return err
	}
	_, err = owner.context.RCFDecimalString(first, precision)
	return err
}
