package api

// Source: src/api/api_solver.cpp public assertion, scope, check, model,
// unsat-core, proof and reason-unknown operations.

import (
	"bytes"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtAPIFixture struct {
	t *testing.T
	c *SMTContext
	m *ast.ASTManager
	// trail: api::context::m_ast_trail of the C API reference programs
	// (see newAPIProgramFixture); nil for the other tests.
	trail *[]ast.ASTRef
}

func TestSMTContextExecuteSMT2SharesSessionWithGoAPI(t *testing.T) {
	c, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	var output bytes.Buffer
	script := `(set-logic QF_UF)(declare-const p Bool)(assert p)(check-sat)`
	astTestMust(t, c.ExecuteSMT2(strings.NewReader(script), &output))
	if output.String() != "sat\n" {
		t.Fatal("first script output", output.String())
	}
	assertions, err := c.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 {
		t.Fatal("script assertion visibility", len(assertions))
	}
	md, err := c.Model()
	astTestMust(t, err)
	astTestMust(t, md.DecRef())

	output.Reset()
	script = `(push)(assert (not p))(check-sat)(pop)(check-sat)`
	astTestMust(t, c.ExecuteSMT2(strings.NewReader(script), &output))
	if output.String() != "unsat\nsat\n" {
		t.Fatal("continued script output", output.String())
	}
	if err = c.ExecuteSMT2(nil, &output); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("nil script reader", err)
	}
}

func newSMTAPIFixture(t *testing.T, logic string, configure func(*SMTContext)) *smtAPIFixture {
	t.Helper()
	c, err := NewSMTContext()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Close()) })
	if configure != nil {
		configure(c)
	}
	astTestMust(t, c.SetLogic(logic))
	m, err := c.Manager()
	astTestMust(t, err)
	return &smtAPIFixture{t: t, c: c, m: m}
}

func (f *smtAPIFixture) pin(h ast.ASTHandle, err error) ast.ASTHandle {
	f.t.Helper()
	astTestMust(f.t, err)
	ref, err := f.m.NewRef(h)
	astTestMust(f.t, err)
	if f.trail != nil {
		*f.trail = append(*f.trail, ref)
		return h
	}
	f.t.Cleanup(func() { astTestMust(f.t, f.m.DelRef(&ref)) })
	return h
}

func (f *smtAPIFixture) arithmeticVariable(name string, integer bool) ast.ASTHandle {
	f.t.Helper()
	family, err := f.m.ArithmeticFamilyID()
	astTestMust(f.t, err)
	kind := int32(0)
	if integer {
		kind = 1
	}
	sort := f.pin(f.m.MkFamilySort(family, kind, nil))
	symbol, err := f.m.Symbols.Intern(name)
	astTestMust(f.t, err)
	return f.pin(f.m.MkConst(symbol, sort))
}

func TestSMTContextEndToEndModelScopesAndAssumptions(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	x := f.arithmeticVariable("api-x", true)
	four := f.pin(f.m.ArithmeticIntegerNumeralText("4"))
	equalFour := f.pin(f.m.MkEq(x, four))
	astTestMust(t, f.c.Assert(equalFour))
	assertions, err := f.c.Assertions()
	astTestMust(t, err)
	if len(assertions) != 1 || assertions[0] != equalFour {
		t.Fatal("assertion order", assertions)
	}
	status, err := f.c.CheckSat()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("base status", status)
	}
	md, err := f.c.Model()
	astTestMust(t, err)
	declaration, err := f.m.GetDecl(x)
	astTestMust(t, err)
	value, err := md.ConstantInterpretation(declaration)
	astTestMust(t, err)
	text, integer, matched, err := f.m.ArithmeticNumeralText(value)
	astTestMust(t, err)
	if !matched || !integer || text != "4" {
		t.Fatal("model value", text, integer, matched)
	}
	astTestMust(t, md.DecRef())
	stats, err := f.c.Statistics()
	astTestMust(t, err)
	if stats.Size() == 0 {
		t.Fatal("missing solver statistics")
	}

	astTestMust(t, f.c.Push())
	level, err := f.c.ScopeLevel()
	astTestMust(t, err)
	if level != 1 {
		t.Fatal("scope level", level)
	}
	five := f.pin(f.m.ArithmeticIntegerNumeralText("5"))
	equalFive := f.pin(f.m.MkEq(x, five))
	astTestMust(t, f.c.Assert(equalFive))
	status, err = f.c.CheckSat()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("scoped contradiction", status)
	}
	astTestMust(t, f.c.Pop(1))
	level, err = f.c.ScopeLevel()
	astTestMust(t, err)
	if level != 0 {
		t.Fatal("popped scope level", level)
	}
	boolean := f.pin(f.m.MkBoolSort())
	assumptionName, err := f.m.Symbols.Intern("api-assumption")
	astTestMust(t, err)
	assumption := f.pin(f.m.MkConst(assumptionName, boolean))
	status, err = f.c.CheckSat(assumption)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("assumption recovery", status)
	}
}

