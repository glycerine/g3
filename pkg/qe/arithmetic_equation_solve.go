package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util::solve,
// extract_equalities, normalize_sum (mul_lt), isolate_x, reduce_equations,
// reduce_equation, solve_singular, solve_linear, find_variable and
// is_linear. Eliminations are published through the i_solver_context
// (elim_var, add_var).

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

const (
	arithOpAdd   int32 = 6
	arithOpMul   int32 = 9
	arithSortInt int32 = 1
)

// solve is arith_qe_util::solve.
func (p *ArithmeticPlugin) solve(conjs *ConjunctionEnum, formula ast.ASTHandle) (solved bool, err error) {
	if conjs == nil || conjs.manager != p.manager {
		return false, ast.ErrASTHandle
	}
	if p.util.theory == nil || p.util.arith == nil {
		return false, ast.ErrASTHandle
	}
	var eqs []ast.ASTRef
	defer func() {
		// expr_ref_vector destruction releases front to back.
		for i := range eqs {
			err = errors.Join(err, p.manager.DelRef(&eqs[i]))
		}
	}()
	if err = p.extractEqualities(conjs, &eqs); err != nil {
		return false, err
	}
	return p.reduceEquations(eqs, formula)
}

// simplify is arith_qe_util::simplify: m_rewriter(p).
func (p *ArithmeticPlugin) simplify(ref ast.ASTRef) error {
	h, err := p.manager.RefGet(ref)
	if err != nil {
		return err
	}
	return p.util.theory.Rewrite(h, ref)
}

// assignOwned stores an owned result into dst (expr_ref assignment) and
// releases the result's own reference.
func (p *ArithmeticPlugin) assignOwned(dst ast.ASTRef, owned ast.ASTRef) error {
	h, err := p.manager.RefGet(owned)
	if err == nil {
		err = p.manager.SetRef(dst, h)
	}
	return errors.Join(err, p.manager.DelRef(&owned))
}

// extractEqualities is arith_qe_util::extract_equalities.
func (p *ArithmeticPlugin) extractEqualities(conjs *ConjunctionEnum, eqs *[]ast.ASTRef) (err error) {
	m := p.manager
	leqs := make(map[ast.ASTHandle]bool)
	var trail []ast.ASTRef
	tmp1, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	tmp2, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return errors.Join(err, m.DelRef(&tmp1))
	}
	defer func() {
		err = errors.Join(err, m.DelRef(&tmp2), m.DelRef(&tmp1))
		for i := range trail {
			err = errors.Join(err, m.DelRef(&trail[i]))
		}
	}()
	count, err := conjs.Len()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		e, err := conjs.At(i)
		if err != nil {
			return err
		}
		kind, a0, a1, matched, err := conjs.arithmeticRelation(e)
		if err != nil {
			return err
		}
		if !matched {
			// drop equality.
			continue
		}
		sub, err := p.util.arith.MkSub2(a0, a1)
		if err != nil {
			return err
		}
		if err = p.assignOwned(tmp1, sub); err != nil {
			return err
		}
		if kind == qeConjunctionEquality {
			if err = p.simplify(tmp1); err != nil {
				return err
			}
			if err = appendRefOf(m, eqs, tmp1); err != nil {
				return err
			}
			continue
		}
		if err = p.normalizeSum(tmp1); err != nil {
			return err
		}
		h1, err := m.RefGet(tmp1)
		if err != nil {
			return err
		}
		neg, err := m.MkArithmeticNeg(h1)
		if err != nil {
			return err
		}
		if err = m.SetRef(tmp2, neg); err != nil {
			return err
		}
		if err = p.normalizeSum(tmp2); err != nil {
			return err
		}
		h2, err := m.RefGet(tmp2)
		if err != nil {
			return err
		}
		if leqs[h2] {
			if err = appendRefOf(m, eqs, tmp1); err != nil {
				return err
			}
		} else {
			if err = appendRefOf(m, &trail, tmp1); err != nil {
				return err
			}
			leqs[h1] = true
		}
	}
	return nil
}

func appendRefOf(m *ast.ASTManager, out *[]ast.ASTRef, src ast.ASTRef) error {
	h, err := m.RefGet(src)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(h)
	if err != nil {
		return err
	}
	*out = append(*out, ref)
	return nil
}

