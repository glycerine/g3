package api

// Source: generated api_commands.cpp, cardinality and pseudo-Boolean constructors.

import "github.com/glycerine/g3/pkg/ast"

func registerZ3GeneratedPseudoBooleanCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{{222, "Z3_mk_atmost"}, {223, "Z3_mk_atleast"}, {224, "Z3_mk_pble"}, {225, "Z3_mk_pbge"}, {226, "Z3_mk_pbeq"}}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayPseudoBoolean(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayPseudoBoolean(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	args, err := replayASTArray(r, owner, 1, 2)
	if err != nil {
		return err
	}
	var result ast.ASTHandle
	if id == 222 || id == 223 {
		bound, e := r.Uint(3)
		if e != nil {
			return e
		}
		if id == 222 {
			result, err = owner.context.PseudoBooleanAtMost(args, bound)
		} else {
			result, err = owner.context.PseudoBooleanAtLeast(args, bound)
		}
	} else {
		// get_int_array(3). z3 4.7.1 bug kept out of g3: log_Z3_mk_pble,
		// _pbge and _pbeq write the int coefficients with U and Au, so 4.7.1
		// never replays its own logs ("expecting int_array at position 3
		// but got uint_array"). Upstream fed977b49 ("fix #1782") logs I and
		// Ai; g3 reads that int array and still accepts the 4.7.1 uint
		// array, reading its words as ints (AUDIT.md api_03-F3).
		raw, e := r.IntArray(3)
		if e != nil {
			words, uintErr := r.UintArray(3)
			if uintErr != nil {
				return e
			}
			raw = make([]int32, len(words))
			for i, word := range words {
				raw[i] = int32(word)
			}
		}
		if len(raw) < len(args) {
			return ErrZ3ReplayReference
		}
		coefficients := make([]int64, len(args))
		for i := range coefficients {
			coefficients[i] = int64(raw[i])
		}
		bound, e := r.Int(4)
		if e != nil {
			return e
		}
		switch id {
		case 224:
			result, err = owner.context.PseudoBooleanLE(args, coefficients, int64(bound))
		case 225:
			result, err = owner.context.PseudoBooleanGE(args, coefficients, int64(bound))
		case 226:
			result, err = owner.context.PseudoBooleanEQ(args, coefficients, int64(bound))
		}
	}
	if err != nil {
		return err
	}
	return r.storeAST(owner, result)
}
