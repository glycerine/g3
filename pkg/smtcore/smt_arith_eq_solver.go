package smtcore

// Source: src/smt/arith_eq_solver.cpp and src/smt/arith_eq_solver.h retained
// integer-row operations used by the full arithmetic theory.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtArithmeticEquationSolver mutates integer coefficient rows (row, a
// vector<rational>) in source order. Column zero is the constant and columns
// one onward are variables. Rows are rationals of q; the caller owns them.
type smtArithmeticEquationSolver struct {
	q *numeral.RationalManager
}

// smtEqSolverScratch holds the solver's local rationals; del frees them.
type smtEqSolverScratch struct {
	q    *numeral.RationalManager
	vals []numeral.Rational
	err  error
}

func (s *smtEqSolverScratch) keep(err error) {
	if err != nil && s.err == nil {
		s.err = err
	}
}

func (s *smtEqSolverScratch) del(rs ...*numeral.Rational) {
	for _, r := range rs {
		s.keep(smtQDel(s.q, r))
	}
}

func smtIntegerRowValid(row []numeral.Rational) bool { return len(row) >= 2 }

// gcdNormalize is gcd_normalize.
func (s *smtArithmeticEquationSolver) gcdNormalize(row []numeral.Rational) (err error) {
	q := s.q
	g, a := smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&g, &a} {
			if e := smtQDel(q, r); e != nil && err == nil {
				err = e
			}
		}
	}()
	for i := 0; i < len(row); i++ {
		if one, e := q.IsOne(&g); e != nil || one {
			if e != nil {
				return e
			}
			break
		}
		zero, e := q.IsZero(&row[i])
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		if e = q.Copy(&a, &row[i]); e != nil {
			return e
		}
		if e = q.Abs(&a); e != nil {
			return e
		}
		gz, e := q.IsZero(&g)
		if e != nil {
			return e
		}
		if gz {
			e = q.Copy(&g, &a)
		} else {
			e = q.GCD(&a, &g, &g)
		}
		if e != nil {
			return e
		}
	}
	gz, e := q.IsZero(&g)
	if e != nil {
		return e
	}
	gOne, e := q.IsOne(&g)
	if e != nil || gz || gOne {
		return e
	}
	for i := range row {
		if e = q.Div(&row[i], &g, &row[i]); e != nil {
			return e
		}
	}
	return nil
}

// gcdTest is gcd_test: the constant is divisible by the gcd of the
// variable coefficients.
func (s *smtArithmeticEquationSolver) gcdTest(row []numeral.Rational) (result bool, err error) {
	if len(row) == 0 {
		return false, ast.ErrASTDomain
	}
	q := s.q
	g, a := smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&g, &a} {
			if e := smtQDel(q, r); e != nil && err == nil {
				err = e
			}
		}
	}()
	for i := 1; i < len(row); i++ {
		if one, e := q.IsOne(&g); e != nil || one {
			if e != nil {
				return false, e
			}
			break
		}
		zero, e := q.IsZero(&row[i])
		if e != nil {
			return false, e
		}
		if zero {
			continue
		}
		if e = q.Copy(&a, &row[i]); e != nil {
			return false, e
		}
		if e = q.Abs(&a); e != nil {
			return false, e
		}
		gz, e := q.IsZero(&g)
		if e != nil {
			return false, e
		}
		if gz {
			e = q.Copy(&g, &a)
		} else {
			e = q.GCD(&a, &g, &g)
		}
		if e != nil {
			return false, e
		}
	}
	if one, e := q.IsOne(&g); e != nil || one {
		return one, e
	}
	if gz, e := q.IsZero(&g); e != nil || gz {
		if e != nil {
			return false, e
		}
		return q.IsZero(&row[0])
	}
	if e := q.Div(&row[0], &g, &a); e != nil {
		return false, e
	}
	return q.IsInt(&a)
}

