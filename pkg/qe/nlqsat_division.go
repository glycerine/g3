package qe

// Source: src/qe/nlqsat.cpp:431-538 div_rewriter_cfg, is_pure_proc, purify
// and ackermanize_div.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// arith_decl_plugin.h decl kinds and sort kinds read by is_pure_proc.
const (
	nlqArithNum    = 0
	nlqArithLE     = 2
	nlqArithGE     = 3
	nlqArithLT     = 4
	nlqArithGT     = 5
	nlqArithAdd    = 6
	nlqArithSub    = 7
	nlqArithUMinus = 8
	nlqArithMul    = 9
	nlqArithDiv    = 10
	nlqArithPower  = 18
	nlqRealSort    = 0
)

// nlqPureCheck is is_pure_proc (nlqsat.cpp:471-511) together with the
// quick_for_each_expr traversal of purify. A node outside NRA raises
// tactic_exception("not NRA"); in qsat mode a non-numeral division only marks
// the formula for division purification.
type nlqPureCheck struct {
	manager    *ast.ASTManager
	arithmetic ast.FamilyID
	boolean    ast.ASTHandle
	mode       qsatMode
	hasDivs    bool
}

func (c *nlqPureCheck) isRealOrBool(h ast.ASTHandle) (bool, error) {
	sort, err := c.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	if sort == c.boolean {
		return true, nil
	}
	family, err := c.manager.GetFamilyID(sort)
	if err != nil || family != c.arithmetic {
		return false, err
	}
	kind, err := c.manager.GetDeclKind(sort)
	return kind == nlqRealSort, err
}

func (c *nlqPureCheck) isNumeral(h ast.ASTHandle) (string, bool, error) {
	text, _, numeral, err := c.manager.ArithmeticNumeralText(h)
	return text, numeral, err
}

func (c *nlqPureCheck) app(n ast.ASTHandle) error {
	m := c.manager
	family, err := m.GetFamilyID(n)
	if err != nil {
		return err
	}
	if family == m.BasicFamilyID() {
		return nil
	}
	args, err := m.GetNumArgs(n)
	if err != nil {
		return err
	}
	if family == ast.NullFamilyID && args == 0 {
		ok, err := c.isRealOrBool(n)
		if err != nil || ok {
			return err
		}
	}
	if family == c.arithmetic {
		kind, err := m.GetDeclKind(n)
		if err != nil {
			return err
		}
		switch kind {
		case nlqArithMul, nlqArithAdd, nlqArithSub, nlqArithUMinus, nlqArithNum,
			nlqArithLE, nlqArithGE, nlqArithLT, nlqArithGT:
			return nil
		case nlqArithDiv:
			if args == 2 {
				denominator, err := m.GetArg(n, 1)
				if err != nil {
					return err
				}
				_, numeral, err := c.isNumeral(denominator)
				if err != nil || numeral {
					return err
				}
			}
			if c.mode == qsatSAT {
				c.hasDivs = true
				return nil
			}
		case nlqArithPower:
			if args == 2 {
				exponent, err := m.GetArg(n, 1)
				if err != nil {
					return err
				}
				text, numeral, err := c.isNumeral(exponent)
				if err != nil {
					return err
				}
				if numeral {
					value, valid := new(big.Rat).SetString(text)
					if valid && value.IsInt() && value.Sign() >= 0 && value.Num().IsUint64() && value.Num().Uint64() <= 0xffffffff {
						return nil
					}
				}
			}
		}
	}
	return tactic.NewTacticException("not NRA")
}

