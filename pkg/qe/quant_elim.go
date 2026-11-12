package qe

// Source: src/qe/qe.cpp, expr_quant_elim (operator(), elim,
// instantiate_expr, abstract_expr), quant_elim_new (eliminate,
// eliminate_exists, eliminate_block, pop_context/push_context,
// eliminate_exists_bind/eliminate_forall_bind, bind_variables) and
// quant_elim_plugin (construction, add_plugin, reset, check, blast_or).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// exprQuantElim is expr_quant_elim. Its quant_elim_new (and the
// quant_elim_plugin stack it keeps) lives as long as the owning tactic.
type exprQuantElim struct {
	manager *ast.ASTManager
	config  Config
	qe      *quantElimNew
	visited map[ast.ASTHandle]ast.ASTHandle
	trail   []ast.ASTRef
}

func newExprQuantElim(m *ast.ASTManager, config Config) *exprQuantElim {
	return &exprQuantElim{manager: m, config: config, visited: make(map[ast.ASTHandle]ast.ASTHandle)}
}

// initQE is expr_quant_elim::init_qe.
func (e *exprQuantElim) initQE() {
	if e.qe == nil {
		e.qe = &quantElimNew{manager: e.manager, config: e.config, eliminateVariablesAsBlock: true}
	}
	// The tactic's parameters (models, qe_nonlinear, statistics) are read
	// when a plugin set is built, as m_fparams is in the source.
	e.qe.config = e.config
}

func (e *exprQuantElim) Close() error {
	if e == nil {
		return nil
	}
	err := e.resetTrail()
	if e.qe != nil {
		err = errors.Join(err, e.qe.Close())
		e.qe = nil
	}
	return err
}

func (e *exprQuantElim) resetTrail() (err error) {
	for i := len(e.trail) - 1; i >= 0; i-- {
		err = errors.Join(err, e.manager.DelRef(&e.trail[i]))
	}
	e.trail = e.trail[:0]
	clear(e.visited)
	return err
}

func (e *exprQuantElim) collectStatistics(st *statistics.Statistics) error {
	if e == nil || e.qe == nil || st == nil {
		return nil
	}
	return e.qe.collectStatistics(st)
}

// run is expr_quant_elim::operator()(assumption, fml, result).
func (e *exprQuantElim) run(assumption, fml ast.ASTHandle) (result ast.ASTRef, err error) {
	m := e.manager
	instantiated, err := instantiateExprFreeVars(m, fml)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, instantiated.Close()) }()
	e.initQE()
	if err = m.SetRef(e.qe.assumptionRef(), assumption); err != nil {
		return result, err
	}
	if err = e.elim(instantiated.formula); err != nil {
		return result, errors.Join(err, e.resetTrail())
	}
	if err = e.resetTrail(); err != nil {
		return result, err
	}
	value, err := m.RefGet(instantiated.formula)
	if err != nil {
		return result, err
	}
	return instantiated.abstract(value)
}

// elim is expr_quant_elim::elim's todo walk over applications and
// quantifiers; the quantifier's body is eliminated before the quantifier.
func (e *exprQuantElim) elim(result ast.ASTRef) error {
	m := e.manager
	root, err := m.RefGet(result)
	if err != nil {
		return err
	}
	if err = e.pushTrail(root); err != nil {
		return err
	}
	todo := []ast.ASTHandle{root}
	for len(todo) > 0 {
		cur := todo[len(todo)-1]
		if _, ok := e.visited[cur]; ok {
			todo = todo[:len(todo)-1]
			continue
		}
		if err = e.checkpoint(); err != nil {
			return err
		}
		kind, err := m.GetKind(cur)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			n, err := m.GetNumArgs(cur)
			if err != nil {
				return err
			}
			args := make([]ast.ASTHandle, 0, n)
			allVisited := true
			for i := uint32(0); i < n; i++ {
				arg, err := m.GetArg(cur, i)
				if err != nil {
					return err
				}
				if r, ok := e.visited[arg]; ok {
					args = append(args, r)
				} else {
					todo = append(todo, arg)
					allVisited = false
				}
			}
			if allVisited {
				decl, err := m.GetDecl(cur)
				if err != nil {
					return err
				}
				r, err := m.MkApp(decl, args)
				if err != nil {
					return err
				}
				todo = todo[:len(todo)-1]
				if err = e.pushTrail(r); err != nil {
					return err
				}
				e.visited[cur] = r
			}
		case ast.ASTQuantifier:
			isForall, err := m.IsForall(cur)
			if err != nil {
				return err
			}
			extracted, err := extractExprQuantifier(m, cur)
			if err != nil {
				return closeFailedExprQuantifierExtract(extracted, err)
			}
			err = e.elimQuantifier(isForall, extracted)
			if err != nil {
				return errors.Join(err, extracted.Close())
			}
			tmp, err := m.RefGet(extracted.body)
			if err == nil {
				err = e.pushTrail(tmp)
			}
			if err = errors.Join(err, extracted.Close()); err != nil {
				return err
			}
			e.visited[cur] = tmp
			todo = todo[:len(todo)-1]
		default:
			return ast.ErrASTDomain
		}
	}
	return m.SetRef(result, e.visited[root])
}