// findAbsMin is find_abs_min.
func (s *smtArithmeticEquationSolver) findAbsMin(row []numeral.Rational) (index int, err error) {
	q := s.q
	v, w := smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&v, &w} {
			if e := smtQDel(q, r); e != nil && err == nil {
				err = e
			}
		}
	}()
	for i := 1; i < len(row); i++ {
		if err = q.Copy(&w, &row[i]); err != nil {
			return 0, err
		}
		if err = q.Abs(&w); err != nil {
			return 0, err
		}
		vz, e := q.IsZero(&v)
		if e != nil {
			return 0, e
		}
		take := vz
		if !take {
			wz, e := q.IsZero(&w)
			if e != nil {
				return 0, e
			}
			c, e := q.Compare(&w, &v)
			if e != nil {
				return 0, e
			}
			take = !wz && c < 0
		}
		if take {
			index = i
			if err = q.Copy(&v, &w); err != nil {
				return 0, err
			}
		}
	}
	return index, nil
}

func (s *smtArithmeticEquationSolver) negRow(row []numeral.Rational) error {
	for i := range row {
		if e := s.q.Neg(&row[i]); e != nil {
			return e
		}
	}
	return nil
}

// modHat is mod_hat(a, b): mod(a, b), less b when twice it exceeds b.
func (s *smtArithmeticEquationSolver) modHat(a, b *numeral.Rational) (err error) {
	q := s.q
	r2 := smtQZero()
	defer func() {
		if e := smtQDel(q, &r2); e != nil && err == nil {
			err = e
		}
	}()
	if err = q.Mod(a, b, a); err != nil {
		return err
	}
	if err = q.Add(a, a, &r2); err != nil {
		return err
	}
	c, err := q.Compare(b, &r2)
	if err != nil || c >= 0 {
		return err
	}
	return q.Sub(a, b, a)
}

// SolveEquation is solve_integer_equation, one Omega equality-elimination
// step. On success, index identifies the solved variable; fresh reports
// whether its replacement uses a new auxiliary integer variable.
func (s *smtArithmeticEquationSolver) SolveEquation(row []numeral.Rational) (index int, fresh bool, satisfiable bool, err error) {
	if !smtIntegerRowValid(row) {
		return 0, false, false, ast.ErrASTDomain
	}
	q := s.q
	if err = s.gcdNormalize(row); err != nil {
		return 0, false, false, err
	}
	if satisfiable, err = s.gcdTest(row); err != nil || !satisfiable {
		return 0, false, satisfiable, err
	}
	if index, err = s.findAbsMin(row); err != nil {
		return 0, false, false, err
	}
	absA := smtQZero()
	defer func() {
		if e := smtQDel(q, &absA); e != nil && err == nil {
			err = e
		}
	}()
	if err = q.Copy(&absA, &row[index]); err != nil {
		return 0, false, false, err
	}
	aOne, err := q.IsOne(&absA)
	if err != nil {
		return 0, false, false, err
	}
	if err = q.Abs(&absA); err != nil {
		return 0, false, false, err
	}
	if zero, e := q.IsZero(&absA); e != nil || zero {
		return index, false, e == nil, e
	}
	if aOne {
		if err = s.negRow(row); err != nil {
			return 0, false, false, err
		}
	}
	absOne, err := q.IsOne(&absA)
	if err != nil {
		return 0, false, false, err
	}
	fresh = !absOne
	if !fresh {
		return index, false, true, nil
	}
	// m = abs_a + 1, held in absA.
	if err = q.Inc(&absA); err != nil {
		return 0, false, false, err
	}
	for i := range row {
		if err = s.modHat(&row[i], &absA); err != nil {
			return 0, false, false, err
		}
	}
	if one, e := q.IsOne(&row[index]); e != nil || one {
		if e != nil {
			return 0, false, false, e
		}
		if err = s.negRow(row); err != nil {
			return 0, false, false, err
		}
	}
	if minusOne, e := q.IsMinusOne(&row[index]); e != nil || !minusOne {
		if e == nil {
			e = ast.ErrASTDomain
		}
		return 0, false, false, e
	}
	if err = q.Copy(&row[index], &absA); err != nil {
		return 0, false, false, err
	}
	return index, true, true, nil
}

