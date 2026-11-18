package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::is_linear overloads.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type legacyArithmeticProjection struct {
	syntax *ArithmeticProjection
	theory *rewriter.TheoryRewriter
}

type legacyLinearLiteral struct {
	manager     *ast.ASTManager
	term        ast.ASTRef
	coefficient unumeral.Rational // m_coeffs entry
	divisor     unumeral.Rational // m_divs entry (zero: no divisibility)
	strict      bool
	equality    bool
	disequality bool
}

type legacyArithmeticProjectionConstructionFailure struct {
	owner          *legacyArithmeticProjection
	runtime        *memory.Runtime
	cause, cleanup error
}

var _ memory.ConstructionFailure = (*legacyArithmeticProjectionConstructionFailure)(nil)

func (f *legacyArithmeticProjectionConstructionFailure) Error() string { return f.Unwrap().Error() }
func (f *legacyArithmeticProjectionConstructionFailure) Unwrap() error {
	return errors.Join(f.cause, f.cleanup)
}
func (f *legacyArithmeticProjectionConstructionFailure) OriginalCause() error { return f.cause }
func (f *legacyArithmeticProjectionConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *legacyArithmeticProjectionConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.runtime
}
func (f *legacyArithmeticProjectionConstructionFailure) Cleanup() error {
	if f.owner == nil {
		f.cleanup = nil
		return nil
	}
	f.cleanup = f.owner.Close()
	if f.cleanup == nil {
		f.owner = nil
	}
	return f.cleanup
}

func closeFailedLegacyArithmeticProjection(owner *legacyArithmeticProjection, cause error) error {
	if cleanup := owner.Close(); cleanup != nil {
		return &legacyArithmeticProjectionConstructionFailure{owner: owner, runtime: owner.syntax.manager.NumeralService().RuntimeOwner(), cause: cause, cleanup: cleanup}
	}
	return cause
}

func newLegacyArithmeticProjection(manager *ast.ASTManager, config Config) (*legacyArithmeticProjection, error) {
	syntax, err := NewArithmeticProjection(manager, config)
	if err != nil {
		return nil, err
	}
	theory, err := rewriter.NewTheoryRewriter(manager, config.Context, config.Globals, nil)
	if err != nil {
		return nil, closeFailedLegacyArithmeticProjection(&legacyArithmeticProjection{syntax: syntax, theory: theory}, err)
	}
	return &legacyArithmeticProjection{syntax: syntax, theory: theory}, nil
}

func (p *legacyArithmeticProjection) Close() error {
	if p == nil || p.syntax == nil {
		return ast.ErrASTHandle
	}
	var err error
	if p.theory != nil {
		err = p.theory.Close()
	}
	if err == nil && p.theory != nil {
		p.theory = nil
	}
	if err == nil {
		err = p.syntax.Close()
	}
	if err == nil {
		p.syntax = nil
	}
	return err
}

func (l *legacyLinearLiteral) Close() error {
	if l == nil || l.manager == nil {
		return ast.ErrASTHandle
	}
	q := l.manager.NumeralService().Manager
	err := errors.Join(l.manager.DelRef(&l.term), q.Del(&l.coefficient), q.Del(&l.divisor))
	if err == nil {
		l.manager = nil
	}
	return err
}

// q is rational's mpq manager: the AST manager's NumeralService().Manager.
func (p *legacyArithmeticProjection) q() *unumeral.RationalManager { return p.syntax.q() }

// sign is r's sign.
func (p *legacyArithmeticProjection) sign(r *unumeral.Rational) (int32, error) {
	// r.is_pos()/r.is_neg(): RationalManager.Sign returns the raw small value.
	return int32(r.Sign()), nil
}

// multiplyLegacyAbs is mk_mul(abs(r), term).
func (p *legacyArithmeticProjection) multiplyLegacyAbs(r *unumeral.Rational, term ast.ASTHandle) (_ ast.ASTHandle, resultErr error) {
	q := p.q()
	abs := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&abs)) }()
	if err := errors.Join(q.Copy(&abs, r), q.Abs(&abs)); err != nil {
		return ast.ASTHandle{}, err
	}
	return p.multiplyLegacy(&abs, term)
}

// multiplyLegacyInt is mk_mul(rational(n), term).
func (p *legacyArithmeticProjection) multiplyLegacyInt(n int32, term ast.ASTHandle) (_ ast.ASTHandle, resultErr error) {
	q := p.q()
	r := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&r)) }()
	if err := q.SetInt32(&r, n); err != nil {
		return ast.ASTHandle{}, err
	}
	return p.multiplyLegacy(&r, term)
}

