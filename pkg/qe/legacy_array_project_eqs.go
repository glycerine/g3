package qe

// Source: src/muz/spacer/spacer_qe_project.cpp:1280-1812
// (array_project_eqs_util) and :2274-2278 (array_project_eqs).

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

const legacyArrayProjectEqsFile = "src/muz/spacer/spacer_qe_project.cpp"

type legacyArrayProjectEqs struct {
	p         *legacyArrayProjection
	m         *ast.ASTManager
	md        *model.ModelCore          // M
	v         ast.ASTHandle             // m_v: array var to eliminate
	hasStores map[ast.ASTHandle]bool    // m_has_stores_v
	substTerm ast.ASTHandle             // m_subst_term_v, owned by vRefs
	trueSub   *rewriter.ExprSafeReplace // m_true_sub_v
	falseSub  *rewriter.ExprSafeReplace // m_false_sub_v
	auxLits   []ast.ASTHandle           // m_aux_lits_v, owned by vRefs
	idxLits   []ast.ASTHandle           // m_idx_lits_v, owned by vRefs
	vRefs     legacyArrayRefs
	auxVars   []ast.ASTHandle // m_aux_vars, owned by auxRefs
	auxRefs   legacyArrayRefs
	outVars   []ast.ASTHandle // aux_vars output, owned by auxRefs
}

func newLegacyArrayProjectEqs(p *legacyArrayProjection) (_ *legacyArrayProjectEqs, resultErr error) {
	m := p.syntax.manager
	u := &legacyArrayProjectEqs{p: p, m: m, hasStores: make(map[ast.ASTHandle]bool),
		vRefs: legacyArrayRefs{manager: m}, auxRefs: legacyArrayRefs{manager: m}}
	var err error
	if u.trueSub, err = rewriter.NewExprSafeReplace(m); err != nil {
		return nil, err
	}
	if u.falseSub, err = rewriter.NewExprSafeReplace(m); err != nil {
		return nil, closeFailedLegacyArrayProjectEqs(u, err)
	}
	return u, nil
}

// Close is the destructor: the per-variable state, then m_aux_vars.
func (u *legacyArrayProjectEqs) Close() error {
	if u == nil || u.m == nil {
		return ast.ErrASTHandle
	}
	if _, err := u.m.ProofMode(); err != nil {
		return err
	}
	if err := u.vRefs.close(); err != nil {
		return err
	}
	for _, replace := range []**rewriter.ExprSafeReplace{&u.falseSub, &u.trueSub} {
		if *replace == nil {
			continue
		}
		if err := (*replace).Close(); err != nil {
			return err
		}
		*replace = nil
	}
	if err := u.auxRefs.close(); err != nil {
		return err
	}
	u.m = nil
	return nil
}

type legacyArrayProjectEqsCleanupFailure struct {
	owner          *legacyArrayProjectEqs
	nested         memory.ConstructionFailure
	runtime        *memory.Runtime
	cause, cleanup error
	reported       error
}

var _ memory.ConstructionFailure = (*legacyArrayProjectEqsCleanupFailure)(nil)

func (f *legacyArrayProjectEqsCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *legacyArrayProjectEqsCleanupFailure) Unwrap() error                 { return f.reported }
func (f *legacyArrayProjectEqsCleanupFailure) OriginalCause() error          { return f.cause }
func (f *legacyArrayProjectEqsCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *legacyArrayProjectEqsCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *legacyArrayProjectEqsCleanupFailure) Cleanup() error {
	if f.nested != nil {
		f.cleanup = f.nested.Cleanup()
		if f.cleanup != nil {
			return f.cleanup
		}
		f.nested = nil
	}
	if f.owner != nil && f.owner.m != nil {
		f.cleanup = f.owner.Close()
		if f.cleanup != nil {
			return f.cleanup
		}
	}
	f.owner = nil
	f.cleanup = nil
	return nil
}

func closeFailedLegacyArrayProjectEqs(owner *legacyArrayProjectEqs, cause error) error {
	m := owner.m
	cleanup := owner.Close()
	if cleanup == nil {
		return cause
	}
	origin := cause
	if origin == nil {
		origin = cleanup
	}
	reported := cleanup
	if cause != nil {
		reported = errors.Join(cause, cleanup)
	}
	var nested memory.ConstructionFailure
	_ = errors.As(cause, &nested)
	return &legacyArrayProjectEqsCleanupFailure{
		owner: owner, nested: nested, runtime: m.NumeralService().RuntimeOwner(),
		cause: origin, cleanup: cleanup, reported: reported,
	}
}