// arithApp matches an application of the arithmetic family with kind.
func (p *ArithmeticPlugin) arithApp(e ast.ASTHandle, kind int32) ([]ast.ASTHandle, bool, error) {
	k, err := p.manager.GetKind(e)
	if err != nil || k != ast.ASTApp {
		return nil, false, err
	}
	return p.util.operation(e, p.family, kind)
}

// binaryMul is arith_util::is_mul(n, a1, a2).
func (p *ArithmeticPlugin) binaryMul(e ast.ASTHandle) (a1, a2 ast.ASTHandle, ok bool, err error) {
	args, matched, err := p.arithApp(e, arithOpMul)
	if err != nil || !matched || len(args) != 2 {
		return a1, a2, false, err
	}
	return args[0], args[1], true, nil
}

// mulLtKey is mul_lt's comparison key: a numeral-scaled product compares by
// its second factor.
func (p *ArithmeticPlugin) mulLtKey(n ast.ASTHandle) (uint32, error) {
	x, y, ok, err := p.binaryMul(n)
	if err != nil {
		return 0, err
	}
	if ok {
		if _, numeral, e := p.util.numeral(x); e != nil {
			return 0, e
		} else if numeral {
			n = y
		}
	}
	return p.manager.GetASTID(n)
}

// normalizeSum is arith_qe_util::normalize_sum.
func (p *ArithmeticPlugin) normalizeSum(ref ast.ASTRef) error {
	if err := p.simplify(ref); err != nil {
		return err
	}
	h, err := p.manager.RefGet(ref)
	if err != nil {
		return err
	}
	args, isAdd, err := p.arithApp(h, arithOpAdd)
	if err != nil || !isAdd {
		return err
	}
	keys := make([]uint32, len(args))
	for i, a := range args {
		if keys[i], err = p.mulLtKey(a); err != nil {
			return err
		}
	}
	order := make([]int, len(args))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return keys[order[i]] < keys[order[j]] })
	sorted := make([]ast.ASTHandle, len(args))
	for i, o := range order {
		sorted[i] = args[o]
	}
	sum, err := p.mkAdd(sorted)
	if err != nil {
		return err
	}
	return p.manager.SetRef(ref, sum)
}

// mkAdd is arith_util::mk_add(num, args).
func (p *ArithmeticPlugin) mkAdd(args []ast.ASTHandle) (ast.ASTHandle, error) {
	if len(args) == 1 {
		return args[0], nil
	}
	return p.manager.MkFamilyApp(p.family, arithOpAdd, nil, args)
}

// reduceEquations is arith_qe_util::reduce_equations.
func (p *ArithmeticPlugin) reduceEquations(eqs []ast.ASTRef, formula ast.ASTHandle) (bool, error) {
	for _, ref := range eqs {
		eq, err := p.manager.RefGet(ref)
		if err != nil {
			return false, err
		}
		reduced, err := p.reduceEquation(eq, formula)
		if err != nil || reduced {
			return reduced, err
		}
	}
	return false, nil
}

// reduceEquation is arith_qe_util::reduce_equation.
func (p *ArithmeticPlugin) reduceEquation(eq, formula ast.ASTHandle) (bool, error) {
	if k, numeral, err := p.util.numeral(eq); err != nil {
		return false, err
	} else if numeral && k.Sign() == 0 {
		return false, nil
	}
	solved, err := p.solveSingular(eq, formula)
	if err != nil || solved {
		return solved, err
	}
	return p.solveLinear(eq, formula)
}

// solveSingular is arith_qe_util::solve_singular(p, fml).
func (p *ArithmeticPlugin) solveSingular(eq, formula ast.ASTHandle) (bool, error) {
	n, err := p.context.NumVariables()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < n; i++ {
		solved, err := p.solveSingularAt(i, eq, formula)
		if err != nil || solved {
			return solved, err
		}
	}
	return false, nil
}

// isolateX is arith_qe_util::isolate_x.
func (p *ArithmeticPlugin) isolateX(poly, x ast.ASTHandle, contains *ast.ContainsApp) (*big.Rat, bool, error) {
	for {
		args, isAdd, err := p.arithApp(poly, arithOpAdd)
		if err != nil {
			return nil, false, err
		}
		if !isAdd {
			break
		}
		found := false
		var next ast.ASTHandle
		for _, arg := range args {
			has, err := contains.Check(arg)
			if err != nil {
				return nil, false, err
			}
			if has {
				if found {
					return nil, false, nil
				}
				found = true
				next = arg
			}
		}
		if next.IsNull() {
			return nil, false, nil
		}
		poly = next
	}
	if poly == x {
		return big.NewRat(1, 1), true, nil
	}
	e1, e2, ok, err := p.binaryMul(poly)
	if err != nil || !ok {
		return nil, false, err
	}
	if k, numeral, err := p.util.numeral(e1); err != nil {
		return nil, false, err
	} else if numeral && e2 == x {
		return k, true, nil
	}
	if k, numeral, err := p.util.numeral(e2); err != nil {
		return nil, false, err
	} else if numeral && e1 == x {
		return k, true, nil
	}
	return nil, false, nil
}

