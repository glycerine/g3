package smtcore

// Source: src/smt/tactic/ctx_solver_simplify_tactic.cpp/.h.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/util/statistics"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ContextSolverSimplifyTactic performs the source solver-guided, first-occurrence
// contextual traversal. It owns a private SMT solver and simplified-app helper.
type ContextSolverSimplifyTactic struct {
	self        *ContextSolverSimplifyTactic
	manager     *ast.ASTManager
	config      SMTSolverConfig
	params      uparams.ParamsRef
	paramsReady bool
	boolSort    ast.ASTHandle
	intSort     ast.ASTHandle
	solver      *SMTSolver
	mkApp       *rewriter.SimplifiedApp
	fn          ast.ASTHandle // m_fn, held
	fns         *ast.ObjMap   // m_fns: sort -> held declaration
	steps       uint32
	refs        uint32
	closed      bool
}

var _ tactic.Tactic = (*ContextSolverSimplifyTactic)(nil)

// MkContextSolverSimplifyTactic is mk_ctx_solver_simplify_tactic:
// clean(alloc(ctx_solver_simplify_tactic)), so cleanup() resets the private
// solver after every application.
func MkContextSolverSimplifyTactic(m *ast.ASTManager, p *uparams.ParamsRef, config SMTSolverConfig) (tactic.Tactic, error) {
	t, err := NewContextSolverSimplifyTactic(m, p, config)
	if err != nil {
		return nil, err
	}
	clean, err := tactic.NewCleanTactic(t)
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return clean, nil
}

