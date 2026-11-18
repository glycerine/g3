package smtcore

// Source: src/smt/theory_lra.cpp:2329-2448, retained theory_opt callbacks.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lp"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (t *SMTLRATheory) AddObjective(term ast.ASTHandle) (SMTTheoryVar, error) {
	s, err := t.State()
	if err != nil {
		return SMTNullTheoryVar, err
	}
	kind, err := s.manager.GetKind(term)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if kind != ast.ASTApp {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	sort, err := s.manager.GetSort(term)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	family, err := s.manager.GetFamilyID(sort)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if family != t.FamilyID() {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	// Unlike the old tableau frontend, native LRA does not add an IsLinear
	// gate here. Its compiler records unsupported terms in notHandled.
	return t.internalizeDef(term)
}

// astQ is the AST manager's rational manager (z3's rational values); the
// LRA's own numerals live on t.numerals.
func (t *SMTLRATheory) astQ() *numeral.RationalManager {
	return t.state.manager.NumeralService().Manager
}

// objectiveValueOut copies an inf_eps of the LRA's rational manager into a
// fresh value of the AST manager's, which the caller owns, and frees v.
func (t *SMTLRATheory) objectiveValueOut(v *simplex.Infinitesimal, cause error) (SMTArithmeticObjectiveValue, error) {
	out := simplex.ZeroInfinitesimal()
	if cause == nil {
		cause = out.CopyFrom(t.astQ(), t.numerals, v)
	}
	return SMTArithmeticObjectiveValue{Finite: out}, smtErrJoin(cause, v.Del(t.numerals))
}

// ObjectiveValue is value(v): inf_eps(get_ivalue(v)), owned by the caller.
func (t *SMTLRATheory) ObjectiveValue(v SMTTheoryVar) (SMTArithmeticObjectiveValue, error) {
	if _, err := t.State(); err != nil {
		return SMTArithmeticObjectiveValue{Finite: simplex.ZeroInfinitesimal()}, err
	}
	value, err := t.rawLRAValue(v)
	return t.objectiveValueOut(&value, err)
}

// lraNumeral is a.mk_numeral(r, is_int) for a rational r of the LRA's
// manager: the numeral is made from a copy on the AST manager's.
func (t *SMTLRATheory) lraNumeral(c *smtLRAAxiomCall, r *numeral.Rational, integer bool) (ast.ASTHandle, error) {
	q := t.astQ()
	copied := smtQZero()
	err := q.CopyFrom(&copied, t.numerals, r)
	var h ast.ASTHandle
	if err == nil {
		h, err = c.hold(t.state.manager.ArithmeticNumeralFromRational(&copied, integer))
	}
	return h, smtErrJoin(err, smtQDel(q, &copied))
}

// objectiveBlocker is mk_gt(v, val) for val on the LRA's rational manager.
func (t *SMTLRATheory) objectiveBlocker(v SMTTheoryVar, value *simplex.Infinitesimal) (result ast.ASTHandle, resultErr error) {
	node, err := t.Enode(v)
	if err != nil {
		return result, err
	}
	sort, err := t.state.manager.GetSort(node.owner)
	if err != nil {
		return result, err
	}
	kind, err := t.state.manager.GetDeclKind(sort)
	if err != nil {
		return result, err
	}
	c := &smtLRAAxiomCall{owner: t}
	defer func() { resultErr = errors.Join(resultErr, c.close()) }()
	q := t.numerals
	r := smtQZero()
	defer func() { resultErr = smtErrJoin(resultErr, smtQDel(q, &r)) }()
	if err = q.Copy(&r, &value.Main); err != nil {
		return result, err
	}
	op := smtArithOpGT
	if kind == 1 {
		// r = floor(r) + 1
		if err = q.Floor(&r, &r); err == nil {
			err = q.Inc(&r)
		}
		if err != nil {
			return result, err
		}
		op = smtArithOpGE
	} else if sign, err := q.Sign(&value.Epsilon); err != nil {
		return result, err
	} else if sign < 0 {
		op = smtArithOpGE
	}
	bound, err := t.lraNumeral(c, &r, kind == 1)
	if err != nil {
		return result, err
	}
	// Keep the final application unrooted for the caller to pin. It owns its
	// numeral argument while the lexical argument reference is retired.
	result, err = t.state.manager.MkFamilyApp(t.FamilyID(), op, nil, []ast.ASTHandle{node.owner, bound})
	if err != nil {
		return result, err
	}
	err = t.emitLRATag("opt", "mk_gt", 2392, func(out io.Writer) error {
		// The source operand is lp::impq (numeric_pair), not inf_rational.
		var texts [3]string
		for i, x := range []*numeral.Rational{&value.Main, &value.Epsilon, &r} {
			text, err := q.String(x)
			if err != nil {
				return err
			}
			texts[i] = text
		}
		if err := smtWriteTheoryText(out, fmt.Sprintf("v%d (%s, %s) %s ", v, texts[0], texts[1], texts[2])); err != nil {
			return err
		}
		if err := t.displayLRAExpression(out, result); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	})
	return result, err
}

func (t *SMTLRATheory) MaximizeObjective(v SMTTheoryVar) (value SMTArithmeticObjectiveValue, blocker ast.ASTHandle, hasShared bool, resultErr error) {
	if _, resultErr = t.State(); resultErr != nil {
		return
	}
	value, resultErr = t.numeric.maximize(v)
	// The maximum crosses to the AST manager's rationals for the caller.
	if !value.Unbounded {
		value, resultErr = t.objectiveValueOut(&value.Finite, resultErr)
	}
	if resultErr != nil {
		return
	}
	// The retained C++ caller initializes has_shared=false. LRA only writes
	// false on infinity paths and leaves it untouched for bounded results.
	if value.Unbounded {
		// An absent mapping takes the earlier, untraced infinity branch.
		if _, mapped := t.numeric.index(v); mapped {
			if resultErr = t.emitLRA("maximize", 2361, func(out io.Writer) error {
				return smtWriteTheoryText(out, fmt.Sprintf("Unbounded %d\n", v))
			}); resultErr != nil {
				return
			}
		}
		blocker, resultErr = t.state.manager.MkFalse()
		return
	}
	current, err := t.rawLRAValue(v)
	if err == nil {
		blocker, err = t.objectiveBlocker(v, &current)
	}
	resultErr = smtErrJoin(err, current.Del(t.numerals))
	return
}

func (t *SMTLRATheory) objectiveExpression(c *smtLRAAxiomCall, v SMTTheoryVar) (ast.ASTHandle, error) {
	node, err := t.Enode(v)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	index, ok := t.numeric.index(v)
	if !ok {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if !t.numeric.solver.IsTerm(index) {
		w := t.variableForLPIndex(index)
		n, err := t.Enode(w)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return n.owner, nil
	}
	sort, err := t.state.manager.GetSort(node.owner)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := t.state.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	term := t.numeric.solver.Terms[index-lp.LARTermStartIndex]
	var args []ast.ASTHandle
	for column, coefficient := range term.Coefficients().All() {
		w := t.variableForLPIndex(column)
		n, err := t.Enode(w)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		numeral, err := t.lraNumeral(c, coefficient, kind == 1)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		product, err := c.app(smtArithOpMul, numeral, n.owner)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args = append(args, product)
	}
	numeral, err := t.lraNumeral(c, &term.Value, kind == 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args = append(args, numeral)
	return c.app(smtArithOpAdd, args...)
}

func (t *SMTLRATheory) MakeObjectiveLowerBound(filter *tactic.FilterModelConverter, v SMTTheoryVar, value simplex.Infinitesimal) (result ast.ASTHandle, resultErr error) {
	if _, err := t.State(); err != nil {
		return result, err
	}
	if filter == nil {
		return result, ast.ErrASTHandle
	}
	q := t.astQ()
	if finite, err := q.IsZero(&value.Infinity); err != nil {
		return result, err
	} else if !finite {
		return result, ast.ErrASTDomain
	}
	if err := t.numeric.ready(); err != nil {
		return result, err
	}
	node, err := t.Enode(v)
	if err != nil {
		return result, err
	}
	sort, err := t.state.manager.GetSort(node.owner)
	if err != nil {
		return result, err
	}
	kind, err := t.state.manager.GetDeclKind(sort)
	if err != nil {
		return result, err
	}
	c := &smtLRAAxiomCall{owner: t}
	defer func() { resultErr = errors.Join(resultErr, c.close()) }()
	// a.mk_ge(mk_obj(v), a.mk_numeral(r, is_int)): GCC evaluates the
	// numeral argument first.
	bound, err := c.hold(t.state.manager.ArithmeticNumeralFromRational(&value.Main, kind == 1))
	if err != nil {
		return result, err
	}
	object, err := t.objectiveExpression(c, v)
	if err != nil {
		return result, err
	}
	epsilon, err := q.Sign(&value.Epsilon)
	if err != nil {
		return result, err
	}
	strict := epsilon > 0
	op := smtArithOpGE
	if strict {
		op = smtArithOpLE
	}
	base, err := c.app(op, object, bound)
	if err != nil {
		return result, err
	}
	if _, found := t.state.BoolVar(base); !found {
		decl, err := t.state.manager.GetDecl(base)
		if err != nil {
			return result, err
		}
		if err = filter.Insert(decl); err != nil {
			return result, err
		}
		bv, err := t.state.MkBoolVar(base)
		if err != nil {
			return result, err
		}
		if err = t.state.SetVariableTheory(bv, t.FamilyID()); err != nil {
			return result, err
		}
		b := &smtLRABound{boolean: bv, variable: v, upper: strict, value: smtQZero()}
		// The bound's rational lives on the LRA's rational manager.
		if err = t.numerals.CopyFrom(&b.value, q, &value.Main); err != nil {
			return result, err
		}
		// Publish ownership before the fallible axiom work, as InternalizeAtom
		// does, so an allocation refusal leaves a registered, retryable bound.
		t.bounds[v] = append(t.bounds[v], b)
		if err = t.updateUnassignedBounds(v, 1); err != nil {
			return result, err
		}
		t.boundTrail = append(t.boundTrail, v)
		t.boundByBoolean[bv] = b
		if err = t.mkBoundAxioms(b); err != nil {
			t.newBounds = append(t.newBounds, b)
			return result, err
		}
		if err = t.emitLRA("mk_ge", 2444, func(out io.Writer) error {
			if err := t.displayLRAExpression(out, base); err != nil {
				return err
			}
			return smtWriteTheoryText(out, "\n")
		}); err != nil {
			return result, err
		}
	}
	result = base
	if strict {
		result, err = t.state.manager.MkNot(base)
		if err != nil {
			return result, err
		}
	}
	err = t.emitLRA("mk_ge", 2449, func(out io.Writer) error {
		if err := t.displayLRAExpression(out, result); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	})
	return result, err // The internalizer owns base; a final NOT is caller-pinned.
}