func (e *exprQuantElim) elimQuantifier(isForall bool, extracted *exprQuantifierExtract) error {
	if err := e.elim(extracted.body); err != nil {
		return err
	}
	e.initQE()
	vars := make([]ast.ASTHandle, len(extracted.variables))
	for i, ref := range extracted.variables {
		v, err := e.manager.RefGet(ref)
		if err != nil {
			return err
		}
		vars[i] = v
	}
	return e.qe.eliminate(isForall, vars, extracted.body)
}

func (e *exprQuantElim) pushTrail(h ast.ASTHandle) error {
	ref, err := e.manager.NewRef(h)
	if err != nil {
		return err
	}
	e.trail = append(e.trail, ref)
	return nil
}

func (e *exprQuantElim) checkpoint() error {
	return qeCheckpoint(e.manager, e.config)
}

func qeCheckpoint(m *ast.ASTManager, config Config) error {
	canceled, err := m.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(m.ResourceLimit().CancelMessage())
	}
	return config.Cooperation.Cooperate("qe")
}

// quantElimNew is quant_elim_new.
type quantElimNew struct {
	manager                   *ast.ASTManager
	config                    Config
	assumption                ast.ASTRef
	assumptionReady           bool
	plugins                   []*quantElimPlugin
	eliminateVariablesAsBlock bool
}

func (q *quantElimNew) assumptionRef() ast.ASTRef {
	if !q.assumptionReady {
		ref, err := q.manager.NewRef(ast.ASTHandle{})
		if err == nil {
			q.assumption = ref
			q.assumptionReady = true
		}
	}
	return q.assumption
}

func (q *quantElimNew) Close() (err error) {
	for i := len(q.plugins) - 1; i >= 0; i-- {
		err = errors.Join(err, q.plugins[i].Close())
	}
	q.plugins = nil
	if q.assumptionReady {
		err = errors.Join(err, q.manager.DelRef(&q.assumption))
		q.assumptionReady = false
	}
	return err
}

func (q *quantElimNew) collectStatistics(st *statistics.Statistics) (err error) {
	for _, th := range q.plugins {
		err = errors.Join(err, th.solver.CollectStatistics(st))
	}
	return err
}

func (q *quantElimNew) eliminate(isForall bool, vars []ast.ASTHandle, fml ast.ASTRef) error {
	if isForall {
		return q.eliminateForallBind(vars, fml)
	}
	return q.eliminateExistsBind(vars, fml)
}

// bindVariables is quant_elim_new::bind_variables.
func (q *quantElimNew) bindVariables(vars []ast.ASTHandle, fml ast.ASTRef) error {
	if len(vars) == 0 {
		return nil
	}
	m := q.manager
	body, err := m.RefGet(fml)
	if err != nil {
		return err
	}
	bound, err := bindExprQuantifierVariables(m, vars, body)
	if err != nil {
		return err
	}
	value, err := m.RefGet(bound)
	if err == nil {
		err = m.SetRef(fml, value)
	}
	return errors.Join(err, m.DelRef(&bound))
}

func (q *quantElimNew) eliminateExistsBind(vars []ast.ASTHandle, fml ast.ASTRef) (err error) {
	if err = qeCheckpoint(q.manager, q.config); err != nil {
		return err
	}
	var free []ast.ASTRef
	defer func() {
		for i := len(free) - 1; i >= 0; i-- {
			err = errors.Join(err, q.manager.DelRef(&free[i]))
		}
	}()
	if _, err = q.eliminateExists(vars, fml, &free); err != nil {
		return err
	}
	handles := make([]ast.ASTHandle, len(free))
	for i, ref := range free {
		if handles[i], err = q.manager.RefGet(ref); err != nil {
			return err
		}
	}
	return q.bindVariables(handles, fml)
}

