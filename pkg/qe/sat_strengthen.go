package qe

// Source: src/qe/qe_sat_tactic.cpp, NNF strengthening extrapolation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// satExclusiveAtoms walks one atom_set in obj_hashtable iteration order,
// leaving out the atoms remove_duplicates deletes; removal does not reorder
// the remaining entries (AUDIT.md qe_05-F4).
func satExclusiveAtoms(atoms *BoolAtomIndex, positive bool) ([]ast.ASTHandle, error) {
	set, other := atoms.positiveAtoms(), atoms.negativeAtoms()
	table, err := atoms.positiveTable()
	if !positive {
		set, other = other, set
		table, err = atoms.negativeTable()
	}
	if err != nil {
		return nil, err
	}
	out := make([]ast.ASTHandle, 0, len(set))
	for _, h := range table {
		if _, both := other[h]; both {
			continue
		}
		out = append(out, h)
	}
	return out, nil
}

// satStrengthen expects B to be asserted in solver, as in the source tactic.
// The returned expression is owned; all temporary solver scopes are popped.
func satStrengthen(m *ast.ASTManager, config Config, normalizer *QENNFNormalizer, solver *smtcore.SMTSolver, rw *rewriter.TheoryRewriter, a, b ast.ASTHandle, checkpoint func() error) (result ast.ASTRef, err error) {
	if m == nil || normalizer == nil || solver == nil || rw == nil {
		return result, ast.ErrASTHandle
	}
	result, err = m.NewRef(b)
	if err != nil {
		return result, err
	}
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	atoms, err := normalizer.Normalize(result)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, atoms.Close()) }()
	sub, err := m.NewExprSubstitution()
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelExprSubstitution(&sub)) }()
	pos, err := satExclusiveAtoms(atoms, true)
	if err != nil {
		return result, err
	}
	neg, err := satExclusiveAtoms(atoms, false)
	if err != nil {
		return result, err
	}
	if err = solver.Push(); err != nil {
		return result, err
	}
	scoped := true
	defer func() {
		if scoped {
			err = errors.Join(err, solver.Pop(1))
		}
	}()
	if err = solver.AssertFormula(a); err != nil {
		return result, err
	}
	for polarity, set := range [][]ast.ASTHandle{pos, neg} {
		var value ast.ASTHandle
		if polarity == 0 {
			value, err = m.MkFalse()
		} else {
			value, err = m.MkTrue()
		}
		if err != nil {
			return result, err
		}
		for _, atom := range set {
			equal, e := m.MkIff(atom, value)
			if e != nil {
				return result, e
			}
			ref, e := m.NewRef(equal)
			if e != nil {
				return result, e
			}
			if e = solver.Push(); e != nil {
				return result, errors.Join(e, m.DelRef(&ref))
			}
			if e = solver.AssertFormula(equal); e == nil {
				var status lbool.LBool
				status, e = solver.CheckSat(nil)
				if e == nil {
					e = solver.Pop(1)
					if e == nil && status == lbool.True {
						// TRACE("qe", tout << "consistent with: " << mk_pp(*it, m)
						//       << " " << mk_pp(value, m) << "\n";) (AUDIT.md qe_05-F6).
						e = qeTrace(m, config, "nnf_strengthen", "src/qe/qe_sat_tactic.cpp", 512, "consistent with: ", atom, " ", value, "\n")
					}
					if e == nil && status == lbool.True {
						if e = m.ExprSubstitutionInsert(sub, atom, value, ast.ASTHandle{}, ast.ASTDependency{}); e == nil {
							e = solver.AssertFormula(equal)
						}
					}
				} else {
					e = errors.Join(e, solver.Pop(1))
				}
			} else {
				e = errors.Join(e, solver.Pop(1))
			}
			e = errors.Join(e, m.DelRef(&ref))
			if e != nil {
				return result, e
			}
			if checkpoint != nil {
				if e = checkpoint(); e != nil {
					return result, e
				}
			}
		}
	}
	if err = solver.Pop(1); err != nil {
		return result, err
	}
	scoped = false
	replace, err := rewriter.NewDefaultExprReplacer(m)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, replace.Close()) }()
	if err = replace.SetSubstitution(&sub); err != nil {
		return result, err
	}
	old, err := m.RefGet(result)
	if err != nil {
		return result, err
	}
	if err = replace.Rewrite(old, result); err != nil {
		return result, err
	}
	old, err = m.RefGet(result)
	if err != nil {
		return result, err
	}
	if err = rw.Rewrite(old, result); err != nil {
		return result, err
	}
	// TRACE("qe", tout << "A:  " << mk_pp(A, m) << "\n"; tout << "B:  "
	//       << mk_pp(B, m) << "\n"; tout << "B': " << mk_pp(Bnnf.get(), m)
	//       << "\n";) (AUDIT.md qe_05-F6).
	err = qeTrace(m, config, "nnf_strengthening_extrapolate", "src/qe/qe_sat_tactic.cpp", 478, "A:  ", a, "\nB:  ", b, "\nB': ", result, "\n")
	return result, err
}
