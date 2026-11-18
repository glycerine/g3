package bv

// Source: src/tactic/bv/elim_small_bv_tactic.cpp/.h.
// Christoph Winter, 2015-11-09.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/statistics"
	"io"
	"math"
	"reflect"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type ElimSmallBVTactic struct {
	self            *ElimSmallBVTactic
	manager         *ast.ASTManager
	context         *uparams.ParamsContext
	globals         *uparams.GlobalParamsService
	cooperation     *cooperate.Cooperation
	params          uparams.ParamsRef
	paramsReady     bool
	refs            uint32
	header          uint64
	closed, running bool
}

var _ tactic.Tactic = (*ElimSmallBVTactic)(nil)

func NewElimSmallBVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, coop *cooperate.Cooperation) (_ *ElimSmallBVTactic, err error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(ElimSmallBVTactic{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &ElimSmallBVTactic{manager: m, context: c, globals: g, cooperation: coop, header: n}
	t.self = t
	defer func() {
		if err != nil {
			err = errors.Join(err, t.Close())
		}
	}()
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	return t, err
}

func MakeElimSmallBVTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, coop *cooperate.Cooperation) (tactic.Tactic, error) {
	t, err := NewElimSmallBVTactic(m, c, g, p, coop)
	if err != nil {
		return nil, err
	}
	w, err := tactic.NewCleanTactic(t)
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return w, nil
}
func (t *ElimSmallBVTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.running {
		return ast.ErrASTPending
	}
	return t.context.Check(&t.params)
}
func (t *ElimSmallBVTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *ElimSmallBVTactic) DecRef() error {
	if e := t.open(); e != nil {
		return e
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
func (t *ElimSmallBVTactic) Close() (err error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.running {
		return ast.ErrASTPending
	}
	if t.paramsReady {
		err = t.context.Del(&t.params)
		if err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.header)
	t.header = 0
	t.closed = true
	return nil
}
func (t *ElimSmallBVTactic) UpdateParams(p *uparams.ParamsRef) error {
	if e := t.open(); e != nil {
		return e
	}
	return t.context.Assign(&t.params, p)
}
func (t *ElimSmallBVTactic) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if e := t.open(); e != nil {
		return e
	}
	// elim_small_bv_tactic::collect_param_descrs (elim_small_bv_tactic.cpp:284-288).
	if e := uparams.InsertMaxMemory(d); e != nil {
		return e
	}
	if e := uparams.InsertMaxSteps(d); e != nil {
		return e
	}
	c := "(default: 4) maximum bit-vector size of quantified bit-vectors to be eliminated."
	return d.InsertName("max_bits", uparams.ParamUInt, &c, nil, nil)
}
func (t *ElimSmallBVTactic) CollectStatistics(*statistics.Statistics) {}
func (t *ElimSmallBVTactic) ResetStatistics() error                   { return t.open() }
func (t *ElimSmallBVTactic) SetLogic(usymbol.Symbol) error            { return t.open() }
func (t *ElimSmallBVTactic) SetProgressCallback(func(uint32)) error   { return t.open() }
func (t *ElimSmallBVTactic) Cleanup() error                           { return t.open() }
func (t *ElimSmallBVTactic) Reset() error                             { return t.open() }
func (t *ElimSmallBVTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	return NewElimSmallBVTactic(m, t.context, t.globals, &t.params, t.cooperation)
}

type elimSmallBVConfig struct {
	rewriter.DefaultConfiguration
	owner             *ElimSmallBVTactic
	params            *uparams.ParamsRef
	simplify          *rewriter.TheoryRewriter
	substitute        *rewriter.VarSubstitution
	bindings          []ast.ASTHandle
	maxBits, maxSteps uint32
	maxMemory         uint64
	eliminated        uint32
}

func (c *elimSmallBVConfig) PreVisit(h ast.ASTHandle) (bool, error) {
	if e := c.emit("elim_small_bv_pre", "pre_visit", 204, func(w io.Writer) error {
		return c.writeParts(w, "pre_visit: ", h, "\n")
	}); e != nil {
		return false, e
	}
	k, e := c.owner.manager.GetKind(h)
	if e != nil {
		return false, e
	}
	if k == ast.ASTQuantifier {
		if e = c.emit("elim_small_bv", "pre_visit", 207, func(w io.Writer) error {
			id, err := c.owner.manager.GetASTID(h)
			if err != nil {
				return err
			}
			body, err := c.owner.manager.GetQuantifierBody(h)
			if err != nil {
				return err
			}
			return c.writeParts(w, "pre_visit quantifier [", strconv.FormatUint(uint64(id), 10), "]: ", body, "\n")
		}); e != nil {
			return false, e
		}
		n, e := c.owner.manager.GetNumDeclarations(h)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < n; i++ {
			s, e := c.owner.manager.GetDeclarationSort(h, i)
			if e != nil {
				return false, e
			}
			c.bindings = append(c.bindings, s)
		}
	}
	return true, nil
}

// checkpoint is C++ rw_cfg::max_steps_exceeded(num_steps).
func (c *elimSmallBVConfig) checkpoint(numSteps uint64) (bool, error) {
	if c.owner.cooperation != nil {
		if e := c.owner.cooperation.Cooperate("elim-small-bv"); e != nil {
			return false, e
		}
	}
	if numSteps > uint64(c.maxSteps) {
		return true, nil
	}
	allocated, e := c.owner.manager.NumeralService().RuntimeOwner().AllocationSize()
	if e != nil {
		return false, e
	}
	if allocated > c.maxMemory {
		return false, tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
	}
	return false, nil
}

// MaxStepsExceeded is the rewriter_tpl hook: rw_cfg::max_steps_exceeded
// hides default_rewriter_cfg's, so check_max_steps reaches it with the
// outer rewriter's step count.
func (c *elimSmallBVConfig) MaxStepsExceeded(n uint32) (bool, error) {
	return c.checkpoint(uint64(n))
}

// replaceVar is rw_cfg::replace_var. The source's expr_ref_vector
// substitution holds the replacement, its local var_subst (with its
// caches) dies at return, and m_simp(res, res, pr) releases the
// substituted term as soon as the simplified one replaces it.
func (c *elimSmallBVConfig) replaceVar(numDecls, maxVarIdxP1, idx uint32, e, replacement ast.ASTHandle) (res ast.ASTRef, err error) {
	m := c.owner.manager
	if err = c.trace("replace_var", 75, "replace idx ", strconv.FormatUint(uint64(idx), 10), " with ", replacement, " in ", e, "\n"); err != nil {
		return ast.ASTRef{}, err
	}
	substitution, err := m.NewRef(replacement)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() {
		// ~var_subst, then ~expr_ref_vector substitution.
		err = errors.Join(err, c.substitute.Reset())
		err = errors.Join(err, m.DelRef(&substitution))
	}()
	args := make([]ast.ASTHandle, numDecls+maxVarIdxP1)
	args[numDecls-idx-1] = replacement
	for l, r := 0, len(args)-1; l < r; l, r = l+1, r-1 {
		args[l], args[r] = args[r], args[l]
	}
	if err = c.traceSubstitution(args); err != nil {
		return ast.ASTRef{}, err
	}
	res, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	fail := func(e error) (ast.ASTRef, error) {
		return ast.ASTRef{}, errors.Join(e, m.DelRef(&res))
	}
	if err = c.substitute.Substitute(e, args, res); err != nil {
		return fail(err)
	}
	sub, err := m.RefGet(res)
	if err != nil {
		return fail(err)
	}
	simp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return fail(err)
	}
	if err = c.simplify.Rewrite(sub, simp); err != nil {
		return fail(errors.Join(err, m.DelRef(&simp)))
	}
	if err = m.AssignRef(res, simp); err != nil {
		return fail(errors.Join(err, m.DelRef(&simp)))
	}
	if err = m.DelRef(&simp); err != nil {
		return fail(err)
	}
	if err = c.trace("replace_var", 109, "replace done: ", mustGet(m, res), "\n"); err != nil {
		return fail(err)
	}
	return res, nil
}

func (c *elimSmallBVConfig) ReduceQuantifier(q, oldBody ast.ASTHandle, _, _ ast.AppArguments, out, _ ast.ASTRef) (changed bool, err error) {
	m := c.owner.manager
	if err = c.trace("reduce_quantifier", 126, "reduce_quantifier ", q, "\n"); err != nil {
		return false, err
	}
	n, err := m.GetNumDeclarations(q)
	if err != nil {
		return false, err
	}
	old := len(c.bindings) - int(n)
	defer func() { c.bindings = c.bindings[:old] }()
	used, err := ast.NewASTUsedVars(m)
	if err != nil {
		return false, err
	}
	if err = used.Collect(q); err != nil {
		return false, err
	}
	max := used.MaxFoundVarIndexPlusOne()
	// expr_ref body(old_body, m), released when the function returns.
	body, err := m.NewRef(oldBody)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&body)) }()
	// C++ reduce_quantifier: num_steps is a local, restarted per quantifier.
	var numSteps uint64
	for i := int(n) - 1; i >= 0; i-- {
		if stop, e := c.checkpoint(numSteps); e != nil {
			return false, e
		} else if stop {
			break
		}
		sort, e := m.GetDeclarationSort(q, uint32(i))
		if e != nil {
			return false, e
		}
		is, e := m.IsBitVectorSort(sort)
		if e != nil {
			return false, e
		}
		if !is {
			continue
		}
		width, e := m.BitVectorSize(sort)
		if e != nil {
			return false, e
		}
		if width > c.maxBits {
			continue
		}
		if stop, e := c.checkpoint(numSteps); e != nil {
			return false, e
		} else if stop {
			continue
		}
		// z3 4.7.1 enumerates only j < bv_sz values, so (forall ((y (_
		// BitVec 2))) (not (= y #b11))) becomes true, and a partial
		// enumeration under max_steps drops instances too. As upstream
		// (8e033c1e7, aa901c4e8) enumerate all 2^bv_sz values and leave
		// the quantifier alone when they do not fit in max_steps.
		if e = c.traceEliminating(q, uint32(i), sort, mustGet(m, body)); e != nil {
			return false, e
		}
		if width >= 31 {
			return false, nil
		}
		maxNum := uint32(1) << width
		if uint64(maxNum)+numSteps > uint64(c.maxSteps) {
			return false, nil
		}
		if changed, e := c.eliminate(q, uint32(i), n, max, width, maxNum, &numSteps, body); e != nil || !changed {
			return false, e
		}
	}
	// quantifier_ref new_q; unused_vars_eliminator el; both die at return,
	// el first, then new_q and body.
	updated, e := m.UpdateQuantifierBody(q, mustGet(m, body))
	if e != nil {
		return false, e
	}
	newQ, e := m.NewRef(updated)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, m.DelRef(&newQ)) }()
	reduced, e := rewriter.ElimUnusedVarsWithParams(m, updated, c.owner.context, c.params)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, m.DelRef(&reduced)) }()
	v, e := m.RefGet(reduced)
	if e != nil {
		return false, e
	}
	if e = m.SetRef(out, v); e != nil {
		return false, e
	}
	return true, c.trace("reduce_quantifier", 196, "elimination result: ", v, "\n")
}