// solveSingularAt is arith_qe_util::solve_singular(var_idx, p, fml).
func (p *ArithmeticPlugin) solveSingularAt(index uint32, poly, formula ast.ASTHandle) (solved bool, err error) {
	m := p.manager
	x, err := p.context.Variable(index)
	if err != nil {
		return false, err
	}
	contains, err := p.context.Contains(index)
	if err != nil {
		return false, err
	}
	k, isolated, err := p.isolateX(poly, x, contains)
	if err != nil || !isolated {
		return false, err
	}
	integer, err := p.util.isInteger(x)
	if err != nil {
		return false, err
	}
	unit := new(big.Rat).Abs(k).Cmp(big.NewRat(1, 1)) == 0
	if integer && !unit {
		return false, nil
	}
	e, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&e)) }()
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	var value ast.ASTHandle
	if unit {
		if k.Sign() < 0 {
			value, err = m.MkArithmeticAdd(poly, x)
		} else {
			value, err = m.MkArithmeticSub(x, poly)
		}
	} else {
		//    p = p' + k*x = 0  =>  x = (p - k*x)/ -k
		ke, cause := m.ArithmeticNumeralRat(new(big.Rat).Neg(k), false)
		var t ast.ASTHandle
		if cause == nil {
			t, cause = m.MkArithmeticMul(ke, x)
		}
		if cause == nil {
			cause = m.SetRef(tmp, t)
		}
		if cause == nil {
			t, cause = m.MkArithmeticAdd(poly, t)
		}
		if cause == nil {
			cause = m.SetRef(tmp, t)
		}
		if cause == nil {
			value, cause = m.MkArithmeticDiv(t, ke)
		}
		err = cause
	}
	if err == nil {
		err = m.SetRef(e, value)
	}
	if err != nil {
		return false, err
	}
	if err = p.substituteAndEliminate(index, x, value, formula); err != nil {
		return false, err
	}
	return true, nil
}

// substituteAndEliminate is the shared tail of solve_singular and
// solve_linear: result = fml[x := def]; simplify(result);
// m_ctx.elim_var(idx, result, def).
func (p *ArithmeticPlugin) substituteAndEliminate(index uint32, x, definition, formula ast.ASTHandle) (err error) {
	m := p.manager
	result, err := m.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&result)) }()
	if err = p.replacer.ApplySubstitution(x, definition, result); err != nil {
		return err
	}
	if err = p.simplify(result); err != nil {
		return err
	}
	h, err := m.RefGet(result)
	if err != nil {
		return err
	}
	return p.context.EliminateVariable(index, h, definition)
}

// findVariable is arith_qe_util::find_variable.
func (p *ArithmeticPlugin) findVariable(e ast.ASTHandle, vars []ast.ASTHandle, values []*big.Rat, k *big.Rat) (bool, error) {
	kind, err := p.manager.GetKind(e)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	n, err := p.manager.GetNumArgs(e)
	if err != nil || n > 0 {
		return false, err
	}
	for i, v := range vars {
		if e == v {
			values[i].Add(values[i], k)
			return true, nil
		}
	}
	return false, nil
}