// check visits every subterm of root once, quantifier bodies and patterns
// included (for_each_expr_core with IgnorePatterns false).
func (c *nlqPureCheck) check(root ast.ASTHandle) error {
	m := c.manager
	visited := make(map[ast.ASTHandle]struct{})
	todo := []ast.ASTHandle{root}
	for len(todo) != 0 {
		n := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if _, seen := visited[n]; seen {
			continue
		}
		visited[n] = struct{}{}
		kind, err := m.GetKind(n)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTVar:
			ok, err := c.isRealOrBool(n)
			if err != nil {
				return err
			}
			if !ok {
				return tactic.NewTacticException("not NRA")
			}
		case ast.ASTApp:
			if err = c.app(n); err != nil {
				return err
			}
			args, err := m.GetNumArgs(n)
			if err != nil {
				return err
			}
			for i := uint32(0); i < args; i++ {
				arg, err := m.GetArg(n, i)
				if err != nil {
					return err
				}
				todo = append(todo, arg)
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(n)
			if err != nil {
				return err
			}
			todo = append(todo, body)
			patterns, err := m.GetNumPatterns(n)
			if err != nil {
				return err
			}
			for i := uint32(0); i < patterns; i++ {
				pattern, err := m.GetPattern(n, i)
				if err != nil {
					return err
				}
				todo = append(todo, pattern)
			}
			patterns, err = m.GetNumNoPatterns(n)
			if err != nil {
				return err
			}
			for i := uint32(0); i < patterns; i++ {
				pattern, err := m.GetNoPattern(n, i)
				if err != nil {
					return err
				}
				todo = append(todo, pattern)
			}
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

type nlqDivision struct {
	numerator, denominator, name ast.ASTRef
}

type nlqDivisionConfiguration struct {
	rewriter.DefaultConfiguration
	manager    *ast.ASTManager
	arithmetic ast.FamilyID
	divisions  []nlqDivision
}

func (c *nlqDivisionConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, _ ast.ASTRef) (rewriter.RewriteStatus, error) {
	family, err := c.manager.GetFamilyID(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	kind, err := c.manager.GetDeclKind(decl)
	if err != nil || family != c.arithmetic || kind != 10 || args.Count() != 2 {
		return rewriter.RewriteFailed, err
	}
	numerator, err := args.At(0)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	denominator, err := args.At(1)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	text, _, numeral, err := c.manager.ArithmeticNumeralText(denominator)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if numeral {
		value, valid := new(big.Rat).SetString(text)
		if valid && value.Sign() != 0 {
			return rewriter.RewriteFailed, nil
		}
	}
	sort, err := c.manager.GetSort(numerator)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	name, err := c.manager.MkFreshConst("div", sort)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	d := nlqDivision{}
	if d.numerator, err = c.manager.NewRef(numerator); err == nil {
		d.denominator, err = c.manager.NewRef(denominator)
	}
	if err == nil {
		d.name, err = c.manager.NewRef(name)
	}
	if err != nil {
		return rewriter.RewriteFailed, errors.Join(err, c.closeDivision(&d))
	}
	c.divisions = append(c.divisions, d)
	return rewriter.RewriteDone, c.manager.SetRef(out, name)
}

func (c *nlqDivisionConfiguration) closeDivision(d *nlqDivision) error {
	var err error
	for _, ref := range []*ast.ASTRef{&d.name, &d.denominator, &d.numerator} {
		if *ref != (ast.ASTRef{}) {
			err = errors.Join(err, c.manager.DelRef(ref))
		}
	}
	return err
}

func (c *nlqDivisionConfiguration) close() error {
	var err error
	for len(c.divisions) != 0 {
		i := len(c.divisions) - 1
		err = errors.Join(err, c.closeDivision(&c.divisions[i]))
		c.divisions = c.divisions[:i]
	}
	return err
}

func (p *qsatPreparation) ackermannizeDivisions(mode qsatMode, formula ast.ASTRef) (err error) {
	if err = p.open(); err != nil {
		return err
	}
	if len(p.variables) < 2 {
		return ast.ErrASTDomain
	}
	arithmetic, err := p.manager.RegisterArithmeticPlugin()
	if err != nil {
		return err
	}
	boolean, err := p.manager.MkBoolSort()
	if err != nil {
		return err
	}
	h, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	pure := nlqPureCheck{manager: p.manager, arithmetic: arithmetic, boolean: boolean, mode: mode}
	if err = pure.check(h); err != nil || !pure.hasDivs {
		return err
	}
	configuration := &nlqDivisionConfiguration{manager: p.manager, arithmetic: arithmetic}
	defer func() { err = errors.Join(err, configuration.close()) }()
	// div_rewriter_cfg's member m_zero(a.mk_real(0)) exists before rewriting.
	zero, err := p.manager.ArithmeticRealNumeralText("0")
	if err != nil {
		return err
	}
	zeroRef, err := p.manager.NewRef(zero)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, p.manager.DelRef(&zeroRef)) }()
	traversal, err := rewriter.NewTraversal(p.manager, configuration)
	if err != nil {
		if traversal != nil {
			err = errors.Join(err, traversal.Close())
		}
		return err
	}
	defer func() { err = errors.Join(err, traversal.Close()) }()
	if err = traversal.Rewrite(h, formula); err != nil || len(configuration.divisions) == 0 {
		return err
	}
	last := len(p.variables) - 2
	for i := range configuration.divisions {
		if err = p.appendRefs(&p.variables[last], []ast.ASTRef{configuration.divisions[i].name}); err != nil {
			return err
		}
	}
	level := EmptyMaxLevel()
	if last%2 == 0 {
		level.Existential = uint32(last)
	} else {
		level.Universal = uint32(last)
	}
	for _, ref := range p.variables[last] {
		variable, getErr := p.manager.RefGet(ref)
		if getErr != nil {
			return getErr
		}
		if err = p.abstraction.SetExpressionLevel(variable, level); err != nil {
			return err
		}
	}
	axioms := make([]ast.ASTHandle, 0, len(configuration.divisions)*2)
	for i, d := range configuration.divisions {
		numerator, getErr := p.manager.RefGet(d.numerator)
		if getErr != nil {
			return getErr
		}
		denominator, getErr := p.manager.RefGet(d.denominator)
		if getErr != nil {
			return getErr
		}
		name, getErr := p.manager.RefGet(d.name)
		if getErr != nil {
			return getErr
		}
		// GCC evaluates mk_or's arguments right to left: the definition
		// num = den*name, then den = 0.
		product, getErr := p.manager.MkArithmeticMul(denominator, name)
		if getErr != nil {
			return getErr
		}
		definition, getErr := p.manager.MkEq(numerator, product)
		if getErr != nil {
			return getErr
		}
		denominatorZero, getErr := p.manager.MkEq(denominator, zero)
		if getErr != nil {
			return getErr
		}
		axiom, getErr := p.manager.MkOr(denominatorZero, definition)
		if getErr != nil {
			return getErr
		}
		axioms = append(axioms, axiom)
		for j := i + 1; j < len(configuration.divisions); j++ {
			other := configuration.divisions[j]
			otherNumerator, getErr := p.manager.RefGet(other.numerator)
			if getErr != nil {
				return getErr
			}
			otherDenominator, getErr := p.manager.RefGet(other.denominator)
			if getErr != nil {
				return getErr
			}
			otherName, getErr := p.manager.RefGet(other.name)
			if getErr != nil {
				return getErr
			}
			// Right to left: name equality, not numerator equality, then not
			// denominator equality.
			nameEqual, getErr := p.manager.MkEq(name, otherName)
			if getErr != nil {
				return getErr
			}
			numeratorEqual, getErr := p.manager.MkEq(numerator, otherNumerator)
			if getErr != nil {
				return getErr
			}
			notNumerator, getErr := p.manager.MkNot(numeratorEqual)
			if getErr != nil {
				return getErr
			}
			denominatorEqual, getErr := p.manager.MkEq(denominator, otherDenominator)
			if getErr != nil {
				return getErr
			}
			notDenominator, getErr := p.manager.MkNot(denominatorEqual)
			if getErr != nil {
				return getErr
			}
			consistency, getErr := p.manager.MkOrArray([]ast.ASTHandle{notDenominator, notNumerator, nameEqual})
			if getErr != nil {
				return getErr
			}
			axioms = append(axioms, consistency)
		}
	}
	antecedent, err := p.manager.MkAndArray(axioms)
	if err != nil {
		return err
	}
	h, err = p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	var result ast.ASTHandle
	if last%2 == 0 {
		result, err = p.manager.MkAnd(h, antecedent)
	} else {
		result, err = p.manager.MkImplies(antecedent, h)
	}
	if err != nil {
		return err
	}
	return p.manager.SetRef(formula, result)
}