func mustGet(m *ast.ASTManager, r ast.ASTRef) ast.ASTHandle {
	h, _ := m.RefGet(r)
	return h
}

// eliminate is the body of the is_small_bv branch of reduce_quantifier:
// new_bodies collects the instances, body becomes their simplified
// conjunction (disjunction), and new_bodies dies at the end of the block.
func (c *elimSmallBVConfig) eliminate(q ast.ASTHandle, i, n, max, width, maxNum uint32, numSteps *uint64, body ast.ASTRef) (ok bool, err error) {
	m := c.owner.manager
	var newBodies []ast.ASTRef
	defer func() {
		// ~expr_ref_vector releases front to back.
		for k := range newBodies {
			err = errors.Join(err, m.DelRef(&newBodies[k]))
		}
	}()
	current, err := m.RefGet(body)
	if err != nil {
		return false, err
	}
	for j := uint32(0); j < maxNum; j++ {
		stop, e := c.checkpoint(*numSteps)
		if e != nil {
			return false, e
		}
		if stop {
			break
		}
		replacement, e := m.BitVectorNumeralUnsigned(j, width)
		if e != nil {
			return false, e
		}
		// expr_ref n(...); expr_ref nb; nb = replace_var(...);
		// new_bodies.push_back(nb); then ~nb and ~n.
		numeral, e := m.NewRef(replacement)
		if e != nil {
			return false, e
		}
		nb, e := c.replaceVar(n, max, i, current, replacement)
		if e != nil {
			return false, errors.Join(e, m.DelRef(&numeral))
		}
		newBodies = append(newBodies, nb)
		if e = m.DelRef(&numeral); e != nil {
			return false, e
		}
		*numSteps++
	}
	if stop, e := c.checkpoint(*numSteps); e != nil {
		return false, e
	} else if stop {
		return false, nil
	}
	bodies := make([]ast.ASTHandle, len(newBodies))
	for k := range newBodies {
		if bodies[k], err = m.RefGet(newBodies[k]); err != nil {
			return false, err
		}
	}
	if e := c.traceNewBodies(bodies); e != nil {
		return false, e
	}
	forall, e := m.IsForall(q)
	if e != nil {
		return false, e
	}
	var combined ast.ASTHandle
	if forall {
		combined, e = m.MkAndArray(bodies)
	} else {
		combined, e = m.MkOrArray(bodies)
	}
	if e != nil {
		return false, e
	}
	// body = mk_and(...); m_simp(body, body, pr);
	if e = m.SetRef(body, combined); e != nil {
		return false, e
	}
	simplified, e := m.NewRef(ast.ASTHandle{})
	if e != nil {
		return false, e
	}
	if e = c.simplify.Rewrite(combined, simplified); e != nil {
		return false, errors.Join(e, m.DelRef(&simplified))
	}
	if e = m.AssignRef(body, simplified); e != nil {
		return false, errors.Join(e, m.DelRef(&simplified))
	}
	if e = m.DelRef(&simplified); e != nil {
		return false, e
	}
	c.eliminated++
	return true, nil
}

