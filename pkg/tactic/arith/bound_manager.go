package arith

// Source: src/tactic/arith/bound_manager.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type managerBound struct {
	value      *unumeral.Rational // a rational of the manager's NumeralService().Manager
	strict     bool
	dependency ast.ASTDependency // borrowed, exactly as in the source maps
}

// BoundManager owns one reference to each bounded expression. Dependencies
// remain borrowed from the source goals, which must outlive their use.
type BoundManager struct {
	self              *BoundManager
	manager           *ast.ASTManager
	context           *uparams.ParamsContext
	globals           *uparams.GlobalParamsService
	family            ast.FamilyID
	lowers, uppers    map[ast.ASTHandle]managerBound
	variables         []ast.ASTHandle
	trace             *diagnostic.TraceTags
	ownerBytes        uint64
	released          int
	resetting, closed bool
}

func NewBoundManager(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*BoundManager, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(BoundManager{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	b := &BoundManager{manager: m, context: c, globals: g, family: family, ownerBytes: bytes, lowers: make(map[ast.ASTHandle]managerBound), uppers: make(map[ast.ASTHandle]managerBound)}
	b.self = b
	return b, nil
}
func (b *BoundManager) owner() error {
	if b == nil || b.self != b {
		return ast.ErrASTHandle
	}
	if b.closed {
		return arena.ErrClosed
	}
	return nil
}
func (b *BoundManager) open() error {
	if err := b.owner(); err != nil {
		return err
	}
	if b.resetting {
		return ast.ErrASTDomain
	}
	return nil
}
func (b *BoundManager) Manager() *ast.ASTManager {
	if b == nil || b.open() != nil {
		return nil
	}
	return b.manager
}
func (b *BoundManager) Variables() ([]ast.ASTHandle, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), b.variables...), nil
}

// q is rational's mpq manager: the AST manager's NumeralService().Manager.
func (b *BoundManager) q() *unumeral.RationalManager { return b.manager.NumeralService().Manager }

// Lower is has_lower(m, l, strict): l is copied into value (a rational of
// the manager's NumeralService().Manager) when found.
func (b *BoundManager) Lower(term ast.ASTHandle, value *unumeral.Rational) (strict, found bool, err error) {
	if err = b.open(); err != nil {
		return
	}
	bound, found := b.lowers[term]
	if found {
		if err = b.q().Copy(value, bound.value); err != nil {
			return false, false, err
		}
		strict = bound.strict
	}
	return strict, found, nil
}

// Upper is has_upper(m, u, strict).
func (b *BoundManager) Upper(term ast.ASTHandle, value *unumeral.Rational) (strict, found bool, err error) {
	if err = b.open(); err != nil {
		return
	}
	bound, found := b.uppers[term]
	if found {
		if err = b.q().Copy(value, bound.value); err != nil {
			return false, false, err
		}
		strict = bound.strict
	}
	return strict, found, nil
}
func (b *BoundManager) HasLower(term ast.ASTHandle) (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	_, found := b.lowers[term]
	return found, nil
}
func (b *BoundManager) HasUpper(term ast.ASTHandle) (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	_, found := b.uppers[term]
	return found, nil
}
func (b *BoundManager) LowerDependency(term ast.ASTHandle) (ast.ASTDependency, error) {
	if err := b.open(); err != nil {
		return ast.ASTDependency{}, err
	}
	return b.lowers[term].dependency, nil
}
func (b *BoundManager) UpperDependency(term ast.ASTHandle) (ast.ASTDependency, error) {
	if err := b.open(); err != nil {
		return ast.ASTDependency{}, err
	}
	return b.uppers[term].dependency, nil
}
func (b *BoundManager) insert(term ast.ASTHandle, lower, strict bool, value *unumeral.Rational, dependency ast.ASTDependency) error {
	bounds, opposite := b.uppers, b.lowers
	if lower {
		bounds, opposite = b.lowers, b.uppers
	}
	q := b.q()
	old, found := bounds[term]
	if found {
		cmp, err := q.Compare(value, old.value)
		if err != nil {
			return err
		}
		if !(lower && cmp > 0 || !lower && cmp < 0 || cmp == 0 && strict && !old.strict) {
			return nil
		}
	} else if _, exists := opposite[term]; !exists {
		if err := b.manager.IncRef(term); err != nil {
			return err
		}
		b.variables = append(b.variables, term)
	}
	var next managerBound
	// m_lowers.insert(v, n) overwrites the stored numeral.
	next.value = old.value
	if next.value == nil {
		next.value = new(unumeral.Rational)
		*next.value = unumeral.NewRational()
	}
	if err := q.Copy(next.value, value); err != nil {
		if old.value == nil {
			err = errors.Join(err, q.Del(next.value))
		}
		return err
	}
	next.strict = strict
	next.dependency = old.dependency
	if !dependency.IsNull() {
		next.dependency = dependency
	}
	bounds[term] = next
	return nil
}
func (b *BoundManager) Reset() error {
	if err := b.owner(); err != nil {
		return err
	}
	b.resetting = true
	for b.released < len(b.variables) {
		consumed, err := b.manager.DecRefConsumed(b.variables[b.released])
		if consumed {
			b.released++
		}
		if err != nil {
			return err
		}
	}
	b.variables = nil
	b.released = 0
	q := b.q()
	for _, bounds := range []map[ast.ASTHandle]managerBound{b.lowers, b.uppers} {
		for term, bound := range bounds {
			if err := q.Del(bound.value); err != nil {
				return err
			}
			delete(bounds, term)
		}
	}
	b.resetting = false
	return nil
}
func (b *BoundManager) Close() error {
	if b == nil || b.self != b {
		return ast.ErrASTHandle
	}
	if b.closed {
		return nil
	}
	if err := b.Reset(); err != nil {
		return err
	}
	b.manager.Budget().Release(b.ownerBytes)
	b.ownerBytes = 0
	b.closed = true
	return nil
}

