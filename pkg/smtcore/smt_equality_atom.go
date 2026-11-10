package smtcore

// Source: src/smt/smt_context.cpp context::mk_eq_atom, src/smt/smt_theory.h
// theory::mk_eq_atom, and src/ast/arith_decl_plugin.h arith_util::mk_eq.

import "github.com/glycerine/g3/pkg/ast"

// mkEqualityAtom uses the operand sort's theory convention for atoms created
// during search. Arithmetic puts numerals on the right; other theories order
// operands by AST ID. User assertions retain their original syntax.
func (s *SMTInternalizerState) mkEqualityAtom(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	// context::mk_eq_atom: theory_arith, the diff logics and utvpi use
	// arith_util::mk_eq; theory_lra keeps theory::mk_eq_atom (id order).
	return s.MkEqAtom(left, right)
}
