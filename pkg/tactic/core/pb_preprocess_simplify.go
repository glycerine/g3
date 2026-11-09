package core

// Source: src/tactic/core/pb_preprocess_tactic.cpp:166-276 (simplify, reset),
// 398-506 (process_vars, classify_vars, insert, next_resolvent, is_valid),
// 510-591 (resolve, is_complement) and 657-673 (subsumes).
//
// Deliberate deviations from 4.7.1, both of which can make the source
// unsound: classify_vars records the index into m_ge but later code reads
// it as a goal index, so g3 records the goal index (m_ge[i]); and the
// subsumption pass never resets args2/coeffs2 between candidates, so g3
// rebuilds them for each candidate.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// pbPreprocessRec is rec { pos, neg }.
type pbPreprocessRec struct{ pos, neg []uint32 }

// pbPreprocessRound is one simplify call's m_vars, m_ge and m_other.
type pbPreprocessRound struct {
	vars  *ast.ObjMap // obj_map<app, rec>: Num indexes recs
	recs  []*pbPreprocessRec
	ge    []uint32
	other []uint32
}

func (r *pbPreprocessRound) rec(m *ast.ASTManager, e ast.ASTHandle) (*pbPreprocessRec, bool, error) {
	n, found, err := r.vars.FindNum(e)
	if err != nil || !found {
		return nil, false, err
	}
	return r.recs[n], true, nil
}

// insert is pb_preprocess_tactic::insert.
func (r *pbPreprocessRound) insert(i uint32, e ast.ASTHandle, pos bool) error {
	n, found, err := r.vars.FindNum(e)
	if err != nil {
		return err
	}
	if !found {
		r.recs = append(r.recs, &pbPreprocessRec{})
		n = uint64(len(r.recs) - 1)
		if err = r.vars.InsertNum(e, n); err != nil {
			return err
		}
	}
	if pos {
		r.recs[n].pos = append(r.recs[n].pos, i)
	} else {
		r.recs[n].neg = append(r.recs[n].neg, i)
	}
	return nil
}

func (t *PBPreprocessTactic) isBasic(h ast.ASTHandle, op int32) (bool, error) {
	m := t.manager
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	f, e := m.GetFamilyID(h)
	if e != nil || f != m.BasicFamilyID() {
		return false, e
	}
	o, e := m.GetDeclKind(h)
	return o == op, e
}

// pureArgs is pure_args.
func (t *PBPreprocessTactic) pureArgs(h ast.ASTHandle) (bool, error) {
	n, e := t.manager.GetNumArgs(h)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < n; i++ {
		a, e := t.manager.GetArg(h, i)
		if e != nil {
			return false, e
		}
		if ok, e := t.pureLiteral(a); e != nil || !ok {
			return false, e
		}
	}
	return true, nil
}

// processVars is process_vars: m_ge for the PB fragment, m_other otherwise.
func (t *PBPreprocessTactic) processVars(r *pbPreprocessRound, i uint32, e ast.ASTHandle) error {
	inGE := false
	if c, x := t.isConstant(e); x != nil {
		return x
	} else if c {
		inGE = true
	}
	if !inGE {
		ge, x := t.manager.IsPseudoBooleanGE(e)
		if x != nil {
			return x
		}
		or, x := t.isBasic(e, ast.ASTOpOr)
		if x != nil {
			return x
		}
		if ge || or {
			if inGE, x = t.pureArgs(e); x != nil {
				return x
			}
		}
	}
	if !inGE {
		if child, yes, x := t.negated(e); x != nil {
			return x
		} else if yes {
			if inGE, x = t.isConstant(child); x != nil {
				return x
			}
		}
	}
	if inGE {
		r.ge = append(r.ge, i)
	} else {
		r.other = append(r.other, i)
	}
	return nil
}

