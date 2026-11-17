package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h:841-1130 optimization.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Source explicitly shares objective syntax with sparse difference logic.
// Only the concrete term/compiler/core storage is shared; dense maximization
// builds a fresh tableau over the dense edge list on every call.
func (t *SMTDenseDiffTheory) AddObjective(term ast.ASTHandle) (SMTTheoryVar, error) {
	return t.syntax.addDiffObjective(term, t)
}
func (t *SMTDenseDiffTheory) Close() error {
	if t == nil {
		return nil
	}
	// ~theory_dense_diff_logic calls reset_eh, whose del_atoms(0) traces the
	// remaining (base-level) atoms.
	if t.state != nil {
		for i := len(t.atoms) - 1; i >= 0; i-- {
			t.traceDeleteAtom(&t.atoms[i], func() {})
		}
	}
	err := t.syntax.closeDiffOptimization()
	if t.ext != nil {
		err = errors.Join(err, t.ext.close())
	}
	if t.syntax.q != nil {
		err = errors.Join(err, smtQDel(t.syntax.q, &t.epsilon))
	}
	return err
}

// smtDenseBigParts returns the to_rational() parts of a dense numeral as
// math/big values for the simplex package (a boundary conversion).
func (t *SMTDenseDiffTheory) bigParts(parts func(n, k *numeral.Rational) error) (*big.Rat, *big.Rat, error) {
	q := t.q()
	n, k := smtQZero(), smtQZero()
	defer smtQDel(q, &n)
	defer smtQDel(q, &k)
	if err := parts(&n, &k); err != nil {
		return nil, nil, err
	}
	main, err := smtQBig(q, &n)
	if err != nil {
		return nil, nil, err
	}
	epsilon, err := smtQBig(q, &k)
	return main, epsilon, err
}
func (t *SMTDenseDiffTheory) ObjectiveValue(v SMTTheoryVar) (SMTArithmeticObjectiveValue, error) {
	objective, err := t.syntax.diffObjective(v)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, err
	}
	constant, err := smtQBig(t.syntax.q, &objective.constant)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, err
	}
	value := simplex.RationalInfinitesimal(constant)
	for i := range objective.terms {
		coefficient, err := smtQBig(t.syntax.q, &objective.terms[i].coefficient)
		if err != nil {
			return SMTArithmeticObjectiveValue{}, err
		}
		// This source revision indexes the assignment by objective index v.
		if v < 0 || int(v) >= t.modelVars {
			return SMTArithmeticObjectiveValue{}, ast.ErrASTDomain
		}
		main, epsilon, err := t.bigParts(func(n, k *numeral.Rational) error { return t.ext.assignmentParts(int(v), n, k) })
		if err != nil {
			return SMTArithmeticObjectiveValue{}, err
		}
		value.Main.Add(&value.Main, new(big.Rat).Mul(coefficient, main))
		value.Epsilon.Add(&value.Epsilon, new(big.Rat).Mul(coefficient, epsilon))
	}
	return SMTArithmeticObjectiveValue{Finite: value}, nil
}
func (t *SMTDenseDiffTheory) MakeObjectiveLowerBound(_ *tactic.FilterModelConverter, v SMTTheoryVar, value simplex.Infinitesimal) (ast.ASTHandle, error) {
	if value.Infinity.Sign() != 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return t.syntax.diffObjectiveInequality(v, value, false)
}

