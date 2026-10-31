package api

// Source: generated api_commands.cpp and src/api/api_bv.cpp, basic BV operations.

import "github.com/glycerine/g3/pkg/ast"

func registerZ3GeneratedBitVectorCommands(r *Z3Replayer) error {
	entries := []struct {
		id    uint32
		name  string
		kind  int32
		arity uint32
	}{
		{80, "Z3_mk_bvnot", 32, 1}, {81, "Z3_mk_bvredand", 43, 1}, {82, "Z3_mk_bvredor", 42, 1},
		{83, "Z3_mk_bvand", 30, 2}, {84, "Z3_mk_bvor", 31, 2}, {85, "Z3_mk_bvxor", 33, 2}, {86, "Z3_mk_bvnand", 34, 2},
		{87, "Z3_mk_bvnor", 35, 2}, {88, "Z3_mk_bvxnor", 36, 2}, {89, "Z3_mk_bvneg", 3, 1},
		{90, "Z3_mk_bvadd", 4, 2}, {91, "Z3_mk_bvsub", 5, 2}, {92, "Z3_mk_bvmul", 6, 2}, {93, "Z3_mk_bvudiv", 8, 2},
		{94, "Z3_mk_bvsdiv", 7, 2}, {95, "Z3_mk_bvurem", 10, 2}, {96, "Z3_mk_bvsrem", 9, 2}, {97, "Z3_mk_bvsmod", 11, 2},
		{98, "Z3_mk_bvult", 26, 2}, {99, "Z3_mk_bvslt", 27, 2}, {100, "Z3_mk_bvule", 22, 2}, {101, "Z3_mk_bvsle", 23, 2},
		{102, "Z3_mk_bvuge", 24, 2}, {103, "Z3_mk_bvsge", 25, 2}, {104, "Z3_mk_bvugt", 28, 2}, {105, "Z3_mk_bvsgt", 29, 2},
		{106, "Z3_mk_concat", 37, 2}, {111, "Z3_mk_bvshl", 45, 2}, {112, "Z3_mk_bvlshr", 46, 2}, {113, "Z3_mk_bvashr", 47, 2},
		{116, "Z3_mk_ext_rotate_left", 50, 2}, {117, "Z3_mk_ext_rotate_right", 51, 2},
	}
	for _, entry := range entries {
		operation := entry
		if err := r.RegisterCommand(entry.id, entry.name, func(r *Z3Replayer) error {
			owner, err := replayContext(r, 0)
			if err != nil {
				return err
			}
			args := make([]ast.ASTHandle, operation.arity)
			for i := range args {
				value, e := r.astObject(owner, uint32(i+1))
				if e != nil {
					return e
				}
				args[i] = value.handle
			}
			h, err := owner.context.bitVectorApp(operation.kind, args...)
			if err != nil {
				return err
			}
			return r.storeAST(owner, h)
		}); err != nil {
			return err
		}
	}
	return nil
}
