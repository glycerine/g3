package smtcore

// Source: src/smt/theory_diff_logic_def.h:1079-1404 optimization callbacks.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtDiffObjectiveTerm is objective_term's (theory_var, rational) pair.
type smtDiffObjectiveTerm struct {
	variable    SMTTheoryVar
	coefficient numeral.Rational
}
type smtDiffObjective struct {
	terms    []smtDiffObjectiveTerm
	constant numeral.Rational // m_objective_consts[v]
	core     []ast.ASTHandle  // owning expr_ref_vector, independent of Boolean scopes
}

func (o *smtDiffObjective) del(q *numeral.RationalManager) error {
	err := smtQDel(q, &o.constant)
	for i := range o.terms {
		if e := smtQDel(q, &o.terms[i].coefficient); err == nil {
			err = e
		}
	}
	return err
}

// The simplex package holds math/big values: rationals cross that boundary
// through smtQBig / SetBigRat.
type smtDiffOptimization struct {
	objectives []smtDiffObjective
	solver     *simplex.Solver
	rows       []simplex.Row
	edges      int
	manager    *ast.ASTManager
	q          *numeral.RationalManager
}

func (o *smtDiffOptimization) resetSimplex() {
	if o.edges != 0 {
		o.solver, o.rows, o.edges = nil, nil, 0
	}
}
func (o *smtDiffOptimization) node(v int32) uint32 {
	return uint32(len(o.objectives)) + 2*uint32(v) + 1
}
func (o *smtDiffOptimization) edge(e int) uint32 { return uint32(len(o.objectives)) + 2*uint32(e) }
func (t *SMTIntegerDiffTheory) closeDiffOptimization() (result error) {
	if t == nil {
		return nil
	}
	o := &t.optimization
	if o.manager == nil {
		return nil
	}
	for i := range o.objectives {
		for _, h := range o.objectives[i].core {
			result = errors.Join(result, o.manager.DecRef(h))
		}
		o.objectives[i].core = nil
		result = errors.Join(result, o.objectives[i].del(o.q))
	}
	o.objectives, o.solver, o.rows, o.edges, o.manager = nil, nil, nil, 0, nil
	return result
}

// The compiler serves the live real/mixed sparse and dense difference
// backends. Internalization dispatches to the concrete sort owner.
// The retained optimizer registers all objectives before maximization; their
// count reserves the initial simplex indices, as in the source bridge.
func (t *SMTRealDiffTheory) Close() error {
	if t == nil {
		return nil
	}
	err := t.SMTIntegerDiffTheory.Close()
	if t.realGraph != nil && t.realGraph.q != nil {
		t.realGraph.release()
		*t.realGraph = SMTInfRationalDiffGraph{q: t.realGraph.q, lastEnabledEdge: -1}
		t.realGraph.heap.graph = t.realGraph
		err = errors.Join(err, t.realGraph.takeErr(nil))
	}
	if t.q != nil {
		err = errors.Join(err, smtQDel(t.q, &t.delta))
		t.delta = smtQZero()
	}
	return err
}

// Close frees the graph's numerals and the objectives, as
// ~theory_diff_logic does.
func (t *SMTIntegerDiffTheory) Close() error {
	if t == nil {
		return nil
	}
	err := t.closeDiffOptimization()
	if t.graph != nil && t.graph.q != nil {
		t.graph.Reset()
		err = errors.Join(err, t.graph.takeErr(nil))
	}
	return err
}

func (t *SMTRealDiffTheory) AddObjective(term ast.ASTHandle) (SMTTheoryVar, error) {
	return t.addDiffObjective(term, nil)
}

