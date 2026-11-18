package qe

// Source: src/qe/qe_arith.cpp imp::operator() and live-row reconstruction.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (p *ArithmeticProjection) Project(md *model.ModelCore, vars, forms *[]ast.ASTRef) (err error) {
	c, err := p.begin(md)
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if vars == nil || forms == nil {
		return ast.ErrASTHandle
	}
	hasArithmetic := false
	for _, r := range *vars {
		h, e := p.manager.RefGet(r)
		if e != nil {
			return e
		}
		if _, e = p.manager.GetDecl(h); e != nil {
			return e
		}
		_, a, e := p.arithmetic(h)
		if e != nil {
			return e
		}
		hasArithmetic = hasArithmetic || a
	}
	if !hasArithmetic {
		return nil
	}
	if err = c.startEvaluator(md); err != nil {
		return err
	}
	return c.project(vars, forms)
}

// ProjectVariable deliberately leaves the additional-variable vector alone:
// the source arithmetic plugin projects a private singleton and adds none.
func (p *ArithmeticProjection) ProjectVariable(md *model.ModelCore, variable ast.ASTHandle, additional, forms *[]ast.ASTRef) (eliminated bool, err error) {
	if additional == nil || forms == nil {
		return false, ast.ErrASTHandle
	}
	c, err := p.begin(md)
	if err != nil {
		return false, err
	}
	defer p.finish(c, &err)
	if _, err = p.manager.GetDecl(variable); err != nil {
		return false, err
	}
	// Retain the actual mutable singleton vector through interrupted cleanup.
	local := new([]ast.ASTRef)
	c.vectors = append(c.vectors, local)
	if err = p.appendRef(local, variable); err != nil {
		return false, err
	}
	_, arith, err := p.arithmetic(variable)
	if err != nil {
		return false, err
	}
	if !arith {
		return false, nil
	}
	if err = c.startEvaluator(md); err != nil {
		return false, err
	}
	err = c.project(local, forms)
	return len(*local) == 0, err
}

