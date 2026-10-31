package api

// Source: generated src/api/api_commands.cpp, basic AST reference, sort and
// Boolean construction commands.

// RegisterZ3GeneratedASTCommands installs the implemented basic AST handlers.
func RegisterZ3GeneratedASTCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{9, "Z3_inc_ref", replayIncAST}, {10, "Z3_dec_ref", replayDecAST},
		{32, "Z3_mk_bool_sort", replayBoolSort}, {33, "Z3_mk_int_sort", replayIntSort}, {34, "Z3_mk_real_sort", replayRealSort}, {35, "Z3_mk_bv_sort", replayBVSort},
		{54, "Z3_mk_true", replayTrue}, {55, "Z3_mk_false", replayFalse}, {56, "Z3_mk_eq", replayEqual}, {58, "Z3_mk_not", replayNot},
		{57, "Z3_mk_distinct", replayDistinct}, {59, "Z3_mk_ite", replayIte}, {60, "Z3_mk_iff", replayIff},
		{61, "Z3_mk_implies", replayImplies}, {62, "Z3_mk_xor", replayXor}, {63, "Z3_mk_and", replayAnd}, {64, "Z3_mk_or", replayOr},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return nil
}
func replayIncAST(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	entry, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	if entry.references == ^uint32(0) {
		return ErrZ3ReplayState
	}
	if err = owner.results.manager.IncRef(entry.handle); err != nil {
		return err
	}
	entry.references++
	return nil
}
func replayDecAST(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	entry, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	if entry.references == 0 {
		return ErrZ3ReplayReference
	}
	consumed, err := owner.results.manager.DecRefConsumed(entry.handle)
	if consumed {
		entry.references--
	}
	if err != nil {
		return err
	}
	if !consumed {
		return ErrZ3ReplayState
	}
	return nil
}
func replayBoolSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	h, err := owner.context.BoolSort()
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayIntSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	h, err := owner.context.IntegerSort()
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayRealSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	h, err := owner.context.RealSort()
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayBVSort(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	bits, err := r.Uint(1)
	if err != nil {
		return err
	}
	h, err := owner.context.BitVectorSort(bits)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayBool(r *Z3Replayer, value bool) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	h, err := owner.context.Bool(value)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayTrue(r *Z3Replayer) error  { return replayBool(r, true) }
func replayFalse(r *Z3Replayer) error { return replayBool(r, false) }
func replayNot(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	arg, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	h, err := owner.context.Not(arg.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
func replayEqual(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	left, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	right, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	h, err := owner.context.Equal(left.handle, right.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, h)
}