// classifyVars is classify_vars.
func (t *PBPreprocessTactic) classifyVars(r *pbPreprocessRound, idx uint32, e ast.ASTHandle) error {
	m := t.manager
	if child, yes, x := t.negated(e); x != nil {
		return x
	} else if yes {
		if c, x := t.isConstant(child); x != nil {
			return x
		} else if c {
			return r.insert(idx, child, false)
		}
	}
	if c, x := t.isConstant(e); x != nil {
		return x
	} else if c {
		return r.insert(idx, e, true)
	}
	n, x := m.GetNumArgs(e)
	if x != nil {
		return x
	}
	for i := uint32(0); i < n; i++ {
		a, x := m.GetArg(e, i)
		if x != nil {
			return x
		}
		if m.IsTrue(a) || m.IsFalse(a) {
			continue
		}
		if child, yes, x := t.negated(a); x != nil {
			return x
		} else if yes {
			if x = r.insert(idx, child, false); x != nil {
				return x
			}
			continue
		}
		if x = r.insert(idx, a, true); x != nil {
			return x
		}
	}
	return nil
}

// declassify is the declassifier over for_each_expr of the m_other
// formulas: applications are visited after their arguments, once each.
func (t *PBPreprocessTactic) declassify(r *pbPreprocessRound, roots []ast.ASTHandle) error {
	m := t.manager
	visited := make(map[ast.ASTHandle]bool)
	for _, root := range roots {
		if r.vars.Len() == 0 {
			return nil
		}
		type frame struct {
			h    ast.ASTHandle
			next uint32
		}
		stack := []frame{{h: root}}
		for len(stack) != 0 {
			top := &stack[len(stack)-1]
			if top.next == 0 && visited[top.h] {
				stack = stack[:len(stack)-1]
				continue
			}
			kind, e := m.GetKind(top.h)
			if e != nil {
				return e
			}
			var count uint32
			switch kind {
			case ast.ASTApp:
				if count, e = m.GetNumArgs(top.h); e != nil {
					return e
				}
			case ast.ASTQuantifier:
				count = 1
			}
			if top.next < count {
				var child ast.ASTHandle
				if kind == ast.ASTQuantifier {
					child, e = m.GetQuantifierBody(top.h)
				} else {
					child, e = m.GetArg(top.h, top.next)
				}
				if e != nil {
					return e
				}
				top.next++
				if !visited[child] {
					stack = append(stack, frame{h: child})
				}
				continue
			}
			visited[top.h] = true
			h := top.h
			stack = stack[:len(stack)-1]
			if kind == ast.ASTApp {
				if found, e := r.vars.Contains(h); e != nil {
					return e
				} else if found {
					if e = r.vars.Erase(h); e != nil {
						return e
					}
				}
			}
		}
	}
	return nil
}

// nextResolvent is next_resolvent over the table-ordered entries.
func nextResolvent(r *pbPreprocessRound, entries []ast.ObjMapEntry, i int) int {
	for i < len(entries) {
		rec := r.recs[entries[i].Num]
		if !(len(rec.pos) > 1 && len(rec.neg) > 1) {
			break
		}
		i++
	}
	return i
}