func (p *legacyArithmeticProjection) contains(root, variable ast.ASTHandle) (bool, error) {
	seen := make(map[ast.ASTHandle]bool)
	work := []ast.ASTHandle{root}
	for len(work) != 0 {
		index := len(work) - 1
		current := work[index]
		work = work[:index]
		if current == variable {
			return true, nil
		}
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return false, err
		}
		for argument := uint32(0); argument < count; argument++ {
			handle, err := p.syntax.manager.GetArg(current, argument)
			if err != nil {
				return false, err
			}
			work = append(work, handle)
		}
	}
	return false, nil
}

func (p *legacyArithmeticProjection) scaledTerm(multiplier *unumeral.Rational, term ast.ASTHandle, integer bool) (ast.ASTHandle, error) {
	if one, err := p.q().IsOne(multiplier); err != nil {
		return ast.ASTHandle{}, err
	} else if one {
		return term, nil
	}
	numeral, err := p.syntax.manager.ArithmeticNumeralFromRational(multiplier, integer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return p.syntax.manager.MkArithmeticMul(numeral, term)
}

// linearTerm is arith_project_util::is_linear(mul, t, c, ts): multiplier and
// coefficient are rationals of q().
func (p *legacyArithmeticProjection) linearTerm(multiplier *unumeral.Rational, root, variable ast.ASTHandle, coefficient *unumeral.Rational, terms *[]ast.ASTHandle, integer bool) (_ bool, resultErr error) {
	q := p.q()
	if root == variable {
		// c += mul;
		return true, q.Add(coefficient, multiplier, coefficient)
	}
	// rational mul1;
	mul1, product := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&mul1), q.Del(&product)) }()
	args, matched, err := p.syntax.operation(root, p.syntax.family, 9)
	if err != nil {
		return false, err
	}
	if matched && len(args) == 2 {
		for index, argument := range args {
			numeral, err := p.syntax.numeral(argument, &mul1)
			if err != nil {
				return false, err
			}
			if numeral {
				if err = q.Mul(multiplier, &mul1, &product); err != nil {
					return false, err
				}
				return p.linearTerm(&product, args[1-index], variable, coefficient, terms, integer)
			}
		}
	}
	args, matched, err = p.syntax.operation(root, p.syntax.family, 6)
	if err != nil {
		return false, err
	}
	if matched {
		for _, argument := range args {
			ok, err := p.linearTerm(multiplier, argument, variable, coefficient, terms, integer)
			if err != nil || !ok {
				return ok, err
			}
		}
		return true, nil
	}
	args, matched, err = p.syntax.operation(root, p.syntax.family, 7)
	if err != nil {
		return false, err
	}
	if matched && len(args) == 2 {
		ok, err := p.linearTerm(multiplier, args[0], variable, coefficient, terms, integer)
		if err != nil || !ok {
			return ok, err
		}
		if err = errors.Join(q.Copy(&product, multiplier), q.Neg(&product)); err != nil {
			return false, err
		}
		return p.linearTerm(&product, args[1], variable, coefficient, terms, integer)
	}
	args, matched, err = p.syntax.operation(root, p.syntax.family, 8)
	if err != nil {
		return false, err
	}
	if matched && len(args) == 1 {
		if err = errors.Join(q.Copy(&product, multiplier), q.Neg(&product)); err != nil {
			return false, err
		}
		return p.linearTerm(&product, args[0], variable, coefficient, terms, integer)
	}
	numeral, err := p.syntax.numeral(root, &mul1)
	if err != nil {
		return false, err
	}
	if numeral {
		if err = q.Mul(multiplier, &mul1, &product); err != nil {
			return false, err
		}
		term, err := p.syntax.manager.ArithmeticNumeralFromRational(&product, integer)
		if err != nil {
			return false, err
		}
		*terms = append(*terms, term)
		return true, nil
	}
	contains, err := p.contains(root, variable)
	if err != nil || contains {
		return false, err
	}
	term, err := p.scaledTerm(multiplier, root, integer)
	if err != nil {
		return false, err
	}
	*terms = append(*terms, term)
	return true, nil
}