func NewContextSolverSimplifyTactic(m *ast.ASTManager, p *uparams.ParamsRef, config SMTSolverConfig) (*ContextSolverSimplifyTactic, error) {
	if m == nil || config.Params == nil || config.Globals == nil || config.Globals.Context != config.Params {
		return nil, uparams.ErrParamsOwner
	}
	if err := config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(ContextSolverSimplifyTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &ContextSolverSimplifyTactic{manager: m, config: config}
	t.self = t
	params, err := config.Params.NewValue()
	t.paramsReady = err == nil
	if err == nil {
		t.params = params
	}
	if err == nil && p != nil {
		err = config.Params.Assign(&t.params, p)
	}
	if err == nil {
		t.boolSort, err = m.MkBoolSort()
	}
	if err == nil {
		t.solver, err = NewSMTSolver(m, &t.params, "", config)
	}
	if err == nil {
		t.mkApp, err = rewriter.NewSimplifiedApp(m, config.Params, config.Globals, &t.params)
	}
	t.fns = ast.NewObjMap(m)
	if err == nil {
		var fid ast.FamilyID
		fid, err = m.ArithmeticFamilyID()
		if err == nil {
			t.intSort, err = m.MkFamilySort(fid, smtArithIntSort, nil)
		}
	}
	if err == nil {
		var fn ast.ASTHandle
		fn, err = m.MkFuncDecl(usymbol.Numerical(0xbeef101), []ast.ASTHandle{t.intSort}, t.boolSort, ast.ASTFuncDeclInfo{})
		if err == nil {
			if err = m.IncRef(fn); err == nil {
				t.fn = fn
			}
		}
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func (t *ContextSolverSimplifyTactic) open() error {
	if t == nil || t.self != t || t.manager == nil {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *ContextSolverSimplifyTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *ContextSolverSimplifyTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}
func (t *ContextSolverSimplifyTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.fns != nil {
		for _, entry := range t.fns.Entries() {
			if err := t.manager.DecRef(entry.Value[0]); err != nil {
				return err
			}
		}
		t.fns = nil
	}
	if !t.fn.IsNull() {
		if err := t.manager.DecRef(t.fn); err != nil {
			return err
		}
		t.fn = ast.ASTHandle{}
	}
	if t.mkApp != nil {
		if err := t.mkApp.Close(); err != nil {
			return err
		}
		t.mkApp = nil
	}
	if t.solver != nil {
		if err := t.solver.Close(); err != nil {
			return err
		}
		t.solver = nil
	}
	if t.manager != nil && t.paramsReady {
		if err := t.config.Params.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	if t.manager != nil {
		t.manager.Budget().Release(uint64(reflect.TypeOf(ContextSolverSimplifyTactic{}).Size()))
	}
	t.manager, t.config = nil, SMTSolverConfig{}
	t.closed = true
	return nil
}

func (t *ContextSolverSimplifyTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if g == nil || out == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err := out.Prepare(t.manager); err != nil {
		return err
	}
	if err := t.reduceGoal(g); err != nil {
		return err
	}
	if err := g.IncDepth(); err != nil {
		return err
	}
	return out.AddGoal(g)
}

// reduceGoal is reduce(goal& g).
func (t *ContextSolverSimplifyTactic) reduceGoal(g *tactic.Goal) (resultErr error) {
	fml, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, t.manager.DelRef(&fml)) }()
	report, err := tactic.NewTacticReport("ctx-solver-simplify", g)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { resultErr = errors.Join(resultErr, report.Close()) }()
	}
	inconsistent, err := g.Inconsistent()
	if err != nil || inconsistent {
		return err
	}
	n, err := g.Size()
	if err != nil {
		return err
	}
	fmls := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		if fmls[i], err = g.Form(i); err != nil {
			return err
		}
	}
	conjunction, err := t.manager.MkAndArray(fmls)
	if err == nil {
		err = t.manager.SetRef(fml, conjunction)
	}
	if err != nil {
		return fmt.Errorf("ctx-solver-simplify: combine goal: %w", err)
	}
	base, err := t.solver.ScopeLevel()
	if err != nil {
		return err
	}
	if err = t.solver.Push(); err != nil {
		return err
	}
	if err = t.reduce(fml); err != nil {
		level, e := t.solver.ScopeLevel()
		if e == nil && level > base {
			e = t.solver.Pop(level - base)
		}
		return fmt.Errorf("ctx-solver-simplify: reduce goal: %w", errors.Join(err, e))
	}
	if err = t.solver.Pop(1); err != nil {
		return err
	}
	result, err := t.manager.RefGet(fml)
	if err != nil {
		return err
	}
	if err = g.Reset(); err != nil {
		return err
	}
	return g.Assert(result)
}

// ctxSolverExprPos is expr_pos.
type ctxSolverExprPos struct {
	parent, self, idx uint32
	expr              ast.ASTHandle
}

// reduce is reduce(expr_ref& result): the single-traversal context
// simplifier, which simplifies only the first occurrence of a sub-term.
func (t *ContextSolverSimplifyTactic) reduce(result ast.ASTRef) (resultErr error) {
	m := t.manager
	var names []ast.ASTHandle
	var todo []ctxSolverExprPos
	var trail []ast.ASTRef
	cache := make(map[ast.ASTHandle]ctxSolverExprPos)
	id, childID := uint32(1), uint32(0)
	var tmp, n2, fml ast.ASTRef
	var err error
	// Locals die in reverse declaration order: fml, n2, tmp, then the trail
	// (forward).
	defer func() {
		resultErr = errors.Join(resultErr, m.DelRef(&fml), m.DelRef(&n2), m.DelRef(&tmp))
		for i := range trail {
			resultErr = errors.Join(resultErr, m.DelRef(&trail[i]))
		}
	}()
	for _, r := range []*ast.ASTRef{&tmp, &n2, &fml} {
		if *r, err = m.NewRef(ast.ASTHandle{}); err != nil {
			return err
		}
	}
	pinTrail := func(h ast.ASTHandle) error {
		r, e := m.NewRef(h)
		if e == nil {
			trail = append(trail, r)
		}
		return e
	}
	n, err := t.mkFresh(&id, t.boolSort)
	if err != nil {
		return err
	}
	if err = pinTrail(n); err != nil {
		return err
	}
	root, err := m.RefGet(result)
	if err != nil {
		return err
	}
	if err = m.SetRef(fml, root); err != nil {
		return err
	}
	iff, err := m.MkIff(root, n)
	if err != nil {
		return err
	}
	neg, err := m.MkNot(iff)
	if err == nil {
		err = m.SetRef(tmp, neg)
	}
	if err == nil {
		err = t.solver.Assert(neg)
	}
	if err != nil {
		return err
	}
	todo = append(todo, ctxSolverExprPos{expr: root})
	names = append(names, n)
	if err = t.solver.Push(); err != nil {
		return err
	}
	canceled := false
	for len(todo) > 0 {
		if canceled, err = m.Canceled(); err != nil {
			return err
		}
		if canceled {
			break
		}
		var res ast.ASTHandle
		top := todo[len(todo)-1]
		e, selfPos, parentPos, selfIdx := top.expr, top.self, top.parent, top.idx
		n = names[len(names)-1]
		done, err := t.reduceStep(e, n, selfPos, &res, &id, &childID, cache, &todo, &names, pinTrail, n2, tmp)
		if err != nil {
			return err
		}
		if !done {
			continue
		}
		if !res.IsNull() {
			cache[e] = ctxSolverExprPos{parent: parentPos, self: selfPos, idx: selfIdx, expr: res}
		}
		todo = todo[:len(todo)-1]
		names = names[:len(names)-1]
		if err = t.solver.Pop(1); err != nil {
			return err
		}
	}
	if !canceled {
		path, ok := cache[root]
		if !ok {
			return fmt.Errorf("ctx-solver-simplify: missing root result")
		}
		return m.SetRef(result, path.expr)
	}
	return nil
}

// reduceStep is one iteration of reduce's loop up to the done label; it
// reports false when a child was scheduled (continue).
func (t *ContextSolverSimplifyTactic) reduceStep(e, n ast.ASTHandle, selfPos uint32, res *ast.ASTHandle, id, childID *uint32, cache map[ast.ASTHandle]ctxSolverExprPos, todo *[]ctxSolverExprPos, names *[]ast.ASTHandle, pinTrail func(ast.ASTHandle) error, n2, tmp ast.ASTRef) (bool, error) {
	m := t.manager
	if _, ok := cache[e]; ok {
		return true, nil
	}
	sort, err := m.GetSort(e)
	if err != nil {
		return false, err
	}
	if sort == t.boolSort {
		simplified, ok, err := t.simplifyBool(n)
		if err != nil {
			return false, err
		}
		if ok {
			*res = simplified
			return true, nil
		}
	}
	kind, err := m.GetKind(e)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		*res = e
		return true, nil
	}
	sz, err := m.GetNumArgs(e)
	if err != nil {
		return false, err
	}
	if err = m.SetRef(n2, ast.ASTHandle{}); err != nil {
		return false, err
	}
	child := ast.ASTHandle{}
	args := make([]ast.ASTHandle, 0, sz)
	for i := uint32(0); i < sz; i++ {
		arg, err := m.GetArg(e, i)
		if err != nil {
			return false, err
		}
		if path, ok := cache[arg]; ok {
			if path.parent == selfPos && path.idx == i {
				args = append(args, path.expr)
			} else {
				args = append(args, arg)
			}
		} else if child.IsNull() {
			argSort, err := m.GetSort(arg)
			if err != nil {
				return false, err
			}
			fresh, err := t.mkFresh(id, argSort)
			if err == nil {
				err = m.SetRef(n2, fresh)
			}
			if err == nil {
				err = pinTrail(fresh)
			}
			if err != nil {
				return false, err
			}
			child = fresh
			*todo = append(*todo, ctxSolverExprPos{parent: selfPos, self: *childID, idx: i, expr: arg})
			*childID++
			*names = append(*names, fresh)
			args = append(args, fresh)
		} else {
			args = append(args, arg)
		}
	}
	decl, err := m.GetDecl(e)
	if err != nil {
		return false, err
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	err = t.mkApp.Make(decl, ast.SliceArguments(args), out)
	if err == nil {
		*res, err = m.RefGet(out)
	}
	if err == nil {
		err = pinTrail(*res)
	}
	if err = errors.Join(err, m.DelRef(&out)); err != nil {
		return false, err
	}
	if child.IsNull() {
		return true, nil
	}
	// child needs to be visited.
	if err = t.solver.Push(); err != nil {
		return false, err
	}
	eq, err := m.MkEq(*res, n)
	if err == nil {
		err = m.SetRef(tmp, eq)
	}
	if err == nil {
		err = t.solver.Assert(eq)
	}
	return false, err
}

// simplifyBool is simplify_bool: n is forced true or false by the context.
func (t *ContextSolverSimplifyTactic) simplifyBool(n ast.ASTHandle) (res ast.ASTHandle, ok bool, resultErr error) {
	m := t.manager
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return res, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&tmp)) }()
	if err = t.solver.Push(); err != nil {
		return res, false, err
	}
	err = t.solver.Assert(n)
	status := lbool.Undef
	if err == nil {
		status, err = t.solver.CheckSat(nil)
	}
	if err = errors.Join(err, t.solver.Pop(1)); err != nil {
		return res, false, err
	}
	if status == lbool.False {
		res, err = m.MkTrue()
		return res, err == nil, err
	}
	if err = t.solver.Push(); err != nil {
		return res, false, err
	}
	negated, err := m.MkNot(n)
	if err == nil {
		err = m.SetRef(tmp, negated)
	}
	if err == nil {
		err = t.solver.Assert(negated)
	}
	if err == nil {
		status, err = t.solver.CheckSat(nil)
	}
	if err = errors.Join(err, t.solver.Pop(1)); err != nil {
		return res, false, err
	}
	if status == lbool.False {
		res, err = m.MkFalse()
		return res, err == nil, err
	}
	return res, false, nil
}

