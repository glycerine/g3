package qe

// Source: src/qe/qe_arith.cpp/.h arith_project_plugin.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/insdict"
)

// ArithmeticProjection linearizes model-selected arithmetic constraints for
// the existing concrete simplex.ModelBasedOpt algorithm. Config and manager
// are borrowed. Mutable variable/formula vectors remain caller-owned.
type ArithmeticProjection struct {
	self    *ArithmeticProjection
	manager *ast.ASTManager
	config  Config
	family  ast.FamilyID
	pending []*arithmeticProjectionCall
	bytes   uint64
	closed  bool
}
type arithmeticProjectionCall struct {
	owner *ArithmeticProjection
	refs  []ast.ASTRef
	// pinned is operator()'s expr_ref_vector of linearized literals; its
	// destructor releases them front to back, before tids, mbo and eval.
	pinned    []ast.ASTRef
	vectors   []*[]ast.ASTRef
	evaluator *model.ModelEvaluator
	mark      *ast.ASTExprMark
	mbo       *simplex.ModelBasedOpt
	ids       *insdict.Dict[ast.ASTHandle, uint32]
	failures  []memory.ConstructionFailure
}
type ArithmeticProjectionFailure struct {
	owner          *ArithmeticProjection
	cause, cleanup error
}

func (e *ArithmeticProjectionFailure) Error() string        { return e.Unwrap().Error() }
func (e *ArithmeticProjectionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *ArithmeticProjectionFailure) OriginalCause() error { return e.cause }
func (e *ArithmeticProjectionFailure) CleanupCause() error  { return e.cleanup }
func (e *ArithmeticProjectionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *ArithmeticProjectionFailure) Cleanup() error { return e.owner.drain() }

