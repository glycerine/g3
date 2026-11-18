package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h:841-1130 optimization.

import (
	"errors"

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

// infParts returns the to_rational() parts of a dense numeral as a fresh
// simplex value of the AST manager's rational manager.
func (t *SMTDenseDiffTheory) infParts(parts func(n, k *numeral.Rational) error) (simplex.Infinitesimal, error) {
	value := simplex.ZeroInfinitesimal()
	if err := parts(&value.Main, &value.Epsilon); err != nil {
		return value, smtErrJoin(err, value.Del(t.q()))
	}
	return value, nil
}

// ObjectiveValue is value(v): the objective constant plus the coefficients
// times the assignment, a fresh value the caller owns.
func (t *SMTDenseDiffTheory) ObjectiveValue(v SMTTheoryVar) (result SMTArithmeticObjectiveValue, err error) {
	result.Finite = simplex.ZeroInfinitesimal()
	objective, err := t.syntax.diffObjective(v)
	if err != nil {
		return result, err
	}
	q := t.syntax.q
	defer func() {
		if err != nil {
			err = smtErrJoin(err, result.Del(q))
		}
	}()
	if err = q.Copy(&result.Finite.Main, &objective.constant); err != nil {
		return result, err
	}
	for i := range objective.terms {
		coefficient := &objective.terms[i].coefficient
		// This source revision indexes the assignment by objective index v.
		if v < 0 || int(v) >= t.modelVars {
			return result, ast.ErrASTDomain
		}
		point, err := t.infParts(func(n, k *numeral.Rational) error { return t.ext.assignmentParts(int(v), n, k) })
		if err == nil {
			err = q.AddMul(&result.Finite.Main, coefficient, &point.Main, &result.Finite.Main)
		}
		if err == nil {
			err = q.AddMul(&result.Finite.Epsilon, coefficient, &point.Epsilon, &result.Finite.Epsilon)
		}
		if err = smtErrJoin(err, point.Del(q)); err != nil {
			return result, err
		}
	}
	return result, nil
}
func (t *SMTDenseDiffTheory) MakeObjectiveLowerBound(_ *tactic.FilterModelConverter, v SMTTheoryVar, value simplex.Infinitesimal) (ast.ASTHandle, error) {
	if finite, err := t.syntax.q.IsZero(&value.Infinity); err != nil {
		return ast.ASTHandle{}, err
	} else if !finite {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return t.syntax.diffObjectiveInequality(v, value, false)
}

// MaximizeObjective builds the source's local simplex S over the dense
// edges; S (and its numeral manager) dies when the call returns.
func (t *SMTDenseDiffTheory) MaximizeObjective(v SMTTheoryVar) (result SMTArithmeticObjectiveValue, blocker ast.ASTHandle, shared bool, err error) {
	result.Finite = simplex.ZeroInfinitesimal()
	objective, err := t.syntax.diffObjective(v)
	if err != nil {
		return result, blocker, false, err
	}
	state, err := t.State()
	if err != nil {
		return result, blocker, false, err
	}
	nodes, edges := t.NumVariables(), len(t.denseMatrix().edges)
	if t.modelVars != nodes {
		return result, blocker, false, ast.ErrASTDomain
	}
	q := t.syntax.q
	solver, err := simplex.NewSolver(&state.manager.Limit, state.manager.NumeralService())
	if err != nil {
		return result, blocker, false, err
	}
	defer func() { err = smtErrJoin(err, solver.Close()) }()
	solver.EnsureVariable(uint32(nodes + edges + len(t.syntax.optimization.objectives)))
	for i := 0; i < nodes; i++ {
		point, err := t.infParts(func(n, k *numeral.Rational) error { return t.ext.assignmentParts(i, n, k) })
		if err == nil {
			err = solver.SetValue(uint32(i), &point)
		}
		if err = smtErrJoin(err, point.Del(q)); err != nil {
			return result, blocker, false, err
		}
	}
	for _, node := range t.variables {
		zero, err := t.syntax.numeralIs(node.owner, 0)
		if err != nil {
			return result, blocker, false, err
		}
		if zero {
			// Source sets bounds on v (objective index), not the zero node index.
			zero := simplex.ZeroInfinitesimal()
			if err = solver.SetLower(uint32(v), &zero); err != nil {
				return result, blocker, false, err
			}
			if err = solver.SetUpper(uint32(v), &zero); err != nil {
				return result, blocker, false, err
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
		if _, err = solver.AddInt64Row(base, []uint32{uint32(edge.target), uint32(edge.source), base}, []int64{1, -1, -1}); err != nil {
			return result, blocker, false, err
		}
		weight, err := t.infParts(func(n, k *numeral.Rational) error { return t.ext.edgeParts(i, n, k) })
		if err == nil {
			err = solver.SetUpper(base, &weight)
		}
		if err = smtErrJoin(err, weight.Del(q)); err != nil {
			return result, blocker, false, err
		}
	}
	base := uint32(nodes+edges) + uint32(v)
	var variables []uint32
	var coefficients []numeral.Rational
	for i := range objective.terms {
		term := &objective.terms[i]
		variables = append(variables, uint32(term.variable))
		coefficients = append(coefficients, term.coefficient)
	}
	variables = append(variables, base)
	// The row borrows the terms' coefficients; the final 1 is a temporary.
	coefficients = append(coefficients, smtQZero())
	one := &coefficients[len(coefficients)-1]
	err = q.SetInt32(one, 1)
	var row simplex.Row
	if err == nil {
		row, err = solver.AddRationalRow(base, variables, coefficients)
	}
	if err = smtErrJoin(err, smtQDel(q, one)); err != nil {
		return result, blocker, false, err
	}
	status, err := solver.MakeFeasible()
	if err != nil {
		return result, blocker, false, err
	}
	if status == lbool.False {
		return result, blocker, false, ast.ErrASTDomain
	}
	if status != lbool.Undef {
		status, err = solver.Minimize(base)
		if err != nil {
			return result, blocker, false, err
		}
	}
	if status != lbool.True {
		blocker, err := state.manager.MkFalse()
		return smtUnboundedObjectiveValue(), blocker, false, err
	}
	defer func() {
		if err != nil {
			err = smtErrJoin(err, result.Del(q))
		}
	}()
	if err = solver.Value(base, &result.Finite); err != nil {
		return result, blocker, false, err
	}
	if err = result.Finite.Neg(q); err != nil {
		return result, blocker, false, err
	}
	for len(objective.core) > 0 {
		last := len(objective.core) - 1
		if err = state.manager.DecRef(objective.core[last]); err != nil {
			return result, blocker, false, err
		}
		objective.core = objective.core[:last]
	}
	entries, err := solver.RowEntries(row)
	if err != nil {
		return result, blocker, false, err
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
		return result, blocker, false, err
	}
	point := simplex.ZeroInfinitesimal()
	for i := 0; i < nodes; i++ {
		// Preserve the source's explicit TBD: dense maximization publishes the
		// rational part only, unlike the sparse backend's finite-delta assignment.
		// numeral a(q); m_assignment[i] = a.
		err = solver.Value(uint32(i), &point)
		if err == nil {
			err = t.ext.setAssignment(i, &point.Main)
		}
		if err != nil {
			return result, blocker, false, smtErrJoin(err, point.Del(q))
		}
	}
	if err = point.Del(q); err != nil {
		return result, blocker, false, err
	}
	blocker, err = t.syntax.diffObjectiveInequality(v, result.Finite, true)
	if e := q.Add(&result.Finite.Main, &objective.constant, &result.Finite.Main); e != nil && err == nil {
		err = e
	}
	return result, blocker, false, err
}
