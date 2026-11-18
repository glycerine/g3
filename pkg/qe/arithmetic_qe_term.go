package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util term decomposition.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// ArithmeticQETermUtil is the dependency-closed arithmetic term and bound
// foundation used by the retained arithmetic QE plugin.
type ArithmeticQETermUtil struct {
	self    *ArithmeticQETermUtil
	manager *ast.ASTManager
	family  ast.FamilyID
	// theory, arith and boolRw are arith_qe_util's m_rewriter,
	// m_arith_rewriter and m_bool_rewriter (default parameters), owned when
	// the util is built with parameter services.
	theory *rewriter.TheoryRewriter
	arith  *rewriter.ArithRewriter
	boolRw *rewriter.BoolRewriter
	// context is the i_solver_context whose blast_or expands symbolic
	// Cooper disjunctions; nil outside a quant_elim_plugin.
	context *SolverContext
	closed  bool
}

// NewArithmeticQETermUtil builds arith_qe_util. With parameter services in
// config it owns its default th_rewriter, arith_rewriter and bool_rewriter;
// without them only the term decomposition helpers are available.
func NewArithmeticQETermUtil(manager *ast.ASTManager, config Config) (*ArithmeticQETermUtil, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	u := &ArithmeticQETermUtil{manager: manager, family: family}
	u.self = u
	if config.Context == nil {
		return u, nil
	}
	u.theory, err = rewriter.NewTheoryRewriter(manager, config.Context, config.Globals, nil)
	if err == nil {
		u.arith, err = rewriter.NewArithRewriter(manager, config.Context, config.Globals, nil)
	}
	if err == nil {
		u.boolRw, err = rewriter.NewBoolRewriter(manager, config.Context, config.Globals, nil)
	}
	if err != nil {
		return nil, errors.Join(err, u.Close())
	}
	return u, nil
}

func (u *ArithmeticQETermUtil) open() error {
	if u == nil || u.self != u || u.manager == nil {
		return ast.ErrASTHandle
	}
	if u.closed {
		return arena.ErrClosed
	}
	return nil
}

func (u *ArithmeticQETermUtil) Close() error {
	if u == nil || u.self != u {
		return ast.ErrASTHandle
	}
	if u.closed {
		return nil
	}
	var err error
	if u.boolRw != nil {
		err = errors.Join(err, u.boolRw.Close())
	}
	if u.arith != nil {
		err = errors.Join(err, u.arith.Close())
	}
	if u.theory != nil {
		err = errors.Join(err, u.theory.Close())
	}
	if err != nil {
		return err
	}
	u.theory = nil
	u.arith = nil
	u.boolRw = nil
	u.context = nil
	u.closed = true
	return nil
}

func (u *ArithmeticQETermUtil) operation(expression ast.ASTHandle, family ast.FamilyID, kind int32) ([]ast.ASTHandle, bool, error) {
	// is_app_of: a variable or quantifier (blast_or leaves an exists when
	// expr_quant_elim was interrupted) is no application.
	astKind, err := u.manager.GetKind(expression)
	if err != nil || astKind != ast.ASTApp {
		return nil, false, err
	}
	actualFamily, err := u.manager.GetFamilyID(expression)
	if err != nil {
		return nil, false, err
	}
	actualKind, err := u.manager.GetDeclKind(expression)
	if err != nil || actualFamily != family || actualKind != kind {
		return nil, false, err
	}
	count, err := u.manager.GetNumArgs(expression)
	if err != nil {
		return nil, false, err
	}
	args := make([]ast.ASTHandle, count)
	for i := range args {
		args[i], err = u.manager.GetArg(expression, uint32(i))
		if err != nil {
			return nil, false, err
		}
	}
	return args, true, nil
}

// q is rational's mpq manager: the AST manager's NumeralService().Manager.
func (u *ArithmeticQETermUtil) q() *unumeral.RationalManager {
	return u.manager.NumeralService().Manager
}

// numeral is m_arith.is_numeral(e, r): on a match the value is copied into
// out, a rational of q().
func (u *ArithmeticQETermUtil) numeral(expression ast.ASTHandle, out *unumeral.Rational) (bool, error) {
	_, matched, err := u.manager.ArithmeticNumeralRational(expression, out)
	return matched && err == nil, err
}

// isNumeral is m_arith.is_numeral(e).
func (u *ArithmeticQETermUtil) isNumeral(expression ast.ASTHandle) (bool, error) {
	return u.manager.IsArithmeticNumeral(expression)
}

func (u *ArithmeticQETermUtil) isInteger(expression ast.ASTHandle) (bool, error) {
	sort, err := u.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	family, err := u.manager.GetFamilyID(sort)
	if err != nil || family != u.family {
		return false, errors.Join(ast.ErrASTDomain, err)
	}
	kind, err := u.manager.GetDeclKind(sort)
	return kind == 1, err
}