func TestSMTContextOptimizationObjectivesAndSoftConstraints(t *testing.T) {
	t.Run("arithmetic", func(t *testing.T) {
		f := newSMTAPIFixture(t, "QF_LIA", nil)
		x := f.arithmeticVariable("api-opt-x", true)
		zero := f.pin(f.m.ArithmeticIntegerNumeralText("0"))
		five := f.pin(f.m.ArithmeticIntegerNumeralText("5"))
		lower := f.pin(f.m.MkArithmeticGE(x, zero))
		upper := f.pin(f.m.MkArithmeticLE(x, five))
		astTestMust(t, f.c.Assert(lower))
		astTestMust(t, f.c.Assert(upper))
		astTestMust(t, f.c.Maximize(x))
		status, err := f.c.CheckSat()
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("optimization status", status)
		}
		var output bytes.Buffer
		astTestMust(t, f.c.WriteObjectives(&output))
		if output.String() != "(objectives\n (api-opt-x 5)\n)\n" {
			t.Fatal("objective output", output.String())
		}
		md, err := f.c.Model()
		astTestMust(t, err)
		declaration, err := f.m.GetDecl(x)
		astTestMust(t, err)
		value, err := md.ConstantInterpretation(declaration)
		astTestMust(t, err)
		text, integer, matched, err := f.m.ArithmeticNumeralText(value)
		astTestMust(t, err)
		if !matched || !integer || text != "5" {
			t.Fatal("optimal model value", text, integer, matched)
		}
		astTestMust(t, md.DecRef())
	})

	t.Run("soft", func(t *testing.T) {
		f := newSMTAPIFixture(t, "QF_UF", nil)
		boolean := f.pin(f.m.MkBoolSort())
		symbol, err := f.m.Symbols.Intern("api-soft-p")
		astTestMust(t, err)
		p := f.pin(f.m.MkConst(symbol, boolean))
		notP := f.pin(f.m.MkNot(p))
		astTestMust(t, f.c.AssertSoft(p, big.NewRat(2, 1), "preferences"))
		astTestMust(t, f.c.AssertSoft(notP, big.NewRat(1, 1), "preferences"))
		status, err := f.c.CheckSat()
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("soft status", status)
		}
		var output bytes.Buffer
		astTestMust(t, f.c.WriteObjectives(&output))
		if output.String() != "(objectives\n (preferences 1)\n)\n" {
			t.Fatal("soft objective output", output.String())
		}
	})
}

func TestSMTContextCheckSatUsingPublishesTacticModel(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	x := f.arithmeticVariable("api-tactic-x", true)
	four := f.pin(f.m.ArithmeticIntegerNumeralText("4"))
	equality := f.pin(f.m.MkEq(x, four))
	astTestMust(t, f.c.Assert(equality))
	status, err := f.c.CheckSatUsing("qflia")
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("tactic status", status)
	}
	md, err := f.c.Model()
	astTestMust(t, err)
	declaration, err := f.m.GetDecl(x)
	astTestMust(t, err)
	value, err := md.ConstantInterpretation(declaration)
	astTestMust(t, err)
	text, integer, matched, err := f.m.ArithmeticNumeralText(value)
	astTestMust(t, err)
	if !matched || !integer || text != "4" {
		t.Fatal("tactic model value", text, integer, matched)
	}
	astTestMust(t, md.DecRef())
	if _, err = f.c.CheckSatUsing("not-a-retained-tactic"); err == nil {
		t.Fatal("unknown tactic accepted")
	}
}