// substitute is substitute(r, s, index): r may grow to the size of solved.
func (s *smtArithmeticEquationSolver) substitute(row, solved []numeral.Rational, index int) (_ []numeral.Rational, err error) {
	if !smtIntegerRowValid(solved) || index < 1 || index >= len(solved) {
		return nil, ast.ErrASTDomain
	}
	if index >= len(row) {
		return row, nil
	}
	q := s.q
	c, t := smtQZero(), smtQZero()
	defer func() {
		for _, r := range []*numeral.Rational{&c, &t} {
			if e := smtQDel(q, r); e != nil && err == nil {
				err = e
			}
		}
	}()
	if err = q.Copy(&c, &row[index]); err != nil {
		return nil, err
	}
	if zero, e := q.IsZero(&c); e != nil || zero {
		return row, e
	}
	if err = q.Copy(&t, &solved[index]); err != nil {
		return nil, err
	}
	if err = q.Abs(&t); err != nil {
		return nil, err
	}
	unit, err := q.IsOne(&t)
	if err != nil {
		return nil, err
	}
	n := len(row)
	if unit {
		// c*sign_s, with sign_s the sign of s[index].
		pos, e := q.IsPos(&solved[index])
		if e != nil {
			return nil, e
		}
		if !pos {
			if err = q.Neg(&c); err != nil {
				return nil, err
			}
		}
		for i := 0; i < n; i++ {
			if err = q.Mul(&c, &solved[i], &t); err != nil {
				return nil, err
			}
			if err = q.Sub(&row[i], &t, &row[i]); err != nil {
				return nil, err
			}
		}
		for i := n; i < len(solved); i++ {
			v := smtQZero()
			if err = q.Mul(&c, &solved[i], &v); err == nil {
				err = q.Neg(&v)
			}
			row = append(row, v)
			if err != nil {
				return nil, err
			}
		}
		return row, nil
	}
	if err = q.SetInt64(&row[index], 0); err != nil {
		return nil, err
	}
	for i := 0; i < n; i++ {
		if err = q.Mul(&c, &solved[i], &t); err != nil {
			return nil, err
		}
		if err = q.Add(&row[i], &t, &row[i]); err != nil {
			return nil, err
		}
	}
	for i := n; i < len(solved); i++ {
		v := smtQZero()
		err = q.Mul(&c, &solved[i], &v)
		row = append(row, v)
		if err != nil {
			return nil, err
		}
	}
	return row, nil
}

// copyRow is unsat_row = r: a copy the caller owns.
func (s *smtArithmeticEquationSolver) copyRow(row []numeral.Rational) ([]numeral.Rational, error) {
	out := make([]numeral.Rational, len(row))
	for i := range row {
		out[i] = smtQZero()
		if e := s.q.Copy(&out[i], &row[i]); e != nil {
			_, d := smtQDelAll(s.q, out)
			if d != nil {
				return nil, d
			}
			return nil, e
		}
	}
	return out, nil
}