func (u *ArithmeticQETermUtil) zero(expression ast.ASTHandle) (ast.ASTHandle, error) {
	integer, err := u.isInteger(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if integer {
		return u.manager.ArithmeticIntegerNumeralText("0")
	}
	return u.manager.ArithmeticRealNumeralText("0")
}

// numeralFor is m_arith.mk_numeral(value, is_int(expression)).
func (u *ArithmeticQETermUtil) numeralFor(value *unumeral.Rational, expression ast.ASTHandle) (ast.ASTHandle, error) {
	if value == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	integer, err := u.isInteger(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if integer {
		isInt, err := u.q().IsInt(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !isInt {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
	}
	return u.manager.ArithmeticNumeralFromRational(value, integer)
}

// numeralForInt is numeralFor of the small integer n.
func (u *ArithmeticQETermUtil) numeralForInt(n int32, expression ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	q := u.q()
	value := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&value)) }()
	if err := q.SetInt32(&value, n); err != nil {
		return ast.ASTHandle{}, err
	}
	return u.numeralFor(&value, expression)
}

func (u *ArithmeticQETermUtil) sum(terms []ast.ASTHandle, sortWitness ast.ASTHandle) (result ast.ASTRef, err error) {
	if len(terms) == 0 {
		zero, cause := u.zero(sortWitness)
		if cause != nil {
			return result, cause
		}
		return u.manager.NewRef(zero)
	}
	if len(terms) == 1 {
		return u.manager.NewRef(terms[0])
	}
	// rest = m_arith.mk_add(restl.size(), restl.c_ptr()): one n-ary node
	// (AUDIT.md qe_01-F5).
	sum, err := u.manager.MkFamilyApp(u.family, 6, nil, terms)
	if err != nil {
		return result, err
	}
	return u.manager.NewRef(sum)
}

// Coefficient is get_coeff: it matches p = k*x + rest using the source LIFO
// flattening order, writing k into coefficient (a rational of q(); like the
// source's k reference it may be written even when there is no match).
// Rest is an owning reference when matched.
func (u *ArithmeticQETermUtil) Coefficient(variable *ast.ContainsApp, polynomial ast.ASTHandle, coefficient *unumeral.Rational) (rest ast.ASTRef, matched bool, err error) {
	if err = u.open(); err != nil {
		return rest, false, err
	}
	if variable == nil || coefficient == nil {
		return rest, false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return rest, false, err
	}
	todo := []ast.ASTHandle{polynomial}
	remainder := make([]ast.ASTHandle, 0, 4)
	for len(todo) != 0 {
		expression := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if args, add, cause := u.operation(expression, u.family, 6); cause != nil {
			return rest, false, cause
		} else if add {
			todo = append(todo, args...)
			continue
		}
		if expression == target {
			if err = u.q().SetInt32(coefficient, 1); err != nil {
				return rest, false, err
			}
			matched = true
			break
		}
		args, multiply, cause := u.operation(expression, u.family, 9)
		if cause != nil {
			return rest, false, cause
		}
		if multiply && len(args) == 2 {
			if args[0] == target {
				if numeral, cause := u.numeral(args[1], coefficient); cause != nil {
					return rest, false, cause
				} else if numeral {
					matched = true
					break
				}
			}
			if args[1] == target {
				if numeral, cause := u.numeral(args[0], coefficient); cause != nil {
					return rest, false, cause
				} else if numeral {
					matched = true
					break
				}
			}
		}
		remainder = append(remainder, expression)
	}
	if !matched {
		return rest, false, nil
	}
	for len(todo) != 0 {
		remainder = append(remainder, todo[len(todo)-1])
		todo = todo[:len(todo)-1]
	}
	rest, err = u.sum(remainder, target)
	if err != nil {
		return rest, false, err
	}
	restHandle, err := u.manager.RefGet(rest)
	if err != nil {
		return rest, false, errors.Join(err, u.manager.DelRef(&rest))
	}
	contains, err := variable.Check(restHandle)
	if err != nil || contains {
		return rest, false, errors.Join(err, u.manager.DelRef(&rest))
	}
	return rest, true, nil
}

// Constant extracts the first numeral in source LIFO order into constant (a
// rational of q(), zero when there is none) and owns rest.
func (u *ArithmeticQETermUtil) Constant(polynomial ast.ASTHandle, constant *unumeral.Rational) (rest ast.ASTRef, err error) {
	if err = u.open(); err != nil {
		return rest, err
	}
	todo := []ast.ASTHandle{polynomial}
	remainder := make([]ast.ASTHandle, 0, 4)
	if err = u.q().SetInt32(constant, 0); err != nil {
		return rest, err
	}
	for len(todo) != 0 {
		expression := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if args, add, cause := u.operation(expression, u.family, 6); cause != nil {
			return rest, cause
		} else if add {
			todo = append(todo, args...)
			continue
		}
		numeral, cause := u.numeral(expression, constant)
		if cause != nil {
			return rest, cause
		}
		if numeral {
			break
		}
		remainder = append(remainder, expression)
	}
	for len(todo) != 0 {
		remainder = append(remainder, todo[len(todo)-1])
		todo = todo[:len(todo)-1]
	}
	rest, err = u.sum(remainder, polynomial)
	return rest, err
}