func TestSMTContextEvaluateReturnsOwningValue(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	x := f.arithmeticVariable("api-eval-x", true)
	four := f.pin(f.m.ArithmeticIntegerNumeralText("4"))
	one := f.pin(f.m.ArithmeticIntegerNumeralText("1"))
	equality := f.pin(f.m.MkEq(x, four))
	sum := f.pin(f.m.MkArithmeticAdd(x, one))
	if _, err := f.c.Evaluate(sum, false); err == nil {
		t.Fatal("evaluation accepted before a model was available")
	}
	astTestMust(t, f.c.Assert(equality))
	status, err := f.c.CheckSat()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("evaluation status", status)
	}
	value, err := f.c.Evaluate(sum, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&value)) }()
	handle, err := f.m.RefGet(value)
	astTestMust(t, err)
	text, integer, matched, err := f.m.ArithmeticNumeralText(handle)
	astTestMust(t, err)
	if !matched || !integer || text != "5" {
		t.Fatal("evaluated value", text, integer, matched)
	}
}

func TestSMTContextUnsatCoreAndProofResults(t *testing.T) {
	t.Run("core", func(t *testing.T) {
		f := newSMTAPIFixture(t, "QF_UF", func(c *SMTContext) {
			astTestMust(t, c.SetBoolOption("produce-unsat-cores", true))
		})
		boolean := f.pin(f.m.MkBoolSort())
		symbol, err := f.m.Symbols.Intern("api-core-p")
		astTestMust(t, err)
		p := f.pin(f.m.MkConst(symbol, boolean))
		notP := f.pin(f.m.MkNot(p))
		astTestMust(t, f.c.AssertTracked(p, "positive"))
		astTestMust(t, f.c.AssertTracked(notP, "negative"))
		status, err := f.c.CheckSat()
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal(status)
		}
		core, err := f.c.UnsatCore()
		astTestMust(t, err)
		if len(core) != 2 || core[0] == core[1] {
			t.Fatal("tracked core", core)
		}
	})
	t.Run("proof", func(t *testing.T) {
		f := newSMTAPIFixture(t, "QF_UF", func(c *SMTContext) {
			astTestMust(t, c.SetBoolOption("produce-proofs", true))
		})
		boolean := f.pin(f.m.MkBoolSort())
		symbol, err := f.m.Symbols.Intern("api-proof-p")
		astTestMust(t, err)
		p := f.pin(f.m.MkConst(symbol, boolean))
		notP := f.pin(f.m.MkNot(p))
		astTestMust(t, f.c.Assert(p))
		astTestMust(t, f.c.Assert(notP))
		status, err := f.c.CheckSat()
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal(status)
		}
		proof, err := f.c.Proof()
		astTestMust(t, err)
		fact, err := f.m.GetProofFact(proof)
		astTestMust(t, err)
		if !f.m.IsFalse(fact) {
			t.Fatal("proof does not derive false")
		}
	})
}

func TestSMTContextConsequencesOwnReturnedExpressions(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	boolean := f.pin(f.m.MkBoolSort())
	symbol, err := f.m.Symbols.Intern("api-consequence-p")
	astTestMust(t, err)
	p := f.pin(f.m.MkConst(symbol, boolean))
	astTestMust(t, f.c.Assert(p))
	status, consequences, err := f.c.Consequences(nil, []ast.ASTHandle{p})
	astTestMust(t, err)
	if status != lbool.True || len(consequences) != 1 {
		t.Fatal("consequence result", status, len(consequences))
	}
	value, err := f.m.RefGet(consequences[0])
	astTestMust(t, err)
	sort, err := f.m.GetSort(value)
	astTestMust(t, err)
	if sort != boolean {
		t.Fatal("consequence is not Boolean", value)
	}
	for i := range consequences {
		astTestMust(t, f.m.DelRef(&consequences[i]))
	}
}