// The dense source compiler attaches an already internalized foreign enode
// when it has no arithmetic variable. Sparse sort hooks already attach it.
func (t *SMTIntegerDiffTheory) addDiffObjective(term ast.ASTHandle, dense *SMTDenseDiffTheory) (SMTTheoryVar, error) {
	s, err := t.State()
	if err != nil {
		return SMTNullTheoryVar, err
	}
	u, err := NewSMTTheoryOptimizationUtil(s.manager)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	linear, err := u.IsLinear(term)
	if err != nil || !linear {
		return SMTNullTheoryVar, err
	}
	t.bindQ(s)
	q := t.q
	objective := smtDiffObjective{constant: smtQZero()}
	value := smtQZero()
	defer smtQDel(q, &value)
	// appendTerm adds (v, multiplier) to the objective.
	appendTerm := func(v SMTTheoryVar, multiplier *numeral.Rational) error {
		objective.terms = append(objective.terms, smtDiffObjectiveTerm{variable: v, coefficient: smtQZero()})
		return q.Copy(&objective.terms[len(objective.terms)-1].coefficient, multiplier)
	}
	var compile func(ast.ASTHandle, *numeral.Rational) (bool, error)
	compile = func(h ast.ASTHandle, multiplier *numeral.Rational) (bool, error) {
		isNumeral, err := t.numeral(h, &value)
		if err != nil {
			return false, err
		}
		if isNumeral {
			// Source adds the numeral itself, without multiplying this accumulator.
			return true, q.Add(&objective.constant, &value, &objective.constant)
		}
		kind, err := s.manager.GetKind(h)
		if err != nil || kind != ast.ASTApp {
			return false, err
		}
		op, own, err := t.operation(h)
		if err != nil {
			return false, err
		}
		if own && op == smtArithOpAdd {
			count, err := s.manager.GetNumArgs(h)
			if err != nil {
				return false, err
			}
			for i := uint32(0); i < count; i++ {
				arg, err := s.manager.GetArg(h, i)
				if err != nil {
					return false, err
				}
				ok, err := compile(arg, multiplier)
				if err != nil || !ok {
					return false, err
				}
			}
			return true, nil
		}
		if own && op == smtArithOpMul {
			for i := uint32(0); i < 2; i++ {
				arg, err := s.manager.GetArg(h, i)
				if err != nil {
					return false, err
				}
				isNumeral, err := t.numeral(arg, &value)
				if err != nil {
					return false, err
				}
				if isNumeral {
					arg, err = s.manager.GetArg(h, 1-i)
					if err != nil {
						return false, err
					}
					product := smtQZero()
					defer smtQDel(q, &product)
					if err = q.Mul(multiplier, &value, &product); err != nil {
						return false, err
					}
					return compile(arg, &product)
				}
			}
		}
		if own {
			return false, nil
		}
		if dense == nil {
			// theory_diff_logic::internalize_objective: mk_var(to_app(n)).
			v, err := t.mkVarApp(h)
			if err != nil {
				return false, err
			}
			return true, appendTerm(v, multiplier)
		}
		if err := s.Internalize(h, false); err != nil {
			return false, err
		}
		node := s.context.enodes.get(h)
		if node != nil && !t.IsAttachedToVariable(node) && dense != nil {
			if _, err := dense.createVariable(node); err != nil {
				return false, err
			}
		}
		if node == nil || !t.IsAttachedToVariable(node) {
			return false, ast.ErrASTDomain
		}
		return true, appendTerm(node.TheoryVar(t.FamilyID()), multiplier)
	}
	ok, err := compile(term, smtQIntPtr(1))
	if err != nil || !ok {
		return SMTNullTheoryVar, errors.Join(err, objective.del(q))
	}
	index := SMTTheoryVar(len(t.optimization.objectives))
	t.optimization.objectives = append(t.optimization.objectives, objective)
	t.optimization.manager = s.manager
	t.optimization.q = q
	return index, nil
}
func (t *SMTIntegerDiffTheory) diffObjective(v SMTTheoryVar) (*smtDiffObjective, error) {
	if _, err := t.State(); err != nil {
		return nil, err
	}
	if v < 0 || int(v) >= len(t.optimization.objectives) {
		return nil, ast.ErrASTDomain
	}
	return &t.optimization.objectives[v], nil
}