func (u *legacyArrayProjectEqs) trace(function string, line int, body func(io.Writer) error) error {
	tags := u.p.syntax.config.Trace
	if tags == nil {
		tags = u.m.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("qe", function, legacyArrayProjectEqsFile, line,
		func() bool { return true }, func(out io.Writer) error { return body(traceBodyWriter{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// traceTerms prints prefix, then each term followed by a newline.
func (u *legacyArrayProjectEqs) traceTerms(function string, line int, prefix string, terms ...ast.ASTHandle) error {
	return u.trace(function, line, func(out io.Writer) error {
		_, _ = io.WriteString(out, prefix)
		for _, term := range terms {
			if err := writeExpression(u.m, u.p.syntax.config, out, term); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "\n")
		}
		return nil
	})
}

func (u *legacyArrayProjectEqs) sourceAssert(line int32, condition string, holds bool) error {
	if holds {
		return nil
	}
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(debug)
	if err != nil || !enabled {
		return err
	}
	return diagnostic.DebugAssertionFailure(debug, legacyArrayProjectEqsFile, line, condition)
}

func (u *legacyArrayProjectEqs) isApp(h ast.ASTHandle) (bool, error) {
	kind, err := u.m.GetKind(h)
	return kind == ast.ASTApp, err
}

func (u *legacyArrayProjectEqs) args(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	count, err := u.m.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	args := make([]ast.ASTHandle, count)
	for i := range args {
		if args[i], err = u.m.GetArg(h, uint32(i)); err != nil {
			return nil, err
		}
	}
	return args, nil
}

func (u *legacyArrayProjectEqs) isOp(h ast.ASTHandle, op int32) (bool, error) {
	_, matched, err := u.p.arrayOp(h, op)
	return matched, err
}

func (u *legacyArrayProjectEqs) resetV() error {
	u.v = ast.ASTHandle{}
	u.hasStores = make(map[ast.ASTHandle]bool)
	u.substTerm = ast.ASTHandle{}
	if err := u.trueSub.Reset(); err != nil {
		return err
	}
	if err := u.falseSub.Reset(); err != nil {
		return err
	}
	u.auxLits = nil
	u.idxLits = nil
	return u.vRefs.close()
}

// findArrEqs is find_arr_eqs: all array equalities on m_v or containing
// stores on/of m_v, in the order of the source's explicit LIFO traversal;
// it also marks terms containing stores on/of m_v. The equalities are owned
// by eqsRefs.
func (u *legacyArrayProjectEqs) findArrEqs(fml ast.ASTHandle, eqsRefs *legacyArrayRefs) ([]ast.ASTHandle, error) {
	var eqs []ast.ASTHandle
	app, err := u.isApp(fml)
	if err != nil || !app {
		return nil, err
	}
	done := make(map[ast.ASTHandle]bool)
	todo := []ast.ASTHandle{fml}
	for len(todo) != 0 {
		a := todo[len(todo)-1]
		if done[a] {
			todo = todo[:len(todo)-1]
			continue
		}
		allDone := true
		argsHaveStores := false
		args, err := u.args(a)
		if err != nil {
			return nil, err
		}
		for _, arg := range args {
			app, err := u.isApp(arg)
			if err != nil {
				return nil, err
			}
			if !app {
				continue
			}
			if !done[arg] {
				allDone = false
				todo = append(todo, arg)
			} else if !argsHaveStores && u.hasStores[arg] {
				argsHaveStores = true
			}
		}
		if !allDone {
			continue
		}
		todo = todo[:len(todo)-1]

		// mark if a has stores
		isSelect, err := u.isOp(a, ast.ASTArrayOpSelect)
		if err != nil {
			return nil, err
		}
		storeArgs, isStore, err := u.p.arrayOp(a, ast.ASTArrayOpStore)
		if err != nil {
			return nil, err
		}
		if (!isSelect && argsHaveStores) || (isStore && storeArgs[0] == u.v) {
			u.hasStores[a] = true
			if err = u.traceTerms("find_arr_eqs", 1348, "has stores:\n", a); err != nil {
				return nil, err
			}
		}

		// check if a is a relevant array equality
		eqArgs, isEq, err := u.p.syntax.operation(a, u.m.BasicFamilyID(), ast.ASTOpEq)
		if err != nil {
			return nil, err
		}
		if isEq {
			a0, a1 := eqArgs[0], eqArgs[1]
			array, err := u.p.arrayExpression(a0)
			if err != nil {
				return nil, err
			}
			if a0 == u.v || a1 == u.v || (array && u.hasStores[a]) {
				if err = eqsRefs.pin(a); err != nil {
					return nil, err
				}
				eqs = append(eqs, a)
			}
		}
		// else, we can check for disequalities and handle them using
		// extensionality, but it's not necessary

		done[a] = true
	}
	return eqs, nil
}

// factorSelects is factor_selects: factor out select terms on m_v using
// fresh consts. The result is owned by out.
func (u *legacyArrayProjectEqs) factorSelects(fml ast.ASTHandle, out *legacyArrayRefs) (_ ast.ASTHandle, resultErr error) {
	m := u.m
	selCache := make(map[ast.ASTHandle]ast.ASTHandle)
	done := make(map[ast.ASTHandle]bool)
	pinned := &legacyArrayRefs{manager: m} // to ensure a reference
	defer func() { resultErr = closeLegacyArrayRefs(pinned, resultErr) }()

	todo := []ast.ASTHandle{fml}
	for len(todo) != 0 {
		a := todo[len(todo)-1]
		if done[a] {
			todo = todo[:len(todo)-1]
			continue
		}
		var args []ast.ASTHandle
		allDone := true
		children, err := u.args(a)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		for _, arg := range children {
			app, err := u.isApp(arg)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if !app {
				continue
			}
			if !done[arg] {
				allDone = false
				todo = append(todo, arg)
			} else if allDone { // all done so far..
				argNew, ok := selCache[arg]
				if !ok {
					argNew = arg
				}
				args = append(args, argNew)
			}
		}
		if !allDone {
			continue
		}
		todo = todo[:len(todo)-1]

		decl, err := m.GetDecl(a)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		aNew, err := m.MkApp(decl, args)
		if err == nil {
			err = pinned.pin(aNew)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}

		// if a_new is select on m_v, introduce new constant
		isSelect, err := u.isOp(a, ast.ASTArrayOpSelect)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if isSelect && (args[0] == u.v || u.hasStores[args[0]]) {
			vSort, err := m.GetSort(u.v)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			valSort, err := m.ArrayRange(vSort)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			valConst, err := m.MkFreshConst("sel", valSort)
			if err == nil {
				err = u.auxRefs.pin(valConst)
			}
			if err != nil {
				return ast.ASTHandle{}, err
			}
			u.auxVars = append(u.auxVars, valConst)
			// extend M to include val_const
			if err = u.registerValue(valConst, aNew); err != nil {
				return ast.ASTHandle{}, err
			}
			// add equality
			eq, err := m.MkEq(valConst, aNew)
			if err == nil {
				err = u.vRefs.pin(eq)
			}
			if err != nil {
				return ast.ASTHandle{}, err
			}
			u.auxLits = append(u.auxLits, eq)
			// replace select by const
			aNew = valConst
		}

		if a != aNew {
			selCache[a] = aNew
		}
		done[a] = true
	}
	res, ok := selCache[fml]
	if !ok {
		res = fml
	}
	return res, out.pin(res)
}

// registerValue is m_mev.eval(*M, term, val); M->register_decl(c, val).
func (u *legacyArrayProjectEqs) registerValue(constant, term ast.ASTHandle) (resultErr error) {
	local := &legacyArrayRefs{manager: u.m}
	defer func() { resultErr = closeLegacyArrayRefs(local, resultErr) }()
	val, err := u.p.mevEval(u.md, term, local)
	if err != nil {
		return err
	}
	decl, err := u.m.GetDecl(constant)
	if err != nil {
		return err
	}
	return u.md.RegisterConstant(decl, val)
}

// convertPeqToEq is convert_peq_to_eq: convert the partial equality pExp to
// an equality by recursively adding stores on diff indices; the stores go on
// the lhs or rhs as storesOnRHS is false/true. The result is owned by out.
func (u *legacyArrayProjectEqs) convertPeqToEq(pExp ast.ASTHandle, storesOnRHS bool, out *legacyArrayRefs) (ast.ASTHandle, error) {
	m := u.m
	p, err := u.p.legacyPeqOf(pExp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	eq, diffValConsts, err := u.p.mkPeqEq(p, storesOnRHS)
	if err == nil {
		err = out.pin(eq)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, c := range diffValConsts {
		if err = u.auxRefs.pin(c); err != nil {
			return ast.ASTHandle{}, err
		}
		u.auxVars = append(u.auxVars, c)
	}
	// extend M to include diff_val_consts
	arr := p.lhs
	I := p.indices
	numDiff := len(diffValConsts)
	if err = u.sourceAssert(1456, "num_diff == I.size ()", numDiff == len(I)); err != nil {
		return ast.ASTHandle{}, err
	}
	for i := 0; i < numDiff; i++ {
		// mk val term
		valTerm, err := m.MkFamilyApp(u.p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{arr, I[i]})
		if err == nil {
			err = out.pin(valTerm)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		// evaluate and assign to ith diff_val_const
		if err = u.registerValue(diffValConsts[i], valTerm); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return eq, nil
}

// findSubstTerm is find_subst_term.
func (u *legacyArrayProjectEqs) findSubstTerm(eq ast.ASTHandle) (resultErr error) {
	m := u.m
	const function = "find_subst_term"
	local := &legacyArrayRefs{manager: m}
	defer func() { resultErr = closeLegacyArrayRefs(local, resultErr) }()
	eqArgs, err := u.args(eq)
	if err != nil {
		return err
	}
	pExp, err := u.p.mkPeq(eqArgs[0], eqArgs[1], nil)
	if err == nil {
		err = local.pin(pExp)
	}
	if err != nil {
		return err
	}
	substEqFound := false
	for {
		if err = u.traceTerms(function, 1484, "processing peq:\n", pExp); err != nil {
			return err
		}

		p, err := u.p.legacyPeqOf(pExp)
		if err != nil {
			return err
		}
		lhs, rhs := p.lhs, p.rhs
		if !u.hasStores[lhs] {
			lhs, rhs = rhs, lhs
		}
		if u.hasStores[lhs] {
			/** project using the equivalence:
			 *
			 *  (store(arr0,idx,x) ==I arr1) <->
			 *
			 *  (idx \in I => (arr0 ==I arr1)) /\
			 *  (idx \not\in I => (arr0 ==I+idx arr1) /\ (arr1[idx] == x)))
			 */
			I := append([]ast.ASTHandle(nil), p.indices...)
			aLHS, err := u.args(lhs)
			if err != nil {
				return err
			}
			if len(aLHS) < 3 {
				return diagnostic.BorrowProcessDebugContext().UnreachableSource(legacyArrayProjectEqsFile, 1508)
			}
			arr0, idx, x := aLHS[0], aLHS[1], aLHS[2]
			arr1 := rhs
			// check if (idx \in I) in M
			idxInI := false
			var idxDiseq []ast.ASTHandle
			if len(I) != 0 {
				val, err := u.p.mevEval(u.md, idx, local)
				if err != nil {
					return err
				}
				for i := 0; i < len(I) && !idxInI; i++ {
					if idx == I[i] {
						idxInI = true
					} else {
						idx1 := I[i]
						idxEq, err := m.MkEq(idx, idx1)
						if err == nil {
							err = local.pin(idxEq)
						}
						if err != nil {
							return err
						}
						val1, err := u.p.mevEval(u.md, idx1, local)
						if err != nil {
							return err
						}
						if val == val1 {
							idxInI = true
							if err = u.vRefs.pin(idxEq); err != nil {
								return err
							}
							u.idxLits = append(u.idxLits, idxEq)
						} else {
							notEq, err := m.MkNot(idxEq)
							if err == nil {
								err = local.pin(notEq)
							}
							if err != nil {
								return err
							}
							idxDiseq = append(idxDiseq, notEq)
						}
					}
				}
			}
			if idxInI {
				// The source prints m_idx_lits_v.back (); it is empty when
				// idx is one of I by pointer, so nothing is printed then.
				if err = u.trace(function, 1536, func(out io.Writer) error {
					_, _ = io.WriteString(out, "store index in diff indices:\n")
					if len(u.idxLits) != 0 {
						if err := writeExpression(m, u.p.syntax.config, out, u.idxLits[len(u.idxLits)-1]); err != nil {
							return err
						}
					}
					_, _ = io.WriteString(out, "\n")
					return nil
				}); err != nil {
					return err
				}

				// arr0 ==I arr1
				pExp, err = u.p.mkPeq(arr0, arr1, I)
				if err == nil {
					err = local.pin(pExp)
				}
				if err != nil {
					return err
				}

				if err = u.traceTerms(function, 1544, "new peq:\n", pExp); err != nil {
					return err
				}
			} else {
				for _, diseq := range idxDiseq {
					if err = u.vRefs.pin(diseq); err != nil {
						return err
					}
				}
				u.idxLits = append(u.idxLits, idxDiseq...)
				// arr0 ==I+idx arr1
				I = append(I, idx)
				pExp, err = u.p.mkPeq(arr0, arr1, I)
				if err == nil {
					err = local.pin(pExp)
				}
				if err != nil {
					return err
				}

				if err = u.traceTerms(function, 1555, "new peq:\n", pExp); err != nil {
					return err
				}

				// arr1[idx] == x
				arr1Idx, err := m.MkFamilyApp(u.p.syntax.family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{arr1, idx})
				if err != nil {
					return err
				}
				eq, err := m.MkEq(arr1Idx, x)
				if err == nil {
					err = u.vRefs.pin(eq)
				}
				if err != nil {
					return err
				}
				u.auxLits = append(u.auxLits, eq)

				if err = u.traceTerms(function, 1568, "new eq:\n", eq); err != nil {
					return err
				}
			}
		} else if lhs == rhs { // trivial peq (a ==I a)
			break
		} else if lhs == u.v || rhs == u.v {
			substEqFound = true
			if err = u.trace(function, 1579, func(out io.Writer) error {
				_, err := io.WriteString(out, "subst eq found!\n")
				return err
			}); err != nil {
				return err
			}
			break
		} else {
			return diagnostic.BorrowProcessDebugContext().UnreachableSource(legacyArrayProjectEqsFile, 1585)
		}
	}

	// factor out select terms on m_v from p_exp using fresh constants
	if substEqFound {
		pExp, err = u.factorSelects(pExp, local)
		if err != nil {
			return err
		}

		if err = u.trace(function, 1593, func(out io.Writer) error {
			_, _ = io.WriteString(out, "after factoring selects:\n")
			if err := writeExpression(m, u.p.syntax.config, out, pExp); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "\n")
			// The source's unsigned start index wraps, printing nothing,
			// when m_aux_vars outgrows this variable's m_aux_lits_v.
			for i := len(u.auxLits) - len(u.auxVars); i >= 0 && i < len(u.auxLits); i++ {
				if err := writeExpression(m, u.p.syntax.config, out, u.auxLits[i]); err != nil {
					return err
				}
				_, _ = io.WriteString(out, "\n")
			}
			return nil
		}); err != nil {
			return err
		}

		// find subst_term
		storesOnRHS := true
		pArgs, err := u.args(pExp)
		if err != nil {
			return err
		}
		if pArgs[1] == u.v {
			storesOnRHS = false
		}
		eq, err := u.convertPeqToEq(pExp, storesOnRHS, local)
		if err != nil {
			return err
		}
		substTerm, err := m.GetArg(eq, 1)
		if err == nil {
			err = u.vRefs.pin(substTerm)
		}
		if err != nil {
			return err
		}
		u.substTerm = substTerm

		if err = u.traceTerms(function, 1611, "subst term found:\n", u.substTerm); err != nil {
			return err
		}
	}
	return nil
}

// project is project: try to substitute for m_v, using array equalities;
// compute the substitution term and aux lits. The source always returns true.
func (u *legacyArrayProjectEqs) project(fml ast.ASTHandle) (resultErr error) {
	m := u.m
	const function = "project"
	eqsRefs := &legacyArrayRefs{manager: m}
	defer func() { resultErr = closeLegacyArrayRefs(eqsRefs, resultErr) }()
	eqs, err := u.findArrEqs(fml, eqsRefs)
	if err != nil {
		return err
	}
	if err = u.traceTerms(function, 1628, "array equalities:\n", eqs...); err != nil {
		return err
	}

	// evaluate eqs in M
	var trueEqs []ast.ASTHandle // subset of eqs; eqs ensures references
	for _, eq := range eqs {
		if err = u.traceTerms(function, 1637, "array equality:\n", eq); err != nil {
			return err
		}

		// evaluate eq in M
		args, err := u.args(eq)
		if err != nil {
			return err
		}
		val, err := u.p.mevEvalArrayEq(u.md, eq, args[0], args[1])
		if err != nil {
			return err
		}
		if val.IsNull() {
			// XXX HACK: unable to evaluate. set to true?
			if val, err = m.MkTrue(); err != nil {
				return err
			}
		}
		if err = u.sourceAssert(1652, "m.is_true (val) || m.is_false (val)", m.IsTrue(val) || m.IsFalse(val)); err != nil {
			return err
		}

		if m.IsFalse(val) {
			falsity, err := m.MkFalse()
			if err == nil {
				err = u.falseSub.Insert(eq, falsity)
			}
			if err != nil {
				return err
			}
		} else {
			trueEqs = append(trueEqs, eq)
		}
	}

	// compute nesting depths of stores on m_v in true_eqs, as follows:
	// 0 if m_v appears on both sides of equality
	// 1 if equality is (m_v=t)
	// 2 if equality is (store(m_v,i,v)=t)
	// ...
	numTrueEqs := len(trueEqs)
	nds := make([]int, numTrueEqs)
	for i := 0; i < numTrueEqs; i++ {
		args, err := u.args(trueEqs[i])
		if err != nil {
			return err
		}
		lhs, rhs := args[0], args[1]
		lhsHasV := lhs == u.v || u.hasStores[lhs]
		rhsHasV := rhs == u.v || u.hasStores[rhs]
		store := ast.ASTHandle{}

		if err = u.sourceAssert(1677, "lhs_has_v || rhs_has_v", lhsHasV || rhsHasV); err != nil {
			return err
		}

		if !lhsHasV {
			store = rhs
		} else if !rhsHasV {
			store = lhs
		}
		// else v appears on both sides -- trivial equality
		// put it in the beginning to simplify it away

		nd := 0 // nesting depth
		if !store.IsNull() {
			for nd = 1; ; nd++ {
				storeArgs, isStore, err := u.p.arrayOp(store, ast.ASTArrayOpStore)
				if err != nil {
					return err
				}
				if !isStore {
					break
				}
				store = storeArgs[0]
			}
			if err = u.sourceAssert(1693, "store == m_v", store == u.v); err != nil {
				return err
			}
		}
		nds[i] = nd
	}

	// sort true_eqs according to nesting depth
	// use insertion sort
	for i := 1; i < numTrueEqs; i++ {
		eq := trueEqs[i]
		nd := nds[i]
		j := i
		for ; j >= 1 && nds[j-1] > nd; j-- {
			trueEqs[j] = trueEqs[j-1]
			nds[j] = nds[j-1]
		}
		if j < i {
			trueEqs[j] = eq
			nds[j] = nd
			if err = u.trace(function, 1714, func(out io.Writer) error {
				_, err := io.WriteString(out, "changing eq order!\n")
				return err
			}); err != nil {
				return err
			}
		}
	}

	// search for subst term
	truth, err := m.MkTrue()
	if err != nil {
		return err
	}
	for i := 0; u.substTerm.IsNull() && i < numTrueEqs; i++ {
		eq := trueEqs[i]
		if err = u.trueSub.Insert(eq, truth); err != nil {
			return err
		}
		// try to find subst term
		if err = u.findSubstTerm(eq); err != nil {
			return err
		}
	}
	return nil
}

// mkResult is mk_result.
func (u *legacyArrayProjectEqs) mkResult(fml ast.ASTRef) (resultErr error) {
	m := u.m
	rw, err := u.p.newTheoryRewriter()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, rw.Close()) }()
	if err = u.rewriteRef(rw, fml); err != nil {
		return err
	}
	// add in aux_lits and idx_lits
	// TODO: eliminate possible duplicates, especially in idx_lits
	//       theory rewriting is a possibility, but not sure if it
	//          introduces unwanted terms such as ite's
	current, err := m.RefGet(fml)
	if err != nil {
		return err
	}
	lits := make([]ast.ASTHandle, 0, len(u.idxLits)+len(u.auxLits)+1)
	lits = append(lits, u.idxLits...)
	lits = append(lits, u.auxLits...)
	lits = append(lits, current)
	conjunction, err := m.MkFamilyApp(m.BasicFamilyID(), ast.ASTOpAnd, nil, lits)
	if err == nil {
		err = m.SetRef(fml, conjunction)
	}
	if err != nil {
		return err
	}

	if !u.substTerm.IsNull() {
		if err = u.trueSub.Insert(u.v, u.substTerm); err != nil {
			return err
		}
		if err = u.trueSub.ApplyRef(fml); err != nil {
			return err
		}
	} else {
		if err = u.trueSub.ApplyRef(fml); err != nil {
			return err
		}
		if err = u.falseSub.ApplyRef(fml); err != nil {
			return err
		}
	}
	if err = u.rewriteRef(rw, fml); err != nil {
		return err
	}
	current, err = m.RefGet(fml)
	if err != nil {
		return err
	}
	return u.sourceAssert(1753, "!m.is_false (fml)", !m.IsFalse(current))
}

// rewriteRef is rw(fml) for an expr_ref fml.
func (u *legacyArrayProjectEqs) rewriteRef(rw *rewriter.TheoryRewriter, fml ast.ASTRef) error {
	current, err := u.m.RefGet(fml)
	if err != nil {
		return err
	}
	rewritten, err := u.p.rewriteWith(rw, current)
	if err != nil {
		return err
	}
	if current, err = u.m.RefGet(rewritten); err == nil {
		err = u.m.SetRef(fml, current)
	}
	return closeFailedLegacyArrayRewrite(u.m, rewritten, err)
}

// run is operator(): arrVars are replaced by the remaining array variables;
// the returned auxiliary variables stay owned by u until Close.
func (u *legacyArrayProjectEqs) run(md *model.ModelCore, arrVars []ast.ASTHandle, fml ast.ASTRef) (remaining, auxVars []ast.ASTHandle, resultErr error) {
	m := u.m
	const function = "operator()"
	u.md = md
	var remArrVars []ast.ASTHandle // remaining arr vars
	for _, v := range arrVars {
		if err := u.resetV(); err != nil {
			return nil, nil, err
		}
		u.v = v
		array, err := u.p.arrayExpression(v)
		if err != nil {
			return nil, nil, err
		}
		if !array {
			if err = u.traceTerms(function, 1780, "not an array variable: ", v); err != nil {
				return nil, nil, err
			}
			if err = u.auxRefs.pin(v); err != nil {
				return nil, nil, err
			}
			u.outVars = append(u.outVars, v)
			continue
		}
		if err = u.traceTerms(function, 1786, "projecting equalities on variable: ", v); err != nil {
			return nil, nil, err
		}

		current, err := m.RefGet(fml)
		if err != nil {
			return nil, nil, err
		}
		if err = u.project(current); err != nil {
			return nil, nil, err
		}
		if err = u.mkResult(fml); err != nil {
			return nil, nil, err
		}

		containsV := u.substTerm.IsNull()
		if !containsV {
			contains, err := ast.NewContainsApp(m, u.v)
			if err != nil {
				return nil, nil, err
			}
			containsV, err = contains.Check(u.substTerm)
			if err = closeFailedLegacyArrayContains(m, contains, err); err != nil {
				return nil, nil, err
			}
		}
		if containsV {
			remArrVars = append(remArrVars, u.v)
		}
		if current, err = m.RefGet(fml); err != nil {
			return nil, nil, err
		}
		if err = u.traceTerms(function, 1797, "after projection: \n", current); err != nil {
			return nil, nil, err
		}
	}
	u.outVars = append(u.outVars, u.auxVars...)
	return remArrVars, u.outVars, nil
}