func (t *PBPreprocessTactic) simplify(g *tactic.Goal, mc *tactic.PBPreprocModelConverter, rw *rewriter.TheoryRewriter) (progress bool, err error) {
	m := t.manager
	r := &pbPreprocessRound{vars: ast.NewObjMap(m)}
	if err = t.normalize(g); err != nil {
		return false, err
	}
	if bad, e := g.Inconsistent(); e != nil || bad {
		return false, e
	}
	n, err := g.Size()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < n; i++ {
		f, e := g.Form(i)
		if e != nil {
			return false, e
		}
		if e = t.processVars(r, i, f); e != nil {
			return false, e
		}
	}
	if len(r.ge) == 0 {
		return false, nil
	}
	for _, idx := range r.ge {
		f, e := g.Form(idx)
		if e != nil {
			return false, e
		}
		if e = t.classifyVars(r, idx, f); e != nil {
			return false, e
		}
	}
	others := make([]ast.ASTHandle, 0, len(r.other))
	for _, idx := range r.other {
		f, e := g.Form(idx)
		if e != nil {
			return false, e
		}
		others = append(others, f)
	}
	if err = t.declassify(r, others); err != nil {
		return false, err
	}
	if r.vars.Len() == 0 {
		return false, nil
	}
	truth, err := m.MkTrue()
	if err != nil {
		return false, err
	}
	falsehood, err := m.MkFalse()
	if err != nil {
		return false, err
	}
	// First eliminate variables. The source's m_vars keys are raw app
	// pointers that survive their substitution; g3 pins them for the round.
	entries := r.vars.Entries()
	pinned := 0
	defer func() {
		for i := pinned - 1; i >= 0; i-- {
			err = errors.Join(err, m.DecRef(entries[i].Key))
		}
	}()
	for _, entry := range entries {
		if err = m.IncRef(entry.Key); err != nil {
			return false, err
		}
		pinned++
	}
	for i := nextResolvent(r, entries, 0); i < len(entries); i = nextResolvent(r, entries, i+1) {
		e := entries[i].Key
		rec := r.recs[entries[i].Num]
		if len(rec.pos) == 0 {
			changed, x := t.replace(g, rw, rec.neg, e, falsehood)
			if x != nil {
				return false, x
			}
			progress = progress || changed
			if x = mc.SetValue(e, false); x != nil {
				return false, x
			}
		} else if len(rec.neg) == 0 {
			changed, x := t.replace(g, rw, rec.pos, e, truth)
			if x != nil {
				return false, x
			}
			progress = progress || changed
			if x = mc.SetValue(e, true); x != nil {
				return false, x
			}
		}
		if bad, x := g.Inconsistent(); x != nil || bad {
			return false, x
		}
	}
	// Now resolve clauses.
	for i := nextResolvent(r, entries, 0); i < len(entries); i = nextResolvent(r, entries, i+1) {
		rec := r.recs[entries[i].Num]
		var resolved bool
		var x error
		if len(rec.pos) == 1 && len(rec.neg) != 0 {
			resolved, x = t.resolve(g, mc, rw, r, rec.pos[0], rec.neg)
		} else if len(rec.neg) == 1 && len(rec.pos) != 0 {
			resolved, x = t.resolve(g, mc, rw, r, rec.neg[0], rec.pos)
		}
		if x != nil {
			return false, x
		}
		progress = progress || resolved
		if bad, x := g.Inconsistent(); x != nil || bad {
			return false, x
		}
	}
	// Now check for subsumption.
	for _, gi := range r.ge {
		f, e := g.Form(gi)
		if e != nil {
			return false, e
		}
		first, e := t.parse(f)
		if e != nil {
			return false, e
		}
		if first == nil || len(first.terms) == 0 {
			continue
		}
		arg := first.terms[0].expression
		neg := false
		if child, yes, x := t.negated(arg); x != nil {
			return false, x
		} else if yes {
			arg, neg = child, true
		}
		if c, x := t.isConstant(arg); x != nil || !c {
			if x != nil {
				return false, x
			}
			continue
		}
		rec, found, x := r.rec(m, arg)
		if x != nil {
			return false, x
		}
		if !found {
			continue
		}
		positions := rec.pos
		if neg {
			positions = rec.neg
		}
		for _, k := range positions {
			if k == gi {
				continue
			}
			fk, x := g.Form(k)
			if x != nil {
				return false, x
			}
			second, x := t.parse(fk)
			if x != nil {
				return false, x
			}
			if second == nil || !pbPreprocessSubsumes(first, second) {
				continue
			}
			di, x := g.Dependency(gi)
			if x != nil {
				return false, x
			}
			dk, x := g.Dependency(k)
			if x != nil {
				return false, x
			}
			joined, x := m.DependencyJoin(di, dk)
			if x != nil {
				return false, x
			}
			if x = g.Update(k, truth, ast.ASTHandle{}, joined); x != nil {
				return false, x
			}
			progress = true
		}
	}
	if err = g.ElimTrue(); err != nil {
		return false, err
	}
	return progress, nil
}