func (q *quantElimNew) eliminateForallBind(vars []ast.ASTHandle, fml ast.ASTRef) (err error) {
	m := q.manager
	rw, err := rewriter.NewBoolRewriter(m, q.config.Context, q.config.Globals, nil)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	value, err := m.RefGet(fml)
	if err != nil {
		return err
	}
	if err = rw.MkNot(value, tmp); err != nil {
		return err
	}
	if err = q.eliminateExistsBind(vars, tmp); err != nil {
		return err
	}
	value, err = m.RefGet(tmp)
	if err != nil {
		return err
	}
	return rw.MkNot(value, fml)
}

func appendQEFreeVars(m *ast.ASTManager, free *[]ast.ASTRef, vars []ast.ASTHandle) error {
	for _, v := range vars {
		ref, err := m.NewRef(v)
		if err != nil {
			return err
		}
		*free = append(*free, ref)
	}
	return nil
}

// eliminateExists is quant_elim_new::eliminate_exists with get_first false
// and no definitions, the tactic's use.
func (q *quantElimNew) eliminateExists(vars []ast.ASTHandle, fml ast.ASTRef, free *[]ast.ASTRef) (lbool.LBool, error) {
	if q.eliminateVariablesAsBlock {
		return q.eliminateBlock(vars, fml, free)
	}
	for i := range vars {
		r, err := q.eliminateBlock(vars[i:i+1], fml, free)
		if err != nil {
			return lbool.Undef, err
		}
		switch r {
		case lbool.False:
			return lbool.False, nil
		case lbool.Undef:
			// free_vars.append(num_vars-i-1, vars+1+i)
			return lbool.Undef, appendQEFreeVars(q.manager, free, vars[i+1:])
		}
	}
	return lbool.True, nil
}

func (q *quantElimNew) eliminateBlock(vars []ast.ASTHandle, fml ast.ASTRef, free *[]ast.ASTRef) (_ lbool.LBool, err error) {
	m := q.manager
	if err := qeCheckpoint(m, q.config); err != nil {
		return lbool.Undef, err
	}
	value, err := m.RefGet(fml)
	if err != nil {
		return lbool.Undef, err
	}
	quantified, err := m.HasQuantifiers(value)
	if err != nil {
		return lbool.Undef, err
	}
	if quantified {
		return lbool.Undef, appendQEFreeVars(m, free, vars)
	}
	// expr_ref fml0(fml, m): the input formula stays alive until
	// eliminate_block returns, after check and push_context's reset.
	fml0, err := m.NewRef(value)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&fml0)) }()
	th, err := q.popContext()
	if err != nil {
		return lbool.Undef, err
	}
	err = th.check(vars, q.assumption, fml, free)
	q.pushContext(th)
	if err = errors.Join(err, th.reset()); err != nil {
		return lbool.Undef, err
	}
	value, err = m.RefGet(fml)
	if err != nil {
		return lbool.Undef, err
	}
	if m.IsFalse(value) {
		return lbool.False, nil
	}
	if len(*free) == 0 {
		return lbool.True, nil
	}
	return lbool.Undef, nil
}

// popContext builds the plugin set in the source order: bool, bv, arith,
// array, datatype and dl.
func (q *quantElimNew) popContext() (*quantElimPlugin, error) {
	if len(q.plugins) > 0 {
		th := q.plugins[len(q.plugins)-1]
		q.plugins = q.plugins[:len(q.plugins)-1]
		return th, nil
	}
	return newQuantElimPlugin(q)
}

func (q *quantElimNew) pushContext(th *quantElimPlugin) {
	q.plugins = append(q.plugins, th)
}