func (b *BoundManager) app(term ast.ASTHandle) (family ast.FamilyID, op int32, args []ast.ASTHandle, app bool, err error) {
	kind, err := b.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return family, op, nil, false, err
	}
	family, err = b.manager.GetFamilyID(term)
	if err != nil {
		return family, op, nil, false, err
	}
	op, err = b.manager.GetDeclKind(term)
	if err != nil {
		return family, op, nil, false, err
	}
	count, err := b.manager.GetNumArgs(term)
	if err != nil {
		return family, op, nil, false, err
	}
	args = make([]ast.ASTHandle, count)
	for i := range args {
		args[i], err = b.manager.GetArg(term, uint32(i))
		if err != nil {
			return family, op, nil, false, err
		}
	}
	return family, op, args, true, nil
}
func (b *BoundManager) constant(term ast.ASTHandle) (bool, error) {
	family, _, args, app, err := b.app(term)
	return app && family == ast.NullFamilyID && len(args) == 0, err
}

// numeral is bound_manager::is_numeral(v, n, is_int), writing n (a rational
// of q()).
func (b *BoundManager) numeral(term ast.ASTHandle, value *unumeral.Rational) (integer, found bool, err error) {
	negative := false
	for {
		family, op, args, app, e := b.app(term)
		if e != nil {
			err = e
			return
		}
		if !app || family != b.family || op != 8 || len(args) != 1 {
			break
		}
		negative = !negative
		term = args[0]
	}
	integer, found, err = b.manager.ArithmeticNumeralRational(term, value)
	if err != nil || !found {
		return integer, found, err
	}
	if negative {
		if err = b.q().Neg(value); err != nil {
			return false, false, err
		}
	}
	return integer, true, nil
}
func boundSwap(op int32) (int32, error) {
	switch op {
	case 2:
		return 3, nil
	case 3:
		return 2, nil
	case 4:
		return 5, nil
	case 5:
		return 4, nil
	}
	return op, ast.ErrASTDomain
}
func boundNeg(op int32) (int32, error) {
	switch op {
	case 2:
		return 5, nil
	case 3:
		return 4, nil
	case 4:
		return 3, nil
	case 5:
		return 2, nil
	}
	return op, ast.ErrASTDomain
}

// boundNormalizeInteger is norm(n, k) for an integer bound.
func boundNormalizeInteger(q *unumeral.RationalManager, value *unumeral.Rational, op int32) (int32, error) {
	if op == 4 {
		return 2, q.Dec(value)
	}
	if op == 5 {
		return 3, q.Inc(value)
	}
	return op, nil
}