// mkFresh is mk_fresh: Boolean proxies apply m_fn; other sorts get a
// per-sort declaration symbol(0xbeef101 + id) : Int -> s, kept in m_fns.
func (t *ContextSolverSimplifyTactic) mkFresh(id *uint32, s ast.ASTHandle) (ast.ASTHandle, error) {
	m := t.manager
	fn := t.fn
	if s != t.boolSort {
		found, ok, err := t.fns.Find(s)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if ok {
			fn = found[0]
		} else {
			fn, err = m.MkFuncDecl(usymbol.Numerical(0xbeef101+*id), []ast.ASTHandle{t.intSort}, s, ast.ASTFuncDeclInfo{})
			if err == nil {
				err = m.IncRef(fn)
			}
			if err == nil {
				err = t.fns.Insert(s, [2]ast.ASTHandle{fn})
			}
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
	}
	numeral, err := m.ArithmeticNumeralRat(big.NewRat(int64(*id), 1), true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	*id++
	return m.MkApp(fn, []ast.ASTHandle{numeral})
}

func (t *ContextSolverSimplifyTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.config.Params.Assign(&t.params, p); err != nil {
		return err
	}
	return t.solver.UpdateParams(p)
}
func (t *ContextSolverSimplifyTactic) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.solver.CollectParamDescrs(d)
}
func (t *ContextSolverSimplifyTactic) CollectStatistics(st *statistics.Statistics) {
	if t == nil || t.closed {
		return
	}
	st.UpdateUint64("solver-simplify-steps", uint64(t.steps))
}
func (t *ContextSolverSimplifyTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.steps = 0
	return nil
}
func (t *ContextSolverSimplifyTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	t.steps = 0
	old := t.solver
	next, err := NewSMTSolver(t.manager, &t.params, "", t.config)
	if err != nil {
		return err
	}
	if err = old.Close(); err != nil {
		return errors.Join(err, next.Close())
	}
	t.solver = next
	return nil
}
func (t *ContextSolverSimplifyTactic) Reset() error                             { return t.Cleanup() }
func (t *ContextSolverSimplifyTactic) SetLogic(_ usymbol.Symbol) error          { return t.open() }
func (t *ContextSolverSimplifyTactic) SetProgressCallback(_ func(uint32)) error { return t.open() }
func (t *ContextSolverSimplifyTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	config := t.config
	if err := config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		if config.TargetConfig == nil {
			return nil, err
		}
		config, err = config.TargetConfig(m)
		if err != nil {
			return nil, err
		}
	}
	params, err := t.config.Params.TranslateTo(&t.params, config.Params)
	if err != nil {
		return nil, err
	}
	defer config.Params.Del(&params)
	return NewContextSolverSimplifyTactic(m, &params, config)
}