func (o *smtDiffOptimization) addObjectiveRows() error {
	for len(o.rows) < len(o.objectives) {
		i := len(o.rows)
		objective := &o.objectives[i]
		var variables []uint32
		var coefficients []big.Rat
		for j := range objective.terms {
			term := &objective.terms[j]
			variables = append(variables, o.node(int32(term.variable)))
			c, err := smtQBig(o.q, &term.coefficient)
			if err != nil {
				return err
			}
			coefficients = append(coefficients, *c)
		}
		variables = append(variables, uint32(i))
		coefficients = append(coefficients, *big.NewRat(1, 1))
		row, err := o.solver.AddRationalRow(uint32(i), variables, coefficients)
		if err != nil {
			return err
		}
		o.rows = append(o.rows, row)
	}
	return nil
}
func (t *SMTRealDiffTheory) updateObjectiveSimplex() error {
	o := &t.optimization
	if o.solver == nil {
		o.solver = simplex.NewSolver(&o.manager.Limit)
	}
	for i := range t.realGraph.assignment {
		a, err := t.realInfBig(&t.realGraph.assignment[i])
		if err != nil {
			return err
		}
		o.solver.SetValue(o.node(int32(i)), a)
	}
	zero := simplex.Infinitesimal{}
	if err := o.solver.SetLower(o.node(int32(t.zero)), zero); err != nil {
		return err
	}
	if err := o.solver.SetUpper(o.node(int32(t.zero)), zero); err != nil {
		return err
	}
	for i := o.edges; i < len(t.realGraph.edges); i++ {
		e := &t.realGraph.edges[i]
		if _, err := o.solver.AddIntegerRow(o.edge(i), []uint32{o.node(e.target), o.node(e.source), o.edge(i)}, []big.Int{*big.NewInt(1), *big.NewInt(-1), *big.NewInt(-1)}); err != nil {
			return err
		}
	}
	o.edges = len(t.realGraph.edges)
	for i := range t.realGraph.edges {
		e := &t.realGraph.edges[i]
		if e.enabled {
			w, err := t.realInfBig(&e.weight)
			if err != nil {
				return err
			}
			if err := o.solver.SetUpper(o.edge(i), w); err != nil {
				return err
			}
		} else {
			o.solver.UnsetUpper(o.edge(i))
		}
	}
	return o.addObjectiveRows()
}

func (t *SMTRealDiffTheory) ObjectiveValue(v SMTTheoryVar) (SMTArithmeticObjectiveValue, error) {
	objective, err := t.diffObjective(v)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, err
	}
	// value(v): the source reads get_assignment(v) (the objective index)
	// for every term; kept as in z3.
	q := t.q
	main, epsilon, value, r2 := smtQZero(), smtQZero(), newSMTInfIntRational(), smtQZero()
	defer func() {
		smtQDel(q, &main)
		smtQDel(q, &epsilon)
		value.del(q)
		smtQDel(q, &r2)
	}()
	if err := q.Copy(&main, &objective.constant); err != nil {
		return SMTArithmeticObjectiveValue{}, err
	}
	for i := range objective.terms {
		term := &objective.terms[i]
		if err := t.realGraph.Assignment(int32(v), &value); err != nil {
			return SMTArithmeticObjectiveValue{}, err
		}
		if err := q.AddMul(&main, &term.coefficient, &value.rational, &main); err != nil {
			return SMTArithmeticObjectiveValue{}, err
		}
		if err := q.SetInt64(&r2, int64(value.infinitesimal)); err != nil {
			return SMTArithmeticObjectiveValue{}, err
		}
		if err := q.AddMul(&epsilon, &term.coefficient, &r2, &epsilon); err != nil {
			return SMTArithmeticObjectiveValue{}, err
		}
	}
	result, err := smtQInfinitesimalBig(q, &main, &epsilon)
	return SMTArithmeticObjectiveValue{Finite: result}, err
}