// isLinear is arith_qe_util::is_linear. The numeral k is shared by every
// match attempt, as in the source.
func (p *ArithmeticPlugin) isLinear(poly ast.ASTHandle, vars []ast.ASTHandle) ([]*big.Rat, bool, error) {
	if len(vars) == 0 {
		return nil, false, nil
	}
	values := make([]*big.Rat, len(vars)+1)
	for i := range values {
		values[i] = new(big.Rat)
	}
	k := new(big.Rat)
	numeral := func(e ast.ASTHandle) (bool, error) {
		v, ok, err := p.util.numeral(e)
		if err == nil && ok {
			k.Set(v)
		}
		return ok, err
	}
	todo := []ast.ASTHandle{poly}
	for len(todo) > 0 {
		e := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		args, isAdd, err := p.arithApp(e, arithOpAdd)
		if err != nil {
			return nil, false, err
		}
		if isAdd {
			todo = append(todo, args...)
			continue
		}
		e1, e2, isMul, err := p.binaryMul(e)
		if err != nil {
			return nil, false, err
		}
		if isMul {
			ok, err := numeral(e1)
			if err == nil && ok {
				ok, err = p.findVariable(e2, vars, values[1:], k)
			}
			if err != nil {
				return nil, false, err
			}
			if ok {
				continue
			}
			ok, err = numeral(e2)
			if err == nil && ok {
				ok, err = p.findVariable(e1, vars, values[1:], k)
			}
			if err != nil {
				return nil, false, err
			}
			if ok {
				continue
			}
		}
		ok, err := p.findVariable(e, vars, values[1:], k)
		if err != nil {
			return nil, false, err
		}
		if ok {
			continue
		}
		ok, err = numeral(e)
		if err != nil {
			return nil, false, err
		}
		if ok {
			values[0].Add(values[0], k)
			continue
		}
		return nil, false, nil
	}
	return values, true, nil
}

// solveLinear is arith_qe_util::solve_linear.
func (p *ArithmeticPlugin) solveLinear(poly, formula ast.ASTHandle) (solved bool, err error) {
	m := p.manager
	n, err := p.context.NumVariables()
	if err != nil {
		return false, err
	}
	vars := make([]ast.ASTHandle, n)
	for i := range vars {
		if vars[i], err = p.context.Variable(uint32(i)); err != nil {
			return false, err
		}
	}
	values, linear, err := p.isLinear(poly, vars)
	if err != nil || !linear {
		return false, err
	}
	row := make([]*big.Int, len(values))
	for i, v := range values {
		if !v.IsInt() {
			return false, nil
		}
		row[i] = new(big.Int).Set(v.Num())
	}
	// VERIFY(m_arith_solver.solve_integer_equation(values, index, is_aux)).
	index, isAux, satisfiable, err := solveArithmeticIntegerRow(row)
	if err != nil || !satisfiable {
		return false, err
	}
	if index < 1 || index > len(vars) {
		return false, ast.ErrASTDomain
	}
	x, err := m.NewRef(vars[index-1])
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&x)) }()
	z, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&z)) }()
	p1, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&p1)) }()
	intNumeral := func(v *big.Int) (ast.ASTHandle, error) {
		return m.ArithmeticNumeralRat(new(big.Rat).SetInt(v), true)
	}
	var acc ast.ASTHandle
	if isAux {
		// An auxiliary variable was introduced in lieu of 'x'.
		intSort, e := m.MkFamilySort(p.family, arithSortInt, nil)
		var fresh ast.ASTHandle
		if e == nil {
			fresh, e = m.MkFreshConst("x", intSort)
		}
		if e == nil {
			e = m.SetRef(z, fresh)
		}
		if e == nil {
			e = p.context.AddVariable(fresh)
		}
		var coeff ast.ASTHandle
		if e == nil {
			coeff, e = intNumeral(row[index])
		}
		if e == nil {
			acc, e = m.MkArithmeticMul(coeff, fresh)
		}
		if e != nil {
			return false, e
		}
	} else if acc, err = intNumeral(new(big.Int)); err != nil {
		return false, err
	}
	if err = m.SetRef(p1, acc); err != nil {
		return false, err
	}
	for i := 1; i <= len(vars); i++ {
		if row[i].Sign() == 0 || i == index {
			continue
		}
		coeff, e := intNumeral(row[i])
		var term ast.ASTHandle
		if e == nil {
			term, e = m.MkArithmeticMul(coeff, vars[i-1])
		}
		if e == nil {
			acc, e = m.MkArithmeticAdd(acc, term)
		}
		if e == nil {
			e = m.SetRef(p1, acc)
		}
		if e != nil {
			return false, e
		}
	}
	constant, err := intNumeral(row[0])
	if err == nil {
		acc, err = m.MkArithmeticAdd(acc, constant)
	}
	if err == nil {
		err = m.SetRef(p1, acc)
	}
	if err != nil {
		return false, err
	}
	if err = p.substituteAndEliminate(uint32(index-1), vars[index-1], acc, formula); err != nil {
		return false, err
	}
	return true, nil
}