// pbPreprocessSubsumes is subsumes(args1, coeffs1, k1, args2, coeffs2, k2).
func pbPreprocessSubsumes(a, b *pbPreprocessConstraint) bool {
	if a == nil || b == nil || b.bound.Cmp(a.bound) > 0 {
		return false
	}
	for _, left := range a.terms {
		found := false
		for _, right := range b.terms {
			if left.expression == right.expression {
				if left.coefficient.Cmp(right.coefficient) > 0 {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// isComplement is is_complement.
func (t *PBPreprocessTactic) isComplement(x, y ast.ASTHandle) (bool, error) {
	if c, yes, e := t.negated(x); e != nil || yes {
		return yes && c == y, e
	}
	if c, yes, e := t.negated(y); e != nil || yes {
		return yes && x == c, e
	}
	return false, nil
}

// resolve is the special subsumption-resolution case.
func (t *PBPreprocessTactic) resolve(g *tactic.Goal, mc *tactic.PBPreprocModelConverter, rw *rewriter.TheoryRewriter, r *pbPreprocessRound, idx1 uint32, positions []uint32) (bool, error) {
	m := t.manager
	if len(positions) != 1 {
		return false, nil
	}
	idx2 := positions[0]
	f1, err := g.Form(idx1)
	if err != nil {
		return false, err
	}
	first, err := t.parse(f1)
	if err != nil || first == nil {
		return false, err
	}
	if first.bound.Cmp(big.NewRat(1, 1)) != 0 {
		return false, nil
	}
	f2, err := g.Form(idx2)
	if err != nil {
		return false, err
	}
	second, err := t.parse(f2)
	if err != nil || second == nil {
		return false, err
	}
	// Check that each variable in idx1 occurs only in idx2.
	minIndex := 0
	var minCoeff *big.Rat
	var indices []int
	for i, term := range first.terms {
		x := term.expression
		if child, yes, e := t.negated(x); e != nil {
			return false, e
		} else if yes {
			x = child
		}
		if k, e := m.GetKind(x); e != nil || k != ast.ASTApp {
			return false, e
		}
		rec, found, e := r.rec(m, x)
		if e != nil || !found {
			return false, e
		}
		if len(rec.pos) != 1 || len(rec.neg) != 1 {
			return false, nil
		}
		if rec.pos[0] != idx2 && rec.neg[0] != idx2 {
			return false, nil
		}
		for j, other := range second.terms {
			complement, e := t.isComplement(term.expression, other.expression)
			if e != nil {
				return false, e
			}
			if !complement {
				continue
			}
			if i == 0 {
				minCoeff = other.coefficient
			} else if minCoeff != nil && minCoeff.Cmp(other.coefficient) > 0 {
				minCoeff = other.coefficient
				minIndex = j
			}
			indices = append(indices, j)
		}
	}
	truth, err := m.MkTrue()
	if err != nil {
		return false, err
	}
	falsehood, err := m.MkFalse()
	if err != nil {
		return false, err
	}
	terms := append([]pbPreprocessTerm(nil), second.terms...)
	for _, j := range indices {
		arg := terms[j].expression
		if j == minIndex {
			terms[j].expression = falsehood
		} else {
			terms[j].expression = truth
		}
		if err = mc.SetValue(arg, j != minIndex); err != nil {
			return false, err
		}
	}
	tmp1, err := t.makeGE(terms, second.bound)
	if err != nil {
		return false, err
	}
	pin, err := m.NewRef(tmp1)
	if err != nil {
		return false, err
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, errors.Join(err, m.DelRef(&pin))
	}
	result, err := func() (bool, error) {
		if err := rw.Rewrite(tmp1, out); err != nil {
			return false, err
		}
		tmp2, err := m.RefGet(out)
		if err != nil {
			return false, err
		}
		ge, err := m.IsPseudoBooleanGE(tmp2)
		if err != nil {
			return false, err
		}
		if ge {
			k, err := t.rational(tmp2, nil)
			if err != nil {
				return false, err
			}
			if k.Cmp(big.NewRat(1, 1)) == 0 {
				n, err := m.GetNumArgs(tmp2)
				if err != nil {
					return false, err
				}
				args := make([]ast.ASTHandle, n)
				for i := range args {
					if args[i], err = m.GetArg(tmp2, uint32(i)); err != nil {
						return false, err
					}
				}
				if tmp2, err = m.MkOrArgs(args); err != nil {
					return false, err
				}
			}
		}
		d1, err := g.Dependency(idx1)
		if err != nil {
			return false, err
		}
		d2, err := g.Dependency(idx2)
		if err != nil {
			return false, err
		}
		joined, err := m.DependencyJoin(d1, d2)
		if err != nil {
			return false, err
		}
		if err = g.Update(idx2, tmp2, ast.ASTHandle{}, joined); err != nil {
			return false, err
		}
		if err = g.Update(idx1, truth, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return false, err
		}
		return true, nil
	}()
	return result, errors.Join(err, m.DelRef(&out), m.DelRef(&pin))
}