// smtQInfinitesimalBig is main + epsilon*eps as a simplex value (math/big,
// a boundary).
func smtQInfinitesimalBig(q *numeral.RationalManager, main, epsilon *numeral.Rational) (simplex.Infinitesimal, error) {
	m, err := smtQBig(q, main)
	if err != nil {
		return simplex.Infinitesimal{}, err
	}
	e, err := smtQBig(q, epsilon)
	if err != nil {
		return simplex.Infinitesimal{}, err
	}
	return simplex.NewInfinitesimal(m, e), nil
}

// realInfBig is an inf_int_rational as a simplex value (a boundary).
func (t *SMTRealDiffTheory) realInfBig(v *smtInfIntRational) (simplex.Infinitesimal, error) {
	m, err := smtQBig(t.q, &v.rational)
	if err != nil {
		return simplex.Infinitesimal{}, err
	}
	return simplex.NewInfinitesimal(m, big.NewRat(int64(v.infinitesimal), 1)), nil
}

func (t *SMTIntegerDiffTheory) minimizeObjective(v SMTTheoryVar) (SMTArithmeticObjectiveValue, ast.ASTHandle, bool, error) {
	o := &t.optimization
	s, _ := t.State()
	status, err := o.solver.MakeFeasible()
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	if status == lbool.False {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	if status != lbool.Undef {
		status, err = o.solver.Minimize(uint32(v))
		if err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
	}
	if status != lbool.True {
		blocker, err := s.manager.MkFalse()
		return SMTArithmeticObjectiveValue{Unbounded: true}, blocker, false, err
	}
	objective := &o.objectives[v]
	for len(objective.core) > 0 {
		last := len(objective.core) - 1
		if err := s.manager.DecRef(objective.core[last]); err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
		objective.core = objective.core[:last]
	}
	value := o.solver.Value(uint32(v))
	value.Neg()
	return SMTArithmeticObjectiveValue{Finite: value}, ast.ASTHandle{}, false, nil
}

func (t *SMTIntegerDiffTheory) saveObjectiveCore(v SMTTheoryVar, literals []SMTLiteral) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	for _, literal := range literals {
		expressions, err := s.clauseLiteralExpressions([]SMTLiteral{literal})
		if err != nil {
			return err
		}
		h := expressions[0]
		if err = s.manager.IncRef(h); err != nil {
			return err
		}
		t.optimization.objectives[v].core = append(t.optimization.objectives[v].core, h)
	}
	return nil
}
func (t *SMTRealDiffTheory) MaximizeObjective(v SMTTheoryVar) (SMTArithmeticObjectiveValue, ast.ASTHandle, bool, error) {
	objective, err := t.diffObjective(v)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	if err = t.updateObjectiveSimplex(); err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	result, blocker, shared, err := t.minimizeObjective(v)
	if err != nil || result.Unbounded {
		return result, blocker, shared, err
	}
	entries, err := t.optimization.solver.RowEntries(t.optimization.rows[v])
	if err != nil {
		return result, blocker, false, err
	}
	var literals []SMTLiteral
	for _, entry := range entries {
		offset := int64(entry.Variable) - int64(len(t.optimization.objectives))
		if offset >= 0 && offset%2 == 0 {
			literals = append(literals, t.realGraph.edges[offset/2].explanation)
		}
	}
	if err = t.saveObjectiveCore(v, literals); err != nil {
		return result, blocker, false, err
	}
	if err = t.computeModelDelta(true); err != nil {
		return result, blocker, false, err
	}
	main, epsilon := smtQZero(), smtQZero()
	defer smtQDel(t.q, &main)
	defer smtQDel(t.q, &epsilon)
	for i := range t.realGraph.assignment {
		// r = val.get_rational() + m_delta * val.get_infinitesimal()
		value := t.optimization.solver.Value(t.optimization.node(int32(i)))
		if err = t.q.SetBigRat(&main, &value.Main); err != nil {
			return result, blocker, false, err
		}
		if err = t.q.SetBigRat(&epsilon, &value.Epsilon); err != nil {
			return result, blocker, false, err
		}
		a := &t.realGraph.assignment[i]
		if err = t.q.AddMul(&main, &t.delta, &epsilon, &a.rational); err != nil {
			return result, blocker, false, err
		}
		a.infinitesimal = 0
	}
	blocker, err = t.diffObjectiveInequality(v, result.Finite, true)
	constant, e := smtQBig(t.q, &objective.constant)
	if e != nil {
		return result, blocker, false, errors.Join(err, e)
	}
	result.Finite.Main.Add(&result.Finite.Main, constant)
	return result, blocker, false, err
}