// quantElimPlugin is quant_elim_plugin.
type quantElimPlugin struct {
	manager      *ast.ASTManager
	qe           *quantElimNew
	config       Config
	rw           *rewriter.TheoryRewriter
	kernelParams uparams.ParamsRef
	paramsReady  bool
	solver       *smtcore.SMTSolver
	boolRw       *rewriter.BoolRewriter
	partitioner  *ConjunctionPartitioner
	context      *SolverContext
	boolPlugin   *BoolPlugin
	adapter      *SolverNNFAdapter
	nnf          *QENNFNormalizer
	freeVars     []ast.ASTRef
	// var2contains is m_var2contains' table; reset empties it without
	// shrinking a small table, so its layout carries across checks.
	var2contains *ast.ObjMap
	literals     exprQELiterals
	// subfml is m_subfml: get_max_relevant assigns it in check and
	// quant_elim_plugin::reset leaves it, so it lives until the next check
	// of this plugin or its destruction.
	subfml ast.ASTRef
}

func newQuantElimPlugin(q *quantElimNew) (th *quantElimPlugin, err error) {
	m := q.manager
	config := q.config
	th = &quantElimPlugin{manager: m, qe: q, config: config}
	defer func() {
		if err != nil {
			err = errors.Join(err, th.Close())
			th = nil
		}
	}()
	// m_rewriter with gcd_rounding.
	local, err := config.Context.NewValue()
	if err != nil {
		return th, err
	}
	name := "gcd_rounding"
	err = config.Context.SetBoolName(&local, &name, true)
	if err == nil {
		th.rw, err = rewriter.NewTheoryRewriter(m, config.Context, config.Globals, &local)
	}
	err = errors.Join(err, config.Context.Del(&local))
	if err != nil {
		return th, err
	}
	// m_solver(m, p): eliminate_block's flets (relevancy 0, models,
	// simplify_bit2int, arith_enum_const_mod, bv_enable_int2bv2int,
	// array_canonize_simplify) are in force for every use of the kernel.
	th.kernelParams, err = config.Context.NewValue()
	if err != nil {
		return th, err
	}
	th.paramsReady = true
	relevancy := "relevancy"
	if err = config.Context.SetUIntName(&th.kernelParams, &relevancy, 0); err != nil {
		return th, err
	}
	th.solver, err = smtcore.NewSMTSolver(m, &th.kernelParams, "", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace}})
	if err != nil {
		return th, err
	}
	if err = th.solver.ApplyQuantElimSettings(); err != nil {
		return th, err
	}
	if err = th.solver.SetProduceModels(true); err != nil {
		return th, err
	}
	th.boolRw, err = rewriter.NewBoolRewriter(m, config.Context, config.Globals, nil)
	if err != nil {
		return th, err
	}
	th.partitioner, err = NewConjunctionPartitioner(m)
	if err != nil {
		return th, err
	}
	th.context, err = NewSolverContext(m, nil)
	if err != nil {
		return th, err
	}
	th.context.owner = th
	th.adapter, err = NewSolverNNFAdapter(th.context)
	if err != nil {
		return th, err
	}
	th.nnf, err = NewQENNFNormalizer(m, th.adapter, th.adapter, config)
	if err != nil {
		return th, err
	}
	th.boolPlugin, err = NewBoolPlugin(th.context)
	if err != nil {
		return th, err
	}
	if err = th.addPlugin(th.boolPlugin); err != nil {
		return th, errors.Join(err, th.boolPlugin.Close())
	}
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		return th, err
	}
	bv, err := NewBitVectorPlugin(th.context)
	if err != nil {
		return th, err
	}
	if err = th.addPlugin(bv); err != nil {
		return th, errors.Join(err, bv.Close())
	}
	// mk_arith_plugin
	var arith SolverPlugin
	if config.nonlinearQuantifierElimination {
		nonlinear, e := NewNonlinearArithmeticPlugin(th.context, config.produceModels)
		if e != nil {
			return th, e
		}
		arith = nonlinear
	} else {
		linear, e := NewArithmeticPlugin(th.context, config)
		if e != nil {
			return th, e
		}
		linear.util.context = th.context
		arith = linear
	}
	if err = th.addPlugin(arith); err != nil {
		return th, errors.Join(err, arith.Close())
	}
	array, err := NewArrayPlugin(th.context)
	if err != nil {
		return th, err
	}
	if err = th.addPlugin(array); err != nil {
		return th, errors.Join(err, array.Close())
	}
	datatype, err := NewDatatypeSolverPlugin(th.context)
	if err != nil {
		return th, err
	}
	if err = th.addPlugin(datatype); err != nil {
		return th, errors.Join(err, datatype.Close())
	}
	datalog, err := NewDatalogPlugin(th.context)
	if err != nil {
		return th, err
	}
	if err = th.addPlugin(datalog); err != nil {
		return th, errors.Join(err, datalog.Close())
	}
	return th, nil
}