func (c *arithmeticProjectionCall) project(vars, forms *[]ast.ASTRef) (err error) {
	p := c.owner
	m := p.manager
	j := 0
	// ITE linearization appends branch literals. Source visits these new
	// elements too before compacting the residue vector.
	for i := 0; i < len(*forms); i++ {
		h, err := m.RefGet((*forms)[i])
		if err != nil {
			return err
		}
		ok, err := c.linearizeLiteral(h, forms)
		if err != nil {
			return err
		}
		if !ok {
			if i != j {
				if err = m.SetRef((*forms)[j], h); err != nil {
					return err
				}
			}
			j++
		} else {
			pin, err := m.NewRef(h)
			if err != nil {
				return err
			}
			c.pinned = append(c.pinned, pin)
			if err = p.emit("operator()", 311, func(w io.Writer) error {
				if e := writeExpression(m, p.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); err != nil {
				return err
			}
		}
	}
	if err := p.shrink(forms, j); err != nil {
		return err
	}
	variableSet := make(map[ast.ASTHandle]bool)
	c.mark, err = ast.NewASTExprMark(m)
	if err != nil {
		return err
	}
	for _, r := range *vars {
		h, err := m.RefGet(r)
		if err != nil {
			return err
		}
		variableSet[h] = true
		_, arith, err := p.arithmetic(h)
		if err != nil {
			return err
		}
		if _, ok := c.ids.Get2(h); arith && !ok {
			if _, err = c.idTrace(h, func(value ast.ASTHandle) error {
				return p.emit("operator()", 332, func(w io.Writer) error {
					if e := writeExpression(m, p.config, w, h); e != nil {
						return e
					}
					_, _ = io.WriteString(w, " ")
					if e := writeExpression(m, p.config, w, value); e != nil {
						return e
					}
					_, _ = io.WriteString(w, "\n")
					return nil
				})
			}); err != nil {
				return err
			}
		}
	}
	// Preserve source's root-only residue marking. Recursive protection is
	// supplied by the nonprojected arithmetic terms registered below.
	for _, r := range *forms {
		h, err := m.RefGet(r)
		if err != nil {
			return err
		}
		if err = c.mark.Mark(h, true); err != nil {
			return err
		}
	}
	index := make([]ast.ASTHandle, c.ids.Len())
	for h, id := range c.ids.All() {
		index[id] = h
		if !variableSet[h] {
			// This existing typed visitor marks every node and includes
			// quantifier annotations, matching source mark_rec's overload.
			if _, err := m.GetNumExprsWithExprMark(h, c.mark); err != nil {
				return err
			}
		}
	}
	j = 0
	var selected []uint32
	for i, r := range *vars {
		h, err := m.RefGet(r)
		if err != nil {
			return err
		}
		_, arith, err := p.arithmetic(h)
		if err != nil {
			return err
		}
		marked, e := c.mark.IsMarked(h)
		if e != nil {
			return e
		}
		if arith && !marked {
			id, ok := c.ids.Get2(h)
			if !ok {
				return ast.ErrASTDomain
			}
			selected = append(selected, id)
		} else {
			if i != j {
				if err = m.SetRef((*vars)[j], h); err != nil {
					return err
				}
			}
			j++
		}
	}
	if err := p.shrink(vars, j); err != nil {
		return err
	}
	if err := p.emit("operator()", 362, func(w io.Writer) error {
		_, _ = io.WriteString(w, "remaining vars: ")
		if e := writeExpressionVector(m, p.config, w, *vars); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		for _, id := range selected {
			_, _ = fmt.Fprintf(w, "v%d ", id)
			if e := writeExpression(m, p.config, w, index[id]); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
		}
		return c.mbo.Display(w)
	}); err != nil {
		return err
	}
	if err := c.mbo.Project(selected...); err != nil {
		return err
	}
	if err := p.emit("operator()", 368, func(w io.Writer) error { return c.mbo.Display(w) }); err != nil {
		return err
	}
	// vector<row> rows; mbo.get_live_rows(rows): copies, destroyed when
	// operator() returns.
	rows, err := c.mbo.LiveRows()
	defer func() { err = errors.Join(err, simplex.DelRows(p.q(), rows)) }()
	if err != nil {
		return err
	}
	for i := range rows {
		if len(rows[i].Terms) == 0 {
			continue
		}
		if err = c.projectRow(forms, &rows[i], index); err != nil {
			return err
		}
	}
	return nil
}

// projectRow is one iteration of the live-row loop: expr_ref_vector ts;
// expr_ref t, s, val, all released at the end of the iteration (val, s, t,
// then ts front to back).
func (c *arithmeticProjectionCall) projectRow(forms *[]ast.ASTRef, row *simplex.OptRow, index []ast.ASTHandle) (err error) {
	p := c.owner
	m := p.manager
	var ts []ast.ASTRef
	var t, s, val ast.ASTRef
	defer func() {
		for _, r := range []*ast.ASTRef{&val, &s, &t} {
			if *r != (ast.ASTRef{}) {
				err = errors.Join(err, m.DelRef(r))
			}
		}
		for i := range ts {
			err = errors.Join(err, m.DelRef(&ts[i]))
		}
	}()
	if t, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return err
	}
	if s, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return err
	}
	if val, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return err
	}
	h, err := c.rowExpression(row, index, &ts, t, s)
	if err != nil {
		return err
	}
	if err = p.appendRef(forms, h); err != nil {
		return err
	}
	if err = c.evaluator.Evaluate(h, val); err != nil {
		return err
	}
	value, err := m.RefGet(val)
	if err != nil {
		return err
	}
	if !m.IsTrue(value) {
		line, label := 427, "Evaluated "
		unitNegative, e := c.unitNegative(row)
		if e != nil {
			return e
		}
		if unitNegative {
			line, label = 394, "Evaluated unit "
		}
		if err = p.emit("operator()", line, func(w io.Writer) error {
			_, _ = io.WriteString(w, label)
			if e := writeExpression(m, p.config, w, h); e != nil {
				return e
			}
			_, _ = io.WriteString(w, " to ")
			if e := writeExpression(m, p.config, w, value); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

// rowExpression builds the row's formula into t, holding the summands in
// ts and the bound in s as the source's locals do.
func (c *arithmeticProjectionCall) rowExpression(row *simplex.OptRow, index []ast.ASTHandle, ts *[]ast.ASTRef, t, s ast.ASTRef) (ast.ASTHandle, error) {
	p := c.owner
	m := p.manager
	into := func(r ast.ASTRef) func(ast.ASTHandle, error) (ast.ASTHandle, error) {
		return func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
			if err != nil {
				return ast.ASTHandle{}, err
			}
			return h, m.SetRef(r, h)
		}
	}
	unitNegative, err := c.unitNegative(row)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	qm := p.q()
	var last ast.ASTHandle
	for _, v := range row.Terms {
		if int(v.Variable) >= len(index) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		h, err := into(t)(index[v.Variable], nil)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		h, err = c.termExpression(qm, &v.Coefficient, unitNegative, h, into(t))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		last = h
		if !unitNegative {
			r, err := m.NewRef(h)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			*ts = append(*ts, r)
		}
	}
	integer, _, err := p.arithmetic(last)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	// rational r = unit ? rows[i].m_coeff : -rows[i].m_coeff
	q := unumeral.NewRational()
	err = qm.Copy(&q, &row.Constant)
	if err == nil && !unitNegative {
		err = qm.Neg(&q)
	}
	var rhs ast.ASTHandle
	if err == nil {
		rhs, err = into(s)(m.ArithmeticNumeralFromRational(&q, integer))
	}
	if err = errors.Join(err, qm.Del(&q)); err != nil {
		return ast.ASTHandle{}, err
	}
	lhs := last
	if !unitNegative {
		if len(*ts) == 1 {
			lhs, err = m.RefGet((*ts)[0])
			if err == nil {
				err = m.SetRef(t, lhs)
			}
		} else {
			terms := make([]ast.ASTHandle, len(*ts))
			for i, r := range *ts {
				if terms[i], err = m.RefGet(r); err != nil {
					return ast.ASTHandle{}, err
				}
			}
			lhs, err = into(t)(m.MkFamilyApp(p.family, 6, nil, terms))
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if row.Relation == simplex.Equal {
		return into(t)(m.MkEq(lhs, rhs))
	}
	if row.Relation == simplex.Divides {
		constant, err := qm.Sign(&row.Constant)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if constant != 0 {
			if lhs, err = into(t)(m.MkArithmeticSub(lhs, rhs)); err != nil {
				return ast.ASTHandle{}, err
			}
		}
		// a.mk_eq(a.mk_mod(t, a.mk_numeral(r.m_mod, true)), a.mk_int(0)):
		// GCC evaluates the arguments right to left, so the 0 is created
		// first, then the modulus, then the mod (AUDIT.md qe_01-F7).
		zero, err := m.ArithmeticIntegerNumeralText("0")
		if err != nil {
			return ast.ASTHandle{}, err
		}
		modulus, err := m.ArithmeticNumeralFromRational(&row.Modulus, true)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		mod, err := m.MkFamilyApp(p.family, 13, nil, []ast.ASTHandle{lhs, modulus})
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return into(t)(m.MkEq(mod, zero))
	}
	op := int32(2)
	if row.Relation == simplex.Less {
		op = 4
	}
	if unitNegative {
		op++
	}
	return into(t)(m.MkFamilyApp(p.family, op, nil, []ast.ASTHandle{lhs, rhs}))
}

// unitNegative is the source's row test r.m_vars.size() == 1 &&
// r.m_vars[0].m_coeff.is_neg() && r.m_type != opt::t_mod.
func (c *arithmeticProjectionCall) unitNegative(row *simplex.OptRow) (bool, error) {
	if len(row.Terms) != 1 || row.Relation == simplex.Divides {
		return false, nil
	}
	sign, err := c.owner.q().Sign(&row.Terms[0].Coefficient)
	return sign < 0, err
}

// termExpression is one summand of the row: rational r = unit ? -coeff :
// coeff (a temporary of the iteration), and t = r * x unless r is one.
func (c *arithmeticProjectionCall) termExpression(qm *unumeral.RationalManager, coefficient *unumeral.Rational, unitNegative bool, h ast.ASTHandle, into func(ast.ASTHandle, error) (ast.ASTHandle, error)) (result ast.ASTHandle, err error) {
	p := c.owner
	m := p.manager
	r := unumeral.NewRational()
	defer func() { err = errors.Join(err, qm.Del(&r)) }()
	if err = qm.Copy(&r, coefficient); err != nil {
		return ast.ASTHandle{}, err
	}
	if unitNegative {
		if err = qm.Neg(&r); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	one, err := qm.IsOne(&r)
	if err != nil || one {
		return h, err
	}
	integer, _, err := p.arithmetic(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	coefficientNumeral, err := m.ArithmeticNumeralFromRational(&r, integer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return into(m.MkArithmeticMul(coefficientNumeral, h))
}
