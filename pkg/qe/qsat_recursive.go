package qe

// Source: src/qe/qsat.cpp elim_rec/elim; src/qe/qe.cpp extract_vars.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// extractVariables is extract_vars(q, new_body, vars): vars gets fresh
// constants and new_body the instantiated body. Its locals (tmp, subst,
// shift) end when it returns: shift, then subst, then tmp.
func (c *qsatSearchCall) extractVariables(q ast.ASTHandle, body ast.ASTRef, vars *[]ast.ASTRef) error {
	s := c.owner
	x := s.call()
	if err := x.extractVariablesCore(q, body, vars); err != nil {
		// Failed-call owners stay on pending for the outer cleanup receipt.
		return err
	}
	return s.closeRound(x)
}

func (c *qsatSearchCall) extractVariablesCore(q ast.ASTHandle, body ast.ASTRef, vars *[]ast.ASTRef) error {
	s := c.owner
	m := s.manager
	tmp, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	n, err := m.GetNumDeclarations(q)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		sort, err := m.GetDeclarationSort(q, i)
		if err != nil {
			return err
		}
		v, err := m.MkFreshConst("x", sort)
		if err != nil {
			return err
		}
		if err = s.appendRef(vars, v); err != nil {
			return err
		}
	}
	values := make([]ast.ASTHandle, len(*vars))
	for i, r := range *vars {
		values[i], err = m.RefGet(r)
		if err != nil {
			return err
		}
	}
	sub, err := rewriter.NewVarSubstitution(m, true)
	if sub != nil {
		c.substitutions = append(c.substitutions, sub)
	}
	if err != nil {
		return err
	}
	h, err := m.RefGet(body)
	if err != nil {
		return err
	}
	if err = sub.Substitute(h, values, tmp); err != nil {
		return err
	}
	shift, err := rewriter.NewInverseVarShifter(m)
	if shift != nil {
		c.shifters = append(c.shifters, shift)
	}
	if err != nil {
		return err
	}
	h, err = m.RefGet(tmp)
	if err != nil {
		return err
	}
	return shift.InverseShift(h, uint32(len(values)), body)
}

// eliminate is qsat::elim(vars, _fml). It returns the expr_ref result as an
// owned reference (a null reference when elimination is not possible); its
// locals (fml, defs) end when it returns, defs first.
func (s *qsatSearch) eliminate(vars []ast.ASTRef, original ast.ASTHandle) (result ast.ASTRef, err error) {
	c := s.call()
	result, err = s.eliminateCore(c, vars, original)
	if err != nil {
		// Failed-call owners stay on pending for the outer cleanup receipt.
		return result, err
	}
	// ~expr_ref_vector defs, then ~expr_ref fml.
	if err = s.releaseOperatorDefs(); err != nil {
		return result, err
	}
	return result, s.closeRound(c)
}