// addPlugin is quant_elim_plugin::add_plugin.
func (th *quantElimPlugin) addPlugin(p SolverPlugin) error {
	if err := th.context.AddPlugin(p); err != nil {
		return err
	}
	return th.partitioner.AddPlugin(p)
}

// simplifiers lists the plugins with a simplify override in family order.
func (th *quantElimPlugin) simplifiers() []exprQETreeSimplifier {
	var result []exprQETreeSimplifier
	for _, p := range th.context.plugins {
		if s, ok := p.(exprQETreeSimplifier); ok && p != nil {
			result = append(result, s)
		}
	}
	return result
}

// reset is quant_elim_plugin::reset.
func (th *quantElimPlugin) reset() (err error) {
	return th.resetCheck(nil, nil)
}

// resetCheck is quant_elim_plugin::reset over the state check keeps: free
// variables, m_trail and m_var2contains (in variables), m_root and
// m_new_vars.
func (th *quantElimPlugin) resetCheck(variables *exprQEBranchVariables, root *exprQETree) (err error) {
	for i := range th.freeVars {
		err = errors.Join(err, th.manager.DelRef(&th.freeVars[i]))
	}
	th.freeVars = th.freeVars[:0]
	if err != nil {
		return err
	}
	if err = variables.release(); err != nil {
		return err
	}
	if err = root.Close(); err != nil {
		return err
	}
	if err = variables.releasePending(); err != nil {
		return err
	}
	if th.context != nil {
		err = errors.Join(err, th.context.resetVariables())
	}
	return err
}

// Close is ~quant_elim_plugin: reset, then the members in reverse
// declaration order (m_nnf, m_subfml, m_conjs, m_bool_rewriter, m_literals,
// m_solver, m_rewriter), then i_solver_context's plugins.
func (th *quantElimPlugin) Close() (err error) {
	if th == nil {
		return nil
	}
	err = th.reset()
	if th.nnf != nil {
		err = errors.Join(err, th.nnf.Close())
		th.nnf = nil
	}
	if th.adapter != nil {
		err = errors.Join(err, th.adapter.Close())
		th.adapter = nil
	}
	if th.subfml != (ast.ASTRef{}) {
		err = errors.Join(err, th.manager.DelRef(&th.subfml))
	}
	if th.partitioner != nil {
		err = errors.Join(err, th.partitioner.Close())
		th.partitioner = nil
	}
	if th.boolRw != nil {
		err = errors.Join(err, th.boolRw.Close())
		th.boolRw = nil
	}
	err = errors.Join(err, th.literals.reset(th.manager))
	if th.solver != nil {
		err = errors.Join(err, th.solver.Close())
		th.solver = nil
	}
	if th.paramsReady {
		err = errors.Join(err, th.config.Context.Del(&th.kernelParams))
		th.paramsReady = false
	}
	if th.rw != nil {
		err = errors.Join(err, th.rw.Close())
		th.rw = nil
	}
	if th.context != nil {
		err = errors.Join(err, th.context.Close())
		th.context = nil
	}
	return err
}