func (t *ElimSmallBVTactic) Apply(g *tactic.Goal, r *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || r == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	t.running = true
	defer func() { t.running = false }()
	if err = r.Prepare(t.manager); err != nil {
		return err
	}
	// tactic_report report("elim-small-bv", *g) precedes the proof and core
	// checks.
	report, err := tactic.NewTacticReport("elim-small-bv", g)
	if err != nil {
		return err
	}
	reportOpen := true
	closeReport := func() {
		if reportOpen {
			reportOpen = false
			err = errors.Join(err, report.Close())
		}
	}
	defer closeReport()
	if err = tactic.FailIfProofGeneration("elim-small-bv", g); err != nil {
		return err
	}
	if err = tactic.FailIfUnsatCoreGeneration("elim-small-bv", g); err != nil {
		return err
	}
	key := "max_bits"
	bits, e := t.context.GetUIntName(&t.params, &key, 4)
	if e != nil {
		return e
	}
	key = "max_steps"
	steps, e := t.context.GetUIntName(&t.params, &key, math.MaxUint32)
	if e != nil {
		return e
	}
	key = "max_memory"
	mb, e := t.context.GetUIntName(&t.params, &key, math.MaxUint32)
	if e != nil {
		return e
	}
	cfg := &elimSmallBVConfig{owner: t, params: &t.params, maxBits: bits, maxSteps: steps, maxMemory: memory.MegabytesToBytes(mb)}
	cfg.substitute, e = rewriter.NewVarSubstitution(t.manager, true)
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, cfg.substitute.Close()) }()
	// C++ rw_cfg builds th_rewriter m_simp(_m) with the default (empty)
	// params; the tactic params reach only unused_vars_eliminator.
	cfg.simplify, e = rewriter.NewTheoryRewriter(t.manager, t.context, t.globals, nil)
	if e != nil {
		return e
	}
	trav, e := rewriter.NewTraversal(t.manager, cfg)
	if e != nil {
		return e
	}
	// The source's rw (its cache and rw_cfg::m_simp) lives in the tactic
	// until cleanup(): ~rw_cfg destroys m_simp before the rewriter_tpl
	// base, and both outlive ~tactic_report, while expr_ref new_curr
	// (out) dies before the report.
	defer func() {
		err = errors.Join(err, cfg.simplify.Close())
		err = errors.Join(err, trav.Close())
	}()
	defer closeReport()
	out, e := t.manager.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	size, e := g.Size()
	if e != nil {
		return e
	}
	for i := uint32(0); i < size; i++ {
		f, e := g.Form(i)
		if e != nil {
			return e
		}
		if e = trav.Rewrite(f, out); e != nil {
			return e
		}
		v, e := t.manager.RefGet(out)
		if e != nil {
			return e
		}
		d, e := g.Dependency(i)
		if e != nil {
			return e
		}
		if e = g.Update(i, v, ast.ASTHandle{}, d); e != nil {
			return e
		}
	}
	if e = tactic.ReportTacticProgress(":elim-small-bv-num-eliminated", cfg.eliminated); e != nil {
		return e
	}
	if e = g.IncDepth(); e != nil {
		return e
	}
	if e = r.AddGoal(g); e != nil {
		return e
	}
	tags := diagnostic.BorrowProcessTraceTags()
	e = tags.EmitConditional("elim-small-bv", "operator()", "src/tactic/bv/elim_small_bv_tactic.cpp", 278, func() bool { return true }, func(w io.Writer) error { return g.Display(w) })
	if e != nil && !tags.IsStreamFailure(e) {
		return e
	}
	return nil
}
