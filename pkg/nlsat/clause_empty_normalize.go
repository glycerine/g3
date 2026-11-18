package nlsat

// Source: src/nlsat/nlsat_solver.cpp:694-711 (mk_clause), reached with an
// empty literal list only from update_clauses under inline_vars.

import "github.com/glycerine/g3/pkg/sat"

// normalizeClauseLiterals is a deliberate guard, not a port: mk_clause has
// SASSERT(num_lits > 0), and in the release build an empty clause makes
// attach_clause index m_bwatches[null_bool_var], which is undefined
// behaviour. g3 does not copy the UB; it stores the empty clause as the
// unit clause false, its logical meaning (AUDIT.md nlsat_01-F6). No input
// reaching it has been found, so there is no fork fix yet.
func normalizeClauseLiterals(literals []sat.Literal) []sat.Literal {
	if len(literals) != 0 {
		return literals
	}
	return []sat.Literal{FalseLiteral}
}