func TestSMTContextImpliedEqualities(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	boolean := f.pin(f.m.MkBoolSort())
	terms := make([]ast.ASTHandle, 3)
	for i, name := range []string{"api-implied-p", "api-implied-q", "api-implied-r"} {
		symbol, err := f.m.Symbols.Intern(name)
		astTestMust(t, err)
		terms[i] = f.pin(f.m.MkConst(symbol, boolean))
	}
	equality := f.pin(f.m.MkEq(terms[0], terms[1]))
	astTestMust(t, f.c.Assert(equality))
	status, classes, err := f.c.ImpliedEqualities(terms)
	astTestMust(t, err)
	if status != lbool.True || len(classes) != len(terms) {
		t.Fatal("implied equality result", status, classes)
	}
	if classes[0] != classes[1] || classes[0] == classes[2] {
		t.Fatal("implied equality partition", classes)
	}
}

func TestSMTContextAssertSMT2TransactionalFinalAssertions(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	status, err := f.c.CheckSat()
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal("initial status", status)
	}
	script := `(set-logic QF_UF)
(declare-const p Bool)
(assert p)
(check-sat)
(assert (not p))`
	astTestMust(t, f.c.AssertSMT2(strings.NewReader(script)))
	assertions, err := f.c.Assertions()
	astTestMust(t, err)
	if len(assertions) != 2 {
		t.Fatal("loaded assertion count", len(assertions))
	}
	if _, err = f.c.command.FindFunctionDeclaration("p", nil, nil, ast.ASTHandle{}); err == nil {
		t.Fatal("temporary parser declaration leaked into API context")
	}
	status, err = f.c.CheckSat()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("loaded assertion status", status)
	}

	before := len(assertions)
	err = f.c.AssertSMT2(strings.NewReader("(assert true)(assert"))
	if err == nil {
		t.Fatal("malformed SMT-LIB accepted")
	}
	assertions, assertErr := f.c.Assertions()
	astTestMust(t, assertErr)
	if len(assertions) != before {
		t.Fatal("failed parse published an assertion", len(assertions), before)
	}

	t.Run("arithmetic-plugins", func(t *testing.T) {
		arithmetic := newSMTAPIFixture(t, "QF_LIA", nil)
		status, err := arithmetic.c.CheckSat()
		astTestMust(t, err)
		if status != lbool.True {
			t.Fatal("initial arithmetic status", status)
		}
		script := `(set-logic QF_LIA)
(declare-const x Int)
(assert (= x 3))
(assert (= x 4))`
		astTestMust(t, arithmetic.c.AssertSMT2(strings.NewReader(script)))
		status, err = arithmetic.c.CheckSat()
		astTestMust(t, err)
		if status != lbool.False {
			t.Fatal("translated arithmetic status", status)
		}
	})
}

func TestSMTContextAssertSMT2ParametricDatatypeIntegration(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	script := `(declare-datatype List (par (T) ((nil) (cons (head T) (tail (List T))))))
(declare-const xs (List Bool))
(assert (= xs (cons true nil)))
(assert (not (head xs)))`
	astTestMust(t, f.c.AssertSMT2(strings.NewReader(script)))
	status, err := f.c.CheckSat()
	astTestMust(t, err)
	if status != lbool.False {
		t.Fatal("translated parametric datatype status", status)
	}
}

func TestSMTContextUnknownReason(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", func(c *SMTContext) {
		astTestMust(t, c.SetBoolOption("auto_config", false))
		astTestMust(t, c.SetUIntOption("smt.arith.solver", 0))
	})
	x := f.arithmeticVariable("api-unknown-x", false)
	zero := f.pin(f.m.ArithmeticRealNumeralText("0"))
	lower := f.pin(f.m.MkArithmeticGE(x, zero))
	astTestMust(t, f.c.Assert(lower))
	astTestMust(t, f.c.Push())
	status, err := f.c.CheckSat()
	astTestMust(t, err)
	if status != lbool.Undef {
		t.Fatal("disabled arithmetic status", status)
	}
	reason, err := f.c.ReasonUnknown()
	astTestMust(t, err)
	if reason != "smt tactic failed to show goal to be sat/unsat (incomplete (theory no arithmetic))" {
		t.Fatal("unknown reason", reason)
	}
}

func TestSMTContextCopiesShareClose(t *testing.T) {
	c, err := NewSMTContext()
	astTestMust(t, err)
	copy := *c
	astTestMust(t, copy.Close())
	if _, err = c.CheckSat(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("shared close did not retire aliases", err)
	}
}