func (t *SMTIntegerDiffTheory) diffObjectiveInequality(v SMTTheoryVar, value simplex.Infinitesimal, strict bool) (ast.ASTHandle, error) {
	objective, err := t.diffObjective(v)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s, _ := t.State()
	m := s.manager
	var expression ast.ASTHandle
	opposite := false
	if len(objective.terms) == 2 {
		// objective[0].second == -objective[1].second
		negated := smtQZero()
		err = t.q.Copy(&negated, &objective.terms[1].coefficient)
		if err == nil {
			err = t.q.Neg(&negated)
		}
		if err == nil {
			opposite, err = t.q.Equal(&objective.terms[0].coefficient, &negated)
		}
		if e := smtQDel(t.q, &negated); err == nil {
			err = e
		}
		if err != nil {
			return expression, err
		}
	}
	if len(objective.terms) == 1 && (objective.terms[0].coefficient.IsOne() || objective.terms[0].coefficient.IsMinusOne()) {
		node, err := t.Enode(objective.terms[0].variable)
		if err != nil {
			return expression, err
		}
		expression = node.owner
		if objective.terms[0].coefficient.IsNeg() {
			expression, err = m.MkFamilyApp(t.FamilyID(), smtArithOpUMinus, nil, []ast.ASTHandle{expression})
			if err != nil {
				return expression, err
			}
		}
	} else if len(objective.terms) == 2 && opposite && (objective.terms[0].coefficient.IsOne() || objective.terms[0].coefficient.IsMinusOne()) {
		first, second := objective.terms[0].variable, objective.terms[1].variable
		if objective.terms[0].coefficient.IsNeg() {
			first, second = second, first
		}
		left, err := t.Enode(first)
		if err != nil {
			return expression, err
		}
		right, err := t.Enode(second)
		if err != nil {
			return expression, err
		}
		expression, err = m.MkFamilyApp(t.FamilyID(), smtArithOpSub, nil, []ast.ASTHandle{left.owner, right.owner})
		if err != nil {
			return expression, err
		}
	}
	if expression.IsNull() || !strict && value.Epsilon.Sign() < 0 {
		expression, err = m.MkAndArgs(objective.core)
		if err != nil {
			return expression, err
		}
		if strict {
			return m.MkNot(expression)
		}
		return expression, nil
	}
	root, err := m.NewRef(expression)
	if err != nil {
		return expression, err
	}
	defer m.DelRef(&root)
	sort, err := m.GetSort(expression)
	if err != nil {
		return expression, err
	}
	integer, err := t.integerSort(sort)
	if err != nil {
		return expression, err
	}
	numeral, err := m.ArithmeticNumeralRat(&value.Main, integer)
	if err != nil {
		return expression, err
	}
	op := smtArithOpGE
	if strict && value.Epsilon.Sign() >= 0 {
		op = smtArithOpGT
	}
	return m.MkFamilyApp(t.FamilyID(), op, nil, []ast.ASTHandle{expression, numeral})
}
func (t *SMTRealDiffTheory) MakeObjectiveLowerBound(_ *tactic.FilterModelConverter, v SMTTheoryVar, value simplex.Infinitesimal) (ast.ASTHandle, error) {
	if value.Infinity.Sign() != 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return t.diffObjectiveInequality(v, value, false)
}
