package cmd_context

// Source: src/api/api_tactic.cpp Z3_get_num_tactics/Z3_get_tactic_name and
// the retained tactic factories dispatched by tactic_commands.go. The slice
// preserves the observable order of the concrete Go factories.

import "github.com/glycerine/g3/pkg/arena"

var builtinTacticNames = [...]string{
	"qe-light", "qsat", "nlqsat", "nlqe", "qe2", "qe_rec",
	"smt", "nlsat", "nra", "horn", "sat", "sat-preprocess", "subpaving",
	"simplify", "ctx-simplify", "add-bounds", "elim-and", "elim-term-ite",
	"tseitin-cnf", "tseitin-cnf-core", "factor", "propagate-values",
	"propagate-ineqs", "max-bv-sharing", "bit-blast", "bv1-blast",
	"elim-small-bv", "fpa2bv", "dt2bv", "bv_bound_chk", "reduce-bv-size",
	"reduce-args", "ackermannize_bv", "aig", "solve-eqs", "elim-uncnstr",
	"occf", "split-clause", "symmetry-reduce", "der", "distribute-forall",
	"nnf", "snf", "macro-finder", "quasi-macros", "ufbv", "bv",
	"fix-dl-var", "normalize-bounds", "nla2bv", "diff-neq", "lia2pb",
	"card2bv", "pb2bv", "qfbv", "qfaufbv", "qfufbv", "qfufbv_ackr",
	"qfidl", "qflia", "qfnia", "qfnra", "qffp", "qffpbv", "qffplra",
	"qfauflia", "qflra", "qfuf", "ufnia", "uflra", "auflia", "auflira",
	"aufnira", "skip", "fail", "fail-if-undecided",
}

// IsBuiltinTactic reports whether name has a retained concrete factory.
func IsBuiltinTactic(name string) bool {
	for _, registered := range builtinTacticNames {
		if registered == name {
			return true
		}
	}
	return false
}

// BuiltinTacticCount is the source registry size.
func BuiltinTacticCount() uint32 { return uint32(len(builtinTacticNames)) }

// BuiltinTacticName returns the source-ordered name at index.
func BuiltinTacticName(index uint32) (string, error) {
	if uint64(index) >= uint64(len(builtinTacticNames)) {
		return "", arena.ErrBounds
	}
	return builtinTacticNames[index], nil
}

// BuiltinTacticNames returns a caller-owned registry snapshot.
func BuiltinTacticNames() []string {
	result := make([]string, len(builtinTacticNames))
	copy(result, builtinTacticNames[:])
	return result
}