func (t *SMTDenseDiffTheory) MaximizeObjective(v SMTTheoryVar) (SMTArithmeticObjectiveValue, ast.ASTHandle, bool, error) {
	objective, err := t.syntax.diffObjective(v)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	state, err := t.State()
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	nodes, edges := t.NumVariables(), len(t.denseMatrix().edges)
	if t.modelVars != nodes {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	solver := simplex.NewSolver(&state.manager.Limit)
	solver.EnsureVariable(uint32(nodes + edges + len(t.syntax.optimization.objectives)))
	for i := 0; i < nodes; i++ {
		main, epsilon, err := t.bigParts(func(n, k *numeral.Rational) error { return t.ext.assignmentParts(i, n, k) })
		if err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
		solver.SetValue(uint32(i), simplex.NewInfinitesimal(main, epsilon))
	}
	for _, node := range t.variables {
		zero, err := t.syntax.numeralIs(node.owner, 0)
		if err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
		if zero {
			// Source sets bounds on v (objective index), not the zero node index.
			zero := simplex.Infinitesimal{}
			if err = solver.SetLower(uint32(v), zero); err != nil {
				return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
			}
			if err = solver.SetUpper(uint32(v), zero); err != nil {
				return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
			}
			break
		}
	}
	for i := range t.matrix.edges {
		edge := &t.matrix.edges[i]
		if edge.source < 0 || edge.target < 0 {
			continue
		} // Null edge zero is retained.
		base := uint32(nodes + i)
		if _, err = solver.AddIntegerRow(base, []uint32{uint32(edge.target), uint32(edge.source), base}, []big.Int{*big.NewInt(1), *big.NewInt(-1), *big.NewInt(-1)}); err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
		main, epsilon, err := t.bigParts(func(n, k *numeral.Rational) error { return t.ext.edgeParts(i, n, k) })
		if err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
		if err = solver.SetUpper(base, simplex.NewInfinitesimal(main, epsilon)); err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
	}
	base := uint32(nodes+edges) + uint32(v)
	var variables []uint32
	var coefficients []big.Rat
	for i := range objective.terms {
		term := &objective.terms[i]
		variables = append(variables, uint32(term.variable))
		c, err := smtQBig(t.syntax.q, &term.coefficient)
		if err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
		coefficients = append(coefficients, *c)
	}
	variables = append(variables, base)
	coefficients = append(coefficients, *big.NewRat(1, 1))
	row, err := solver.AddRationalRow(base, variables, coefficients)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	status, err := solver.MakeFeasible()
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	if status == lbool.False {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	if status != lbool.Undef {
		status, err = solver.Minimize(base)
		if err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
	}
	if status != lbool.True {
		blocker, err := state.manager.MkFalse()
		return SMTArithmeticObjectiveValue{Unbounded: true}, blocker, false, err
	}
	value := solver.Value(base)
	value.Neg()
	for len(objective.core) > 0 {
		last := len(objective.core) - 1
		if err = state.manager.DecRef(objective.core[last]); err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
		objective.core = objective.core[:last]
	}
	entries, err := solver.RowEntries(row)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	var literals []SMTLiteral
	for _, entry := range entries {
		if int(entry.Variable) >= nodes && int(entry.Variable) < nodes+edges {
			literal := t.matrix.edges[int(entry.Variable)-nodes].literal
			if literal != SMTNullLiteral {
				literals = append(literals, literal)
			}
		}
	}
	if err = t.syntax.saveObjectiveCore(v, literals); err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	for i := 0; i < nodes; i++ {
		point := solver.Value(uint32(i))
		// Preserve the source's explicit TBD: dense maximization publishes the
		// rational part only, unlike the sparse backend's finite-delta assignment.
		// numeral a(q); m_assignment[i] = a.
		r := smtQZero()
		err := smtQFromBig(t.q(), &r, &point.Main)
		if err == nil {
			err = t.ext.setAssignment(i, &r)
		}
		if e := smtQDel(t.q(), &r); err == nil {
			err = e
		}
		if err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
	}
	blocker, err := t.syntax.diffObjectiveInequality(v, value, true)
	constant, e := smtQBig(t.syntax.q, &objective.constant)
	if e != nil && err == nil {
		err = e
	}
	if constant != nil {
		value.Main.Add(&value.Main, constant)
	}
	return SMTArithmeticObjectiveValue{Finite: value}, blocker, false, err
}