// check is quant_elim_plugin::check with get_first false and no
// definitions.
func (th *quantElimPlugin) check(vars []ast.ASTHandle, assumption ast.ASTRef, fml ast.ASTRef, free *[]ast.ASTRef) (err error) {
	m := th.manager
	if err = th.reset(); err != nil {
		return err
	}
	if err = th.solver.PushCore(); err != nil {
		return err
	}
	popped := false
	defer func() {
		if !popped {
			err = errors.Join(err, th.solver.PopCore(1))
		}
	}()
	variables, err := newExprQEBranchVariables(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, variables.Close()) }()
	if th.var2contains == nil {
		th.var2contains = ast.NewObjMap(m)
	}
	variables.order = th.var2contains
	for _, v := range vars {
		has, e := th.context.HasPlugin(v)
		if e != nil {
			return e
		}
		if has {
			if e = variables.add(v); e != nil {
				return e
			}
			if e = th.context.AddVariable(v); e != nil {
				return e
			}
		} else if e = appendQEFreeVars(m, &th.freeVars, []ast.ASTHandle{v}); e != nil {
			return e
		}
	}
	truth, err := m.MkTrue()
	if err != nil {
		return err
	}
	root, err := newExprQETree(m, th.config, nil, truth)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	root.simplifiers = th.simplifiers()
	root.literals = &th.literals
	if err = variables.consume(root); err != nil {
		return err
	}
	value, err := m.RefGet(fml)
	if err != nil {
		return err
	}
	if err = root.initialize(value); err != nil {
		return err
	}
	// normalize(m_fml, m_root.pos_atoms(), m_root.neg_atoms())
	if err = normalizeExprQETree(root, th.rw, th.nnf); err != nil {
		return err
	}
	work, err := m.RefGet(root.formula)
	if err != nil {
		return err
	}
	if th.subfml == (ast.ASTRef{}) {
		if th.subfml, err = m.NewRef(ast.ASTHandle{}); err != nil {
			return err
		}
	}
	relevant, err := extractMaxRelevantExprInto(m, th.adapter, work, th.subfml)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, relevant.Close()) }()
	rebuilt, err := m.RefGet(relevant.formula)
	if err != nil {
		return err
	}
	sub, err := m.RefGet(relevant.subformula)
	if err != nil {
		return err
	}
	if rebuilt != sub {
		work = rebuilt
		if err = m.SetRef(root.formula, sub); err != nil {
			return err
		}
		if err = th.solver.AssertFormula(sub); err != nil {
			return err
		}
	}
	if err = th.solver.AssertFormula(work); err != nil {
		return err
	}
	if a, e := m.RefGet(assumption); e == nil && !a.IsNull() {
		if err = th.solver.AssertFormula(a); err != nil {
			return err
		}
	}
	isSat := false
	result, complete, err := searchExprQEScalarTreeSat(root, variables, th.context, th.boolPlugin, th.partitioner, th.solver, th.config, th.rw, th.nnf, th.boolRw, &th.freeVars, &isSat)
	if err != nil {
		if result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
		return err
	}
	if !complete {
		// l_undef: every block variable stays free and fml is unchanged.
		popped = true
		return errors.Join(appendQEFreeVars(m, free, vars), th.resetCheck(variables, root), th.solver.PopCore(1))
	}
	defer func() { err = errors.Join(err, m.DelRef(&result)) }()
	answer, err := m.RefGet(result)
	if err != nil {
		return err
	}
	if err = m.SetRef(fml, answer); err != nil {
		return err
	}
	// if (!m_free_vars.empty() || m_solver.inconsistent()): the result
	// replaces m_subfml inside the formula with its irrelevant conjuncts.
	if isSat && (len(th.freeVars) != 0 || th.solver.Inconsistent()) && rebuilt != sub {
		if err = qeReplaceSubformula(m, sub, answer, rebuilt, fml); err != nil {
			return err
		}
	}
	for _, ref := range th.freeVars {
		h, e := m.RefGet(ref)
		if e != nil {
			return e
		}
		if e = appendQEFreeVars(m, free, []ast.ASTHandle{h}); e != nil {
			return e
		}
	}
	popped = true
	return errors.Join(th.resetCheck(variables, root), th.solver.PopCore(1))
}

// blastOr is quant_elim_plugin::blast_or: the variable is eliminated from
// fml by quant_elim_new::eliminate_exists, sharing this plugin's free
// variables.
func (c *SolverContext) blastOr(variable ast.ASTHandle, fml ast.ASTRef) error {
	if err := c.open(); err != nil {
		return err
	}
	th := c.owner
	if th == nil {
		return ast.ErrASTHandle
	}
	_, err := th.qe.eliminateExists([]ast.ASTHandle{variable}, fml, &th.freeVars)
	return err
}

// qeReplaceSubformula is mk_default_expr_replacer's
// apply_substitution(subfml, value, fml): out receives fml with every
// occurrence of subfml replaced by value.
func qeReplaceSubformula(m *ast.ASTManager, subfml, value, formula ast.ASTHandle, out ast.ASTRef) (err error) {
	replacement, err := m.NewExprSubstitutionPolicy(false, false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelExprSubstitution(&replacement)) }()
	if err = m.ExprSubstitutionInsert(replacement, subfml, value, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
		return err
	}
	replacer, err := rewriter.NewDefaultExprReplacer(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, replacer.Close()) }()
	if err = replacer.SetSubstitution(&replacement); err != nil {
		return err
	}
	return replacer.Rewrite(formula, out)
}