func NewArithmeticProjection(m *ast.ASTManager, config Config) (*ArithmeticProjection, error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err := config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(ArithmeticProjection{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	p := &ArithmeticProjection{manager: m, config: config, family: family, bytes: bytes}
	p.self = p
	return p, nil
}
func (p *ArithmeticProjection) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	_, err := p.manager.ProofsEnabled()
	return err
}
func (p *ArithmeticProjection) FamilyID() (ast.FamilyID, error) {
	if err := p.open(); err != nil {
		return ast.NullFamilyID, err
	}
	return p.family, nil
}
func projectionHandleHash(h ast.ASTHandle) uint64 {
	return h.Origin() ^ uint64(h.Slot())*0x9e3779b97f4a7c15 ^ uint64(h.Generation())*0xbf58476d1ce4e5b9
}

// projectionTermMap is linearize's obj_map<expr, rational> ts:
// extract_coefficients walks it in hash-table order, which fixes the order
// in which mbo variables are created and coefficients are listed.
// The values are rationals of the manager's NumeralService().Manager owned
// by the map (release destroys them, as the obj_map's destructor does).
type projectionTermMap struct {
	q      *unumeral.RationalManager
	table  *ast.ObjMap
	values []*unumeral.Rational
}

func projectionTerms(m *ast.ASTManager) *projectionTermMap {
	return &projectionTermMap{q: m.NumeralService().Manager, table: ast.NewObjMap(m)}
}

// release destroys the map's rationals.
func (t *projectionTermMap) release() (err error) {
	for _, v := range t.values {
		err = errors.Join(err, t.q.Del(v))
	}
	t.values = nil
	return err
}

// get is ts.find(x, w): the stored rational itself.
func (t *projectionTermMap) get(h ast.ASTHandle) (*unumeral.Rational, bool, error) {
	i, ok, err := t.table.FindNum(h)
	if err != nil || !ok {
		return nil, false, err
	}
	return t.values[i], true, nil
}

// set is ts.insert(x, v): an existing key keeps its slot; v is copied.
func (t *projectionTermMap) set(h ast.ASTHandle, v *unumeral.Rational) error {
	if i, ok, err := t.table.FindNum(h); err != nil {
		return err
	} else if ok {
		return t.q.Copy(t.values[i], v)
	}
	stored := new(unumeral.Rational)
	*stored = unumeral.NewRational()
	if err := t.q.Copy(stored, v); err != nil {
		return errors.Join(err, t.q.Del(stored))
	}
	t.values = append(t.values, stored)
	return t.table.InsertNum(h, uint64(len(t.values)-1))
}

// entries lists (key, value) in table order.
func (t *projectionTermMap) entries() ([]ast.ASTHandle, []*unumeral.Rational) {
	es := t.table.Entries()
	keys := make([]ast.ASTHandle, len(es))
	values := make([]*unumeral.Rational, len(es))
	for i, e := range es {
		keys[i], values[i] = e.Key, t.values[e.Num]
	}
	return keys, values
}
func (p *ArithmeticProjection) begin(md *model.ModelCore) (*arithmeticProjectionCall, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if md == nil || md.Manager() != p.manager {
		return nil, ast.ErrASTHandle
	}
	if err := p.drain(); err != nil {
		return nil, err
	}
	c := &arithmeticProjectionCall{owner: p, mbo: simplex.NewModelBasedOpt(), ids: insdict.NewDictFunc[ast.ASTHandle, uint32](projectionHandleHash)}
	p.pending = append(p.pending, c)
	return c, nil
}
func (c *arithmeticProjectionCall) startEvaluator(md *model.ModelCore) error {
	p := c.owner
	var err error
	c.evaluator, err = model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace})
	return err
}
func (c *arithmeticProjectionCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return h, err
}
func (c *arithmeticProjectionCall) retain(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		c.failures = append(c.failures, failure)
	}
}
func (c *arithmeticProjectionCall) close() error {
	for len(c.failures) != 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	for len(c.pinned) != 0 {
		if c.pinned[0] != (ast.ASTRef{}) {
			if err := c.owner.manager.DelRef(&c.pinned[0]); err != nil {
				c.retain(err)
				return err
			}
		}
		c.pinned = c.pinned[1:]
	}
	if c.evaluator != nil {
		if err := c.evaluator.Close(); err != nil {
			c.retain(err)
			return err
		}
		c.evaluator = nil
	}
	if c.mark != nil {
		if err := c.mark.Close(); err != nil {
			c.retain(err)
			return err
		}
		c.mark = nil
	}
	for len(c.vectors) != 0 {
		i := len(c.vectors) - 1
		if err := c.owner.shrink(c.vectors[i], 0); err != nil {
			c.retain(err)
			return err
		}
		c.vectors = c.vectors[:i]
	}
	for len(c.refs) != 0 {
		i := len(c.refs) - 1
		if c.refs[i] != (ast.ASTRef{}) {
			if err := c.owner.manager.DelRef(&c.refs[i]); err != nil {
				c.retain(err)
				return err
			}
		}
		c.refs = c.refs[:i]
	}
	c.mbo = nil
	c.ids = nil
	return nil
}
func (p *ArithmeticProjection) drain() error {
	for len(p.pending) != 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *ArithmeticProjection) finish(c *arithmeticProjectionCall, err *error) {
	c.retain(*err)
	if cleanup := p.drain(); cleanup != nil {
		*err = &ArithmeticProjectionFailure{owner: p, cause: *err, cleanup: cleanup}
	}
}
func (p *ArithmeticProjection) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if err := p.drain(); err != nil {
		return err
	}
	p.manager.Budget().Release(p.bytes)
	p.bytes = 0
	p.closed = true
	return nil
}
func (p *ArithmeticProjection) emit(function string, line int, body func(io.Writer) error) error {
	tags := p.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("qe", function, "src/qe/qe_arith.cpp", line, func() bool { return true }, func(w io.Writer) error { return body(traceBodyWriter{w}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (p *ArithmeticProjection) arithmetic(h ast.ASTHandle) (integer, arith bool, err error) {
	s, err := p.manager.GetSort(h)
	if err != nil {
		return false, false, err
	}
	fid, err := p.manager.GetFamilyID(s)
	if err != nil || fid != p.family {
		return false, false, err
	}
	k, err := p.manager.GetDeclKind(s)
	return k == 1, k == 0 || k == 1, err
}
func (p *ArithmeticProjection) operation(h ast.ASTHandle, fid ast.FamilyID, k int32) ([]ast.ASTHandle, bool, error) {
	kind, err := p.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return nil, false, err
	}
	f, err := p.manager.GetFamilyID(h)
	if err != nil || f != fid {
		return nil, false, err
	}
	op, err := p.manager.GetDeclKind(h)
	if err != nil || op != k {
		return nil, false, err
	}
	n, err := p.manager.GetNumArgs(h)
	if err != nil {
		return nil, false, err
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		args[i], err = p.manager.GetArg(h, uint32(i))
		if err != nil {
			return nil, false, err
		}
	}
	return args, true, nil
}
func (p *ArithmeticProjection) appendRef(v *[]ast.ASTRef, h ast.ASTHandle) error {
	r, err := p.manager.NewRef(h)
	if err == nil {
		*v = append(*v, r)
	}
	return err
}
func (p *ArithmeticProjection) shrink(v *[]ast.ASTRef, n int) error {
	for len(*v) > n {
		i := len(*v) - 1
		if (*v)[i] != (ast.ASTRef{}) {
			if err := p.manager.DelRef(&(*v)[i]); err != nil {
				return err
			}
		}
		*v = (*v)[:i]
	}
	return nil
}

// evalLocal is `expr_ref val = eval(h)` for a C++ local: the caller releases
// the returned reference where the local dies.
func (c *arithmeticProjectionCall) evalLocal(h ast.ASTHandle) (ast.ASTRef, ast.ASTHandle, error) {
	m := c.owner.manager
	r, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, err
	}
	if err = c.evaluator.Evaluate(h, r); err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, errors.Join(err, m.DelRef(&r))
	}
	v, err := m.RefGet(r)
	if err != nil {
		return ast.ASTRef{}, ast.ASTHandle{}, errors.Join(err, m.DelRef(&r))
	}
	return r, v, nil
}

func (c *arithmeticProjectionCall) eval(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.owner.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.refs = append(c.refs, r)
	if err = c.evaluator.Evaluate(h, r); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.owner.manager.RefGet(r)
}

// q is rational's mpq manager: the AST manager's NumeralService().Manager.
func (p *ArithmeticProjection) q() *unumeral.RationalManager {
	return p.manager.NumeralService().Manager
}

// numeral is a.is_numeral(h, out).
func (p *ArithmeticProjection) numeral(h ast.ASTHandle, out *unumeral.Rational) (bool, error) {
	_, ok, err := p.manager.ArithmeticNumeralRational(h, out)
	return ok && err == nil, err
}

// Source's syntactic numeral recognizer includes products, sums, unary minus
// and binary subtraction; division and coercions remain ordinary terms. The
// value is written into out (a rational of q()).
func (p *ArithmeticProjection) numeralExpression(h ast.ASTHandle, out *unumeral.Rational) (_ bool, resultErr error) {
	ok, err := p.numeral(h, out)
	if err != nil || ok {
		return ok, err
	}
	q := p.q()
	for _, op := range []int32{8, 9, 6, 7} {
		args, matched, err := p.operation(h, p.family, op)
		if err != nil {
			return false, err
		}
		if !matched {
			continue
		}
		if (op == 8 && len(args) != 1) || (op == 7 && len(args) != 2) {
			return false, nil
		}
		v := unumeral.NewRational()
		defer func() { resultErr = errors.Join(resultErr, q.Del(&v)) }()
		start := int32(0)
		if op == 9 {
			start = 1
		}
		if err = q.SetInt32(out, start); err != nil {
			return false, err
		}
		for i, arg := range args {
			ok, err := p.numeralExpression(arg, &v)
			if err != nil || !ok {
				return false, err
			}
			switch op {
			case 8:
				err = errors.Join(q.Copy(out, &v), q.Neg(out))
			case 9:
				err = q.Mul(out, &v, out)
			case 6:
				err = q.Add(out, &v, out)
			case 7:
				if i == 0 {
					err = q.Copy(out, &v)
				} else {
					err = q.Sub(out, &v, out)
				}
			}
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}
func (c *arithmeticProjectionCall) id(h ast.ASTHandle) (uint32, error) {
	return c.idTrace(h, nil)
}

// idTrace is tids.insert(v, mbo.add_var(r, a.is_int(v))) for a term not in
// tids yet: `expr_ref val = eval(v)` is local to the step, and trace sees
// the value while it lives. tids holds no reference to v.
func (c *arithmeticProjectionCall) idTrace(h ast.ASTHandle, trace func(value ast.ASTHandle) error) (id uint32, err error) {
	if id, ok := c.ids.Get2(h); ok {
		return id, nil
	}
	m := c.owner.manager
	val, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&val)) }()
	if err = c.evaluator.Evaluate(h, val); err != nil {
		return 0, err
	}
	v, err := m.RefGet(val)
	if err != nil {
		return 0, err
	}
	// rational r;
	qm := c.owner.q()
	r := unumeral.NewRational()
	defer func() { err = errors.Join(err, qm.Del(&r)) }()
	ok, err := c.owner.numeral(v, &r)
	if err != nil {
		return 0, err
	}
	if !ok {
		// source leaves the default rational at zero
		if err = qm.SetInt32(&r, 0); err != nil {
			return 0, err
		}
	}
	if trace != nil {
		if err = trace(v); err != nil {
			return 0, err
		}
	}
	integer, _, err := c.owner.arithmetic(h)
	if err != nil {
		return 0, err
	}
	if id, err = mboAddVariable(c.mbo, qm, &r, integer); err != nil {
		return 0, err
	}
	c.ids.Set(h, id)
	return id, nil
}
func (c *arithmeticProjectionCall) coefficients(terms *projectionTermMap) ([]simplex.LinearTerm, error) {
	if err := c.evaluator.SetModelCompletion(true); err != nil {
		return nil, err
	}
	var result []simplex.LinearTerm
	keys, values := terms.entries()
	for i, h := range keys {
		id, err := c.id(h)
		if err != nil {
			return nil, err
		}
		zero, err := terms.q.IsZero(values[i])
		if err != nil {
			return nil, err
		}
		if !zero {
			term, err := mboLinearTerm(terms.q, id, values[i])
			if err != nil {
				return nil, err
			}
			result = append(result, term)
		} else {
			if err := c.owner.emit("extract_coefficients", 520, func(w io.Writer) error {
				if e := writeExpression(c.owner.manager, c.owner.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, " has coefficeint 0\n")
				return nil
			}); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// Solve is deliberately the source cheap-solve no-op. Projection is performed
// by Project/ProjectVariable, rather than silently extending this hook.
func (p *ArithmeticProjection) Solve(md *model.ModelCore, vars, lits *[]ast.ASTRef) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if md == nil || md.Manager() != p.manager || vars == nil || lits == nil {
		return false, ast.ErrASTHandle
	}
	return false, nil
}
