package smtcore

// Source: src/smt/theory_lra.cpp get_value/init_variable_values and model hooks.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/lp"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// The generator's proto model owns the factory. The theory only borrows it.
type smtLRAModelOwner struct {
	generator *SMTModelGenerator
	factory   *protomodel.ArithmeticFactory
}

var _ SMTModelValueTheory = (*SMTLRATheory)(nil)
var _ SMTTheoryDirectValue = (*SMTLRATheory)(nil)

// GetTheoryValue preserves the source cache eligibility test; it does not
// initialize a model or read fresh LP values on behalf of the quick checker.
func (t *SMTLRATheory) GetTheoryValue(node *SMTEnode) (_ ast.ASTHandle, _ bool, err error) {
	s, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if node == nil || node.manager != s.manager {
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	if err = t.numeric.ready(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if !t.HasVariable(node.TheoryVar(t.FamilyID())) || !t.canGetModelValue(node.TheoryVar(t.FamilyID())) {
		return ast.ASTHandle{}, false, nil
	}
	// rational val = get_value(v) lives until get_value(n, r) returns.
	astQ := s.manager.NumeralService().Manager
	value := smtQZero()
	defer func() {
		if e := smtQDel(astQ, &value); e != nil && err == nil {
			err = e
		}
	}()
	if err = t.cachedModelValue(node.TheoryVar(t.FamilyID()), &value); err != nil {
		return ast.ASTHandle{}, false, err
	}
	sort, err := s.manager.GetSort(node.owner)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	kind, err := s.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	expression, err := s.manager.ArithmeticNumeralFromRational(&value, kind == 1)
	return expression, err == nil, err
}

func (t *SMTLRATheory) canGetModelValue(v SMTTheoryVar) bool {
	index, ok := t.numeric.index(v)
	return ok && (t.numeric.solver.IsTerm(index) || t.modelValues[index] != nil)
}

// cachedModelValue is get_value(v). The value is computed and cached as a
// rational of the theory's rational manager; the returned rational is
// copied into out, a rational of the AST manager's numeral manager that the
// caller owns (both are the source's one rational manager).
func (t *SMTLRATheory) cachedModelValue(v SMTTheoryVar, out *numeral.Rational) (err error) {
	if err = t.numeric.ready(); err != nil {
		return err
	}
	q := t.numerals
	astQ := t.state.manager.NumeralService().Manager
	if !t.canGetModelValue(v) {
		return astQ.Reset(out)
	}
	index, _ := t.numeric.index(v)
	if cached := t.modelValues[index]; cached != nil {
		return astQ.CopyFrom(out, q, cached)
	}
	defer func() {
		if err != nil {
			t.numeric.stopped = true
		}
	}()
	term := t.numeric.solver.Terms[index-lp.LARTermStartIndex]
	result := smtQZero()
	if err = q.Copy(&result, &term.Value); err != nil {
		smtQDel(q, &result)
		return err
	}
	if t.modelValues == nil {
		t.modelValues = make(map[uint32]*numeral.Rational)
	}
	for column, coefficient := range term.Coefficients().All() {
		x := t.modelValues[column]
		if x == nil {
			zero := smtQZero()
			x = &zero
			t.modelValues[column] = x
		}
		// result += coeff * m_variable_values[column]
		if err = q.AddMul(&result, coefficient, x, &result); err != nil {
			smtQDel(q, &result)
			return err
		}
	}
	t.modelValues[index] = &result
	return astQ.CopyFrom(out, q, &result)
}

func (t *SMTLRATheory) initializeVariableValues() error {
	if err := t.numeric.ready(); err != nil {
		return err
	}
	if t.NumVariables() == 0 {
		return nil
	}
	columns, err := t.numeric.modelColumnValues()
	if err != nil {
		return err
	}
	if t.modelValues == nil {
		t.modelValues = make(map[uint32]*numeral.Rational)
	}
	// Native GetModel updates the current extent without erasing older
	// entries; an overwritten value is freed.
	for column, value := range columns {
		if old := t.modelValues[column]; old != nil {
			if err = smtQDel(t.numerals, old); err != nil {
				return err
			}
		}
		t.modelValues[column] = value
	}
	return nil
}

func (t *SMTLRATheory) InitializeModel(g *SMTModelGenerator) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	if g == nil || g.manager != s.manager {
		return ast.ErrASTDomain
	}
	if err = g.open(); err != nil {
		return err
	}
	if g.model == nil {
		return ast.ErrASTDomain
	}
	if err = t.initializeVariableValues(); err != nil {
		return err
	}
	factory, err := protomodel.NewArithmeticFactory(s.manager)
	if err != nil {
		return err
	}
	if err = g.RegisterFactory(factory); err != nil {
		return errors.Join(err, factory.Close())
	}
	t.modelOwner = &smtLRAModelOwner{g, factory}
	return t.emitLRA("init_model", 2258, t.Display)
}

func (t *SMTLRATheory) ModelValue(node *SMTEnode, g *SMTModelGenerator) (_ SMTModelValueProcedure, err error) {
	s, err := t.State()
	if err != nil {
		return nil, err
	}
	if node == nil || g == nil || g.manager != s.manager || !t.HasVariable(node.TheoryVar(t.FamilyID())) || t.modelOwner == nil || t.modelOwner.generator != g {
		return nil, ast.ErrASTDomain
	}
	if err = g.open(); err != nil {
		return nil, err
	}
	if g.model == nil || g.model.Factory(t.FamilyID()) != t.modelOwner.factory {
		return nil, ast.ErrASTDomain
	}
	// rational value = get_value(v) lives until mk_value returns.
	astQ := s.manager.NumeralService().Manager
	value := smtQZero()
	defer func() { err = errors.Join(err, smtQDel(astQ, &value)) }()
	if err = t.cachedModelValue(node.TheoryVar(t.FamilyID()), &value); err != nil {
		return nil, err
	}
	sort, err := s.manager.GetSort(node.owner)
	if err != nil {
		return nil, err
	}
	kind, err := s.manager.GetDeclKind(sort)
	if err != nil {
		return nil, err
	}
	// The native LRA factory delegates directly to arith mk_numeral, which
	// rejects a fractional Int. It does not floor an incomplete assignment.
	expression, err := t.modelOwner.factory.MkNumValueRational(&value, kind == 1)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(expression)
}