// equality is is_equality(e, x, n, is_int) writing n into value.
func (b *BoundManager) equality(form ast.ASTHandle, value *unumeral.Rational) (variable ast.ASTHandle, integer, found bool, err error) {
	family, op, args, app, err := b.app(form)
	if err != nil || !app || family != b.manager.BasicFamilyID() || op != ast.ASTOpEq {
		return variable, false, false, err
	}
	if len(args) != 2 {
		return variable, false, false, ast.ErrASTDomain
	}
	x, y := args[0], args[1]
	constant, e := b.constant(x)
	if e != nil {
		return variable, false, false, e
	}
	if !constant {
		x, y = y, x
	}
	constant, e = b.constant(x)
	if e != nil || !constant {
		return variable, false, false, e
	}
	integer, found, err = b.numeral(y, value)
	return x, integer, found, err
}
func (b *BoundManager) disjunction(form ast.ASTHandle, dependency ast.ASTDependency) (_ bool, resultErr error) {
	family, op, args, app, err := b.app(form)
	if err != nil || !app || family != b.manager.BasicFamilyID() || op != ast.ASTOpOr || len(args) == 0 {
		return false, err
	}
	var variable ast.ASTHandle
	// rational lo, hi, n;
	q := b.q()
	low, high, n := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&low), q.Del(&high), q.Del(&n)) }()
	for _, eq := range args {
		x, integer, found, err := b.equality(eq, &n)
		if err != nil || !found || !integer {
			return false, err
		}
		if !variable.IsNull() && x != variable {
			return false, nil
		}
		if variable.IsNull() {
			variable = x
			if err = errors.Join(q.Copy(&low, &n), q.Copy(&high, &n)); err != nil {
				return false, err
			}
		}
		if c, err := q.Compare(&n, &low); err != nil {
			return false, err
		} else if c < 0 {
			if err = q.Copy(&low, &n); err != nil {
				return false, err
			}
		}
		if c, err := q.Compare(&n, &high); err != nil {
			return false, err
		} else if c > 0 {
			if err = q.Copy(&high, &n); err != nil {
				return false, err
			}
		}
	}
	if err = b.traceBounds(&low, &high); err != nil {
		return false, err
	}
	if err = b.insert(variable, true, false, &low, dependency); err != nil {
		return false, err
	}
	return true, b.insert(variable, false, false, &high, dependency)
}
func (b *BoundManager) Collect(form ast.ASTHandle, dependency ast.ASTDependency) (resultErr error) {
	if err := b.open(); err != nil {
		return err
	}
	// numeral n;
	q := b.q()
	value := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&value)) }()
	if err := b.traceProcessing(form); err != nil {
		return err
	}
	if found, err := b.disjunction(form, dependency); err != nil || found {
		return err
	}
	variable, _, found, err := b.equality(form, &value)
	if err != nil {
		return err
	}
	if found {
		if err = b.insert(variable, true, false, &value, dependency); err != nil {
			return err
		}
		return b.insert(variable, false, false, &value, dependency)
	}
	positive := true
	for {
		family, op, args, app, e := b.app(form)
		if e != nil {
			return e
		}
		if !app || family != b.manager.BasicFamilyID() || op != ast.ASTOpNot || len(args) != 1 {
			break
		}
		positive = !positive
		form = args[0]
	}
	family, op, args, app, err := b.app(form)
	if err != nil || !app || family != b.family || op < 2 || op > 5 {
		return err
	}
	if len(args) < 2 {
		return ast.ErrASTDomain
	}
	x, y := args[0], args[1]
	constant, err := b.constant(x)
	if err != nil {
		return err
	}
	var integer bool
	if constant {
		integer, found, err = b.numeral(y, &value)
	}
	if err != nil {
		return err
	}
	if !constant || !found {
		constant, err = b.constant(y)
		if err != nil || !constant {
			return err
		}
		integer, found, err = b.numeral(x, &value)
		if err != nil || !found {
			return err
		}
		x = y
		op, err = boundSwap(op)
		if err != nil {
			return err
		}
	}
	if !positive {
		op, err = boundNeg(op)
		if err != nil {
			return err
		}
	}
	if integer {
		if op, err = boundNormalizeInteger(q, &value, op); err != nil {
			return err
		}
	}
	if err = b.traceFound(x); err != nil {
		return err
	}
	return b.insert(x, op == 3 || op == 5, op == 4 || op == 5, &value, dependency)
}
func (b *BoundManager) CollectGoal(goal *tactic.Goal) error {
	if err := b.open(); err != nil {
		return err
	}
	if goal == nil || goal.Manager() != b.manager {
		return ast.ErrASTHandle
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		form, e := goal.Form(i)
		if e != nil {
			return e
		}
		dep, e := goal.Dependency(i)
		if e != nil {
			return e
		}
		if err = b.Collect(form, dep); err != nil {
			return err
		}
	}
	return nil
}