// SolveEquations is solve_integer_equations (solve_integer_equations_gcd).
// A false result with a row returns unsat_row, a row whose constant
// violates the variable coefficient GCD divisibility test; the caller owns
// it. A false result without a row is the source's pivot that is not a
// unit (unsat_row left empty).
func (s *smtArithmeticEquationSolver) SolveEquations(rows [][]numeral.Rational) (_ bool, _ []numeral.Rational, err error) {
	if len(rows) == 0 {
		return true, nil, nil
	}
	q := s.q
	columns := len(rows[0])
	live := make([]int, len(rows))
	for i := range rows {
		if !smtIntegerRowValid(rows[i]) || len(rows[i]) != columns {
			return false, nil, ast.ErrASTDomain
		}
		live[i] = i
		if err = s.gcdNormalize(rows[i]); err != nil {
			return false, nil, err
		}
		valid, e := s.gcdTest(rows[i])
		if e != nil {
			return false, nil, e
		}
		if !valid {
			row, e := s.copyRow(rows[i])
			return false, row, e
		}
	}
	sc := &smtEqSolverScratch{q: q}
	g, k, u, v, t1, t2 := smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero(), smtQZero()
	var gcds []numeral.Rational
	defer func() {
		sc.del(&g, &k, &u, &v, &t1, &t2)
		_, e := smtQDelAll(q, gcds)
		sc.keep(e)
		if err == nil {
			err = sc.err
		}
	}()
	// unsigned_vector useful is declared once per call and never reset:
	// from the second pivot on r0 is the first pivot row ever chosen,
	// recombined with every earlier useful row.
	var useful, gcdPos []int
	for change := true; change && len(live) != 0; {
		change = false
		for i := 1; i < columns; i++ {
			if err = q.SetInt64(&g, 0); err != nil {
				return false, nil, err
			}
			if gcds, err = smtQDelAll(q, gcds); err != nil {
				return false, nil, err
			}
			gcdPos = gcdPos[:0]
			j := 0
			for ; j < len(live); j++ {
				kz, e := q.IsZero(&rows[live[j]][i])
				if e != nil {
					return false, nil, e
				}
				if kz {
					continue
				}
				if err = q.Copy(&k, &rows[live[j]][i]); err != nil {
					return false, nil, err
				}
				if err = q.Abs(&k); err != nil {
					return false, nil, err
				}
				gz, e := q.IsZero(&g)
				if e != nil {
					return false, nil, e
				}
				if gz {
					err = q.Copy(&g, &k)
				} else {
					err = q.GCD(&g, &k, &g)
				}
				if err != nil {
					return false, nil, err
				}
				if one, e := q.IsOne(&g); e != nil || one {
					if e != nil {
						return false, nil, e
					}
					break
				}
				saved, e := smtQNew(q, &g)
				if e != nil {
					return false, nil, e
				}
				gcds = append(gcds, saved)
				gcdPos = append(gcdPos, live[j])
			}
			if j == len(live) {
				continue
			}
			change = true
			// found gcd, now identify reduced set of rows with GCD = 1.
			if err = q.Copy(&g, &rows[live[j]][i]); err != nil {
				return false, nil, err
			}
			if err = q.Abs(&g); err != nil {
				return false, nil, err
			}
			useful = append(useful, live[j])
			livePos := j
			for j = len(gcds); j > 0; {
				one, e := q.IsOne(&g)
				if e != nil {
					return false, nil, e
				}
				if one {
					break
				}
				j--
				prior := false
				if j > 0 {
					if err = q.GCD(&g, &gcds[j-1], &t1); err != nil {
						return false, nil, err
					}
					if prior, err = q.IsOne(&t1); err != nil {
						return false, nil, err
					}
				}
				if j == 0 || !prior {
					useful = append(useful, gcdPos[j])
					if err = q.GCD(&g, &gcds[j], &g); err != nil {
						return false, nil, err
					}
				}
			}
			// we now have a set "useful" of rows whose combined GCD = 1.
			// pivot the remaining with the first row.
			r0 := rows[useful[0]]
			for j = 1; j < len(useful); j++ {
				r1 := rows[useful[j]]
				if err = q.ExtendedGCD(&r0[i], &r1[i], &u, &v, &g); err != nil {
					return false, nil, err
				}
				for c := 0; c < columns; c++ {
					if err = q.Mul(&u, &r0[c], &t1); err != nil {
						return false, nil, err
					}
					if err = q.Mul(&v, &r1[c], &t2); err != nil {
						return false, nil, err
					}
					if err = q.Add(&t1, &t2, &r0[c]); err != nil {
						return false, nil, err
					}
				}
			}
			if err = q.Copy(&t1, &r0[i]); err != nil {
				return false, nil, err
			}
			if err = q.Abs(&t1); err != nil {
				return false, nil, err
			}
			if one, e := q.IsOne(&t1); e != nil || !one {
				return false, nil, e
			}
			live = append(live[:livePos], live[livePos+1:]...)
			for _, rowIndex := range live {
				rz, e := q.IsZero(&rows[rowIndex][i])
				if e != nil {
					return false, nil, e
				}
				if rz {
					continue
				}
				row, e := s.substitute(rows[rowIndex], r0, i)
				if e != nil {
					return false, nil, e
				}
				rows[rowIndex] = row
				if err = s.gcdNormalize(row); err != nil {
					return false, nil, err
				}
				valid, e := s.gcdTest(row)
				if e != nil {
					return false, nil, e
				}
				if !valid {
					out, e := s.copyRow(row)
					return false, out, e
				}
			}
		}
	}
	return true, nil, nil
}