func (p *legacyArithmeticProjection) linearLiteral(variable, literal ast.ASTHandle) (_ *legacyLinearLiteral, resultErr error) {
	if p == nil || p.syntax == nil {
		return nil, ast.ErrASTHandle
	}
	integer, arithmetic, err := p.syntax.arithmetic(variable)
	if err != nil || !arithmetic {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	negative, inner, err := p.syntax.manager.IsNot(literal)
	if err != nil {
		return nil, err
	}
	if negative {
		literal = inner
	}
	q := p.q()
	// rational mul(1);
	multiplier, negMultiplier, divisor := unumeral.NewRational(), unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&multiplier), q.Del(&negMultiplier), q.Del(&divisor)) }()
	if err = q.SetInt32(&multiplier, 1); err != nil {
		return nil, err
	}
	if negative {
		if err = q.Neg(&multiplier); err != nil {
			return nil, err
		}
	}
	result := &legacyLinearLiteral{manager: p.syntax.manager, coefficient: unumeral.NewRational(), divisor: unumeral.NewRational()}
	// The result owns its rationals from here; a nil return releases them.
	defer func() {
		if result != nil && (resultErr != nil || result.term == (ast.ASTRef{})) {
			resultErr = errors.Join(resultErr, q.Del(&result.coefficient), q.Del(&result.divisor))
		}
	}()
	var left, right ast.ASTHandle
	matched := false
	// A divisibility literal (= (mod t k) 0) runs is_linear on t only
	// (AUDIT.md qe_02-F7).
	divisibility := false
	for _, operation := range []int32{2, 3, 4, 5} {
		args, yes, err := p.syntax.operation(literal, p.syntax.family, operation)
		if err != nil {
			return nil, err
		}
		if !yes || len(args) != 2 {
			continue
		}
		left, right = args[0], args[1]
		if operation == 3 || operation == 5 {
			left, right = right, left
		}
		result.strict = (operation == 2 || operation == 3) == negative
		matched = true
		break
	}
	if !matched {
		args, equality, err := p.syntax.operation(literal, p.syntax.manager.BasicFamilyID(), ast.ASTOpEq)
		if err != nil {
			return nil, err
		}
		if equality && len(args) == 2 {
			for index := 0; index < 2 && !matched; index++ {
				mod, modular, err := p.syntax.operation(args[index], p.syntax.family, 13)
				if err != nil {
					return nil, err
				}
				zero, err := p.syntax.manager.ArithmeticNumeralIsZero(args[1-index])
				if err != nil {
					return nil, err
				}
				if !modular || len(mod) != 2 || !zero {
					continue
				}
				numeral, err := p.syntax.numeral(mod[1], &divisor)
				if err != nil {
					return nil, err
				}
				divisorZero := false
				if numeral {
					if divisorZero, err = q.IsZero(&divisor); err != nil {
						return nil, err
					}
				}
				if !numeral || divisorZero {
					return nil, nil
				}
				left, right = mod[0], args[1-index]
				if err = q.Copy(&result.divisor, &divisor); err != nil {
					return nil, err
				}
				matched = true
				divisibility = true
			}
		}
		if equality && len(args) == 2 && !matched {
			_, arithmetic, err := p.syntax.arithmetic(args[0])
			if err != nil {
				return nil, err
			}
			if arithmetic {
				left, right = args[0], args[1]
				result.equality = !negative
				result.disequality = negative
				matched = true
			}
		}
	}
	if !matched {
		return nil, nil
	}
	terms := make([]ast.ASTHandle, 0, 4)
	ok, err := p.linearTerm(&multiplier, left, variable, &result.coefficient, &terms, integer)
	if err == nil && ok && !divisibility {
		if err = errors.Join(q.Copy(&negMultiplier, &multiplier), q.Neg(&negMultiplier)); err == nil {
			ok, err = p.linearTerm(&negMultiplier, right, variable, &result.coefficient, &terms, integer)
		}
	}
	if err != nil || !ok {
		return nil, err
	}
	var term ast.ASTHandle
	switch len(terms) {
	case 0:
		if integer {
			term, err = p.syntax.manager.ArithmeticIntegerNumeralText("0")
		} else {
			term, err = p.syntax.manager.ArithmeticRealNumeralText("0")
		}
	case 1:
		term = terms[0]
	default:
		// t = a.mk_add(ts.size(), ts.c_ptr()) (AUDIT.md qe_02-F7).
		term, err = p.syntax.manager.MkFamilyApp(p.syntax.family, 6, nil, terms)
	}
	if err != nil {
		return nil, err
	}
	result.term, err = p.syntax.manager.NewRef(term)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func closeLegacyLinearLiterals(values ...*legacyLinearLiteral) (result error) {
	for _, value := range values {
		if value != nil {
			result = errors.Join(result, value.Close())
		}
	}
	return result
}