// eliminateCore solves ex (x) phi(x).
//
// BACKPORT upstream z3 1ce0d7512 / 8ad939a10 (fix #2974/#2990): elim uses
// the same code path as qe: re-wrap as exists vars. fml, negate, hoist
// (which gives every free constant a level, so abstract_atoms no longer
// dereferences a missing level for e.g. exists x. x or free), and require a
// ground result. It returns a null expr_ref when fml has nested quantifiers
// or the check is not l_false; the answer is existentially closed only over
// the projected free vars that still occur in it. This replaces the old
// [empty, vars] + initialize_levels setup, its dedupe of m_free_vars, and
// g3's former ComputeLevel workaround for the old crash.
func (s *qsatSearch) eliminateCore(c *qsatSearchCall, vars []ast.ASTRef, original ast.ASTHandle) (result ast.ASTRef, err error) {
	m := s.manager
	// expr_ref fml(_fml, m);
	formula, err := c.ref(original)
	if err != nil {
		return result, err
	}
	// if (has_quantifiers(fml)) return expr_ref(m);
	quantified, err := m.HasQuantifiers(original)
	if err != nil {
		return result, err
	}
	if quantified {
		return m.NewRef(ast.ASTHandle{})
	}
	if err = s.resetState(); err != nil {
		return result, err
	}
	// fml = ::mk_exists(m, vars.size(), vars.c_ptr(), fml);
	bound := make([]ast.ASTHandle, len(vars))
	for i, r := range vars {
		if bound[i], err = m.RefGet(r); err != nil {
			return result, err
		}
	}
	if err = s.mkExists(bound, original, formula); err != nil {
		return result, err
	}
	// fml = ::push_not(fml);
	h, err := m.RefGet(formula)
	if err != nil {
		return result, err
	}
	if h, err = m.PushNot(h); err != nil {
		return result, err
	}
	if err = m.SetRef(formula, h); err != nil {
		return result, err
	}
	// hoist(fml);
	if err = s.preparation.hoist(s.mode, formula); err != nil {
		return result, err
	}
	if h, err = m.RefGet(formula); err != nil {
		return result, err
	}
	ground, err := m.IsGround(h)
	if err != nil {
		return result, err
	}
	if !ground {
		return result, tactic.NewTacticException("formula is not hoistable")
	}
	if err = s.prepareKernels(c, formula); err != nil {
		return result, err
	}
	if err = s.preparation.abstraction.emit("elim", 1063, func(w io.Writer) error {
		_, _ = io.WriteString(w, "ex: ")
		h, e := m.RefGet(formula)
		if e != nil {
			return e
		}
		if e = writeExpression(m, s.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return result, err
	}
	status, err := s.checkLoop()
	if err != nil {
		return result, err
	}
	if status != lbool.False {
		// default: return expr_ref(m);
		return m.NewRef(ast.ASTHandle{})
	}
	// fml = ::mk_and(m_answer);
	answers := make([]ast.ASTHandle, len(s.answers))
	for i, r := range s.answers {
		if answers[i], err = m.RefGet(r); err != nil {
			return result, err
		}
	}
	h, err = m.MkAndArray(answers)
	if err != nil {
		return result, err
	}
	if err = m.SetRef(formula, h); err != nil {
		return result, err
	}
	// for (app* v : m_free_vars) if (occurs(v, fml)) m_free_vars[j++] = v;
	// m_free_vars.shrink(j);
	j := 0
	for i := range s.freeVariables {
		v, err := m.RefGet(s.freeVariables[i])
		if err != nil {
			return result, err
		}
		occurs, err := m.OccursExpr(v, h)
		if err != nil {
			return result, err
		}
		if occurs {
			if err = m.SetRef(s.freeVariables[j], v); err != nil {
				return result, err
			}
			j++
		}
	}
	// shrink's dec_range_ref releases the tail front to back.
	for k := j; k < len(s.freeVariables); k++ {
		if err = m.DelRef(&s.freeVariables[k]); err != nil {
			return result, err
		}
	}
	s.freeVariables = s.freeVariables[:j]
	if len(s.freeVariables) > 0 {
		free := make([]ast.ASTHandle, len(s.freeVariables))
		for i, r := range s.freeVariables {
			if free[i], err = m.RefGet(r); err != nil {
				return result, err
			}
		}
		// fml = ::mk_exists(m, m_free_vars.size(), m_free_vars.c_ptr(), fml);
		if err = s.mkExists(free, h, formula); err != nil {
			return result, err
		}
	}
	// return fml;
	h, err = m.RefGet(formula)
	if err != nil {
		return result, err
	}
	return m.NewRef(h)
}

// mkExists is fml = mk_exists(m, n, bound, body) with the abstractor scoped
// to the call.
func (s *qsatSearch) mkExists(bound []ast.ASTHandle, body ast.ASTHandle, formula ast.ASTRef) (err error) {
	m := s.manager
	a, err := ast.NewExprAbstractor(m)
	if err != nil {
		if a != nil {
			err = errors.Join(err, a.Close())
		}
		return err
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return errors.Join(err, a.Close())
	}
	if err = a.MkExists(bound, body, out); err == nil {
		err = a.Close()
		a = nil
	}
	if err == nil {
		var h ast.ASTHandle
		if h, err = m.RefGet(out); err == nil {
			err = m.SetRef(formula, h)
		}
	}
	if a != nil {
		err = errors.Join(err, a.Close())
	}
	return errors.Join(err, m.DelRef(&out))
}

func (s *qsatSearch) eliminateRecursive(root ast.ASTHandle, out ast.ASTRef) (err error) {
	c, err := s.begin()
	if err != nil {
		return err
	}
	defer s.finish(c, &err)
	// fml = elim_rec(fml): the caller's fml holds root until it is
	// assigned the result.
	r, err := s.eliminateRecursiveFrame(root)
	if err != nil {
		return err
	}
	h, err := s.manager.RefGet(r)
	if err == nil {
		err = s.manager.SetRef(out, h)
	}
	return errors.Join(err, s.manager.DelRef(&r))
}

// eliminateRecursiveFrame is qsat::elim_rec(fml): its own tmp and trail
// end when it returns, after the result reference is taken.
func (s *qsatSearch) eliminateRecursiveFrame(root ast.ASTHandle) (result ast.ASTRef, err error) {
	m := s.manager
	c := s.call()
	// expr_ref tmp(m); expr_ref_vector trail(m);
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	trail := c.vector()
	result, err = s.eliminateRecursiveLoop(c, root, tmp, trail)
	if err != nil {
		// Failed-call owners stay on pending for the outer cleanup receipt.
		return result, errors.Join(err, m.DelRef(&tmp))
	}
	// ~trail (front to back), then ~tmp.
	for i := range *trail {
		if err = m.DelRef(&(*trail)[i]); err != nil {
			return result, err
		}
	}
	*trail = nil
	if err = m.DelRef(&tmp); err != nil {
		return result, err
	}
	return result, s.closeRound(c)
}

func (s *qsatSearch) eliminateRecursiveLoop(c *qsatSearchCall, root ast.ASTHandle, tmp ast.ASTRef, trail *[]ast.ASTRef) (result ast.ASTRef, err error) {
	m := s.manager
	visited := make(map[ast.ASTHandle]ast.ASTHandle)
	if err = s.appendRef(trail, root); err != nil {
		return result, err
	}
	todo := []ast.ASTHandle{root}
	for len(todo) > 0 {
		if err := s.checkpoint(); err != nil {
			return result, err
		}
		h := todo[len(todo)-1]
		if _, ok := visited[h]; ok {
			todo = todo[:len(todo)-1]
			continue
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return result, err
		}
		switch kind {
		case ast.ASTApp:
			n, err := m.GetNumArgs(h)
			if err != nil {
				return result, err
			}
			args := make([]ast.ASTHandle, 0, n)
			ready := true
			for i := uint32(0); i < n; i++ {
				child, err := m.GetArg(h, i)
				if err != nil {
					return result, err
				}
				if v, ok := visited[child]; ok {
					args = append(args, v)
				} else {
					todo = append(todo, child)
					ready = false
				}
			}
			if ready {
				decl, err := m.GetDecl(h)
				if err != nil {
					return result, err
				}
				v, err := m.MkApp(decl, args)
				if err != nil {
					return result, err
				}
				todo = todo[:len(todo)-1]
				if err = s.appendRef(trail, v); err != nil {
					return result, err
				}
				visited[h] = v
			}
		case ast.ASTQuantifier:
			v, err := s.eliminateRecursiveQuantifier(h, tmp)
			if err != nil {
				return result, err
			}
			// BACKPORT upstream z3 1ce0d7512 / 8ad939a10 (fix #2974/#2990):
			// a null elim result keeps the quantifier e itself.
			if v.IsNull() {
				visited[h] = h
				todo = todo[:len(todo)-1]
				break
			}
			if err = s.appendRef(trail, v); err != nil {
				return result, err
			}
			visited[h] = v
			todo = todo[:len(todo)-1]
		default:
			return result, ast.ErrASTDomain
		}
	}
	// VERIFY(visited.find(fml, e)); return expr_ref(e, m);
	return m.NewRef(visited[root])
}

// eliminateRecursiveQuantifier is elim_rec's AST_QUANTIFIER case; its
// app_ref_vector vars ends with the case block. A null handle result means
// elim could not eliminate the quantifier (tmp is then null).
func (s *qsatSearch) eliminateRecursiveQuantifier(h ast.ASTHandle, tmp ast.ASTRef) (ast.ASTHandle, error) {
	c := s.call()
	v, err := s.eliminateRecursiveQuantifierCore(c, h, tmp)
	if err != nil {
		// Failed-call owners stay on pending for the outer cleanup receipt.
		return v, err
	}
	return v, s.closeRound(c)
}

func (s *qsatSearch) eliminateRecursiveQuantifierCore(c *qsatSearchCall, h ast.ASTHandle, tmp ast.ASTRef) (ast.ASTHandle, error) {
	m := s.manager
	vars := c.vector()
	forall, err := m.IsForall(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	body, err := m.GetQuantifierBody(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// tmp = q->get_expr(); extract_vars(q, tmp, vars);
	if err = m.SetRef(tmp, body); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.extractVariables(h, tmp, vars); err != nil {
		return ast.ASTHandle{}, err
	}
	body, err = m.RefGet(tmp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.preparation.abstraction.emit("elim_rec", 1000, func(w io.Writer) error {
		if e := writeExpressionVector(m, s.config, w, *vars); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " ")
		if e := writeExpression(m, s.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " ")
		if e := writeExpression(m, s.config, w, body); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	// tmp = elim_rec(tmp);
	inner, err := s.eliminateRecursiveFrame(body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.assignTemporary(tmp, inner); err != nil {
		return ast.ASTHandle{}, err
	}
	if forall {
		if err = s.pushNotInto(tmp); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	body, err = m.RefGet(tmp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// BACKPORT upstream z3 1ce0d7512 / 8ad939a10 (fix #2974/#2990): trace
	// elim's input, and keep e when elim returns null.
	if err = s.preparation.abstraction.emit("elim_rec", 1008, func(w io.Writer) error {
		_, _ = io.WriteString(w, "elim-rec ")
		if e := writeExpression(m, s.config, w, body); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	// tmp = elim(vars, tmp);
	eliminated, err := s.eliminate(*vars, body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.assignTemporary(tmp, eliminated); err != nil {
		return ast.ASTHandle{}, err
	}
	body, err = m.RefGet(tmp)
	if err != nil || body.IsNull() {
		return ast.ASTHandle{}, err
	}
	if forall {
		if err = s.pushNotInto(tmp); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	body, err = m.RefGet(tmp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// Source traces tmp after trail.push_back(tmp); the caller's push
	// follows here instead, which the trace (no ownership effect) permits.
	if err = s.preparation.abstraction.emit("elim_rec", 1019, func(w io.Writer) error {
		if e := writeExpression(m, s.config, w, body); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	return body, nil
}

// assignTemporary is `dst = f(...)` for an expr_ref temporary result:
// dst takes the value, then the temporary dies.
func (s *qsatSearch) assignTemporary(dst ast.ASTRef, temporary ast.ASTRef) error {
	h, err := s.manager.RefGet(temporary)
	if err == nil {
		err = s.manager.SetRef(dst, h)
	}
	return errors.Join(err, s.manager.DelRef(&temporary))
}

// pushNotInto is `r = ::push_not(r)`.
func (s *qsatSearch) pushNotInto(r ast.ASTRef) error {
	h, err := s.manager.RefGet(r)
	if err != nil {
		return err
	}
	negated, err := s.manager.PushNot(h)
	if err != nil {
		return err
	}
	temporary, err := s.manager.NewRef(negated)
	if err != nil {
		return err
	}
	return s.assignTemporary(r, temporary)
}
