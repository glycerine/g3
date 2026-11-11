package spacer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerNestedDatatypeExpansionProcessesAppendedMixedFields(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) { testNestedMixedLiteralExpansion(t, reverse, -1) })
	}
}

func testNestedMixedLiteralExpansion(t *testing.T, reverse bool, stop int64) bool {
	t.Helper()
	var budget *arena.Budget
	t.Cleanup(func() {
		if budget != nil && budget.Used() != 0 {
			t.Fatal("nested expansion leaked bytes", budget.Used())
		}
	})
	m, globals := newSpacerTestServices(t)
	budget = m.Budget()
	ownRefs := []ast.ASTRef{}
	defer func() { depthMust(t, releaseNormalizeRefs(m, ownRefs)) }()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		depthMust(t, err)
		r, e := m.NewRef(h)
		depthMust(t, e)
		ownRefs = append(ownRefs, r)
		return h
	}
	name := func(s string) symbol.Symbol { v, e := m.Symbols.Intern(s); depthMust(t, e); return v }
	zero := own(m.ArithmeticIntegerNumeralText("0"))
	integer, e := m.GetSort(zero)
	depthMust(t, e)
	seven := own(m.ArithmeticIntegerNumeralText("7"))
	two := own(m.ArithmeticIntegerNumeralText("2"))
	bits := own(m.BitVectorNumeralUnsigned(5, 3))
	bv, e := m.GetSort(bits)
	depthMust(t, e)
	boolean := own(m.MkBoolSort())
	truth := own(m.MkTrue())
	makeSort := func(label string, fields []ast.DatatypeAccessorDefinition) (ast.ASTHandle, ast.ASTHandle) {
		sorts, e := m.MkDatatypes([]ast.DatatypeDefinition{{Name: name(label), Constructors: []ast.DatatypeConstructorDefinition{{Name: name(label + "-empty"), Recognizer: name(label + "-is-empty")}, {Name: name(label + "-make"), Recognizer: name(label + "-is-make"), Accessors: fields}}}})
		depthMust(t, e)
		own(sorts[0], nil)
		ctors, e := m.DatatypeConstructors(sorts[0])
		depthMust(t, e)
		return sorts[0], ctors[1]
	}
	inner, innerCtor := makeSort("nested-inner", []ast.DatatypeAccessorDefinition{{Name: name("inner-number"), Range: integer}, {Name: name("inner-bits"), Range: bv}})
	outer, outerCtor := makeSort("nested-outer", []ast.DatatypeAccessorDefinition{{Name: name("outer-child"), Range: inner}, {Name: name("outer-suffix"), Range: integer}, {Name: name("outer-flag"), Range: boolean}})
	innerValue := own(m.MkApp(innerCtor, []ast.ASTHandle{seven, bits}))
	value := own(m.MkApp(outerCtor, []ast.ASTHandle{innerValue, two, truth}))
	term := own(m.MkFreshConst("nested-expansion", outer))
	left, right := term, value
	if reverse {
		left, right = right, left
	}
	input := own(m.MkEq(left, right))
	before, e := m.GetRefCount(input)
	depthMust(t, e)
	pt := &PredicateTransformer{manager: m}
	succeeded := false
	if stop >= 0 {
		references := m.Stats().LiveReferences
		mode, e := m.ProofMode()
		depthMust(t, e)
		m.Budget().FailAfter(stop)
		attempt, cause := pt.expandInductiveLiteral(input)
		m.Budget().FailAfter(-1)
		if cause != nil {
			if attempt != nil {
				t.Fatal("failed nested expansion published output")
			}
			var cleanup memory.ConstructionFailure
			if errors.As(cause, &cleanup) {
				depthMust(t, cleanup.Cleanup())
			}
			var failure memory.Failure
			if !errors.Is(cause, arena.ErrInjectedFailure) && (!errors.As(cause, &failure) || failure.Kind != memory.FailureMemout) {
				t.Fatal("unexpected expansion failure", stop, cause)
			}
		} else {
			succeeded = true
			depthMust(t, attempt.Close())
		}
		if m.Stats().LiveReferences != references {
			t.Fatal("nested expansion leaked reference slots", stop)
		}
		count, e := m.GetRefCount(input)
		depthMust(t, e)
		if count != before {
			t.Fatal("nested expansion changed caller ownership", stop, count, before)
		}
		currentMode, e := m.ProofMode()
		depthMust(t, e)
		if currentMode != mode {
			t.Fatal("nested expansion changed proof mode")
		}
	}
	expanded, e := pt.expandInductiveLiteral(input)
	depthMust(t, e)
	defer func() { depthMust(t, expanded.Close()) }()
	if len(expanded.literals) != 10 {
		t.Fatal("wrong nested expansion length", len(expanded.literals))
	}
	outerAccess, e := m.DatatypeConstructorAccessors(outerCtor)
	depthMust(t, e)
	child := own(m.MkApp(outerAccess[0], []ast.ASTHandle{term}))
	suffix := own(m.MkApp(outerAccess[1], []ast.ASTHandle{term}))
	flag := own(m.MkApp(outerAccess[2], []ast.ASTHandle{term}))
	innerAccess, e := m.DatatypeConstructorAccessors(innerCtor)
	depthMust(t, e)
	number := own(m.MkApp(innerAccess[0], []ast.ASTHandle{child}))
	selectedBits := own(m.MkApp(innerAccess[1], []ast.ASTHandle{child}))
	outerRecognizer, e := m.DatatypeConstructorIs(outerCtor)
	depthMust(t, e)
	innerRecognizer, e := m.DatatypeConstructorIs(innerCtor)
	depthMust(t, e)
	expected := []ast.ASTHandle{own(m.MkApp(outerRecognizer, []ast.ASTHandle{term})), own(m.MkApp(innerRecognizer, []ast.ASTHandle{child})), own(m.MkArithmeticLE(suffix, two)), own(m.MkArithmeticGE(suffix, two)), own(m.MkArithmeticLE(number, seven)), own(m.MkArithmeticGE(number, seven)), own(m.MkEq(flag, truth))}
	bvFamily, e := m.BitVectorUtilityFamilyID()
	depthMust(t, e)
	one := own(m.MkFamilyApp(bvFamily, 1, nil, nil)) // OP_BIT1
	for bit := uint32(0); bit < 3; bit++ {
		selected := own(m.MkBitVectorExtract(bit, bit, selectedBits))
		// The appended bit equalities compare against the bit1 application,
		// which bv.is_numeral does not accept, so the source loop leaves
		// them as they are when it visits them again.
		lit := own(m.MkEq(one, selected))
		if bit == 1 {
			lit = own(m.MkNot(lit))
		}
		expected = append(expected, lit)
	}
	for i, want := range expected {
		if expanded.literals[i] != want {
			t.Fatal("source expansion ordering mismatch", i)
		}
	}
	if count, e := m.GetRefCount(input); e != nil || count != before {
		t.Fatal("expansion kept replaced input", count, before, e)
	}
	conjunction := own(m.MkAndArray(expanded.literals))
	equivalent := own(m.MkEq(input, conjunction))
	negation := own(m.MkNot(equivalent))
	solver, e := smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	depthMust(t, e)
	defer func() { depthMust(t, solver.Close()) }()
	depthMust(t, solver.Assert(negation))
	status, e := solver.CheckSat(nil)
	depthMust(t, e)
	if status != lbool.False {
		t.Fatal("nested expansion changed the formula", status)
	}
	return succeeded
}

func TestSpacerNestedLiteralExpansionAllocationFailureAndRetry(t *testing.T) {
	succeeded := false
	for stop := int64(0); stop < 1024 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) { succeeded = testNestedMixedLiteralExpansion(t, false, stop) })
		if t.Failed() {
			return
		}
		if succeeded {
			// The expansion releases each replaced vector element as the
			// source does, so it may complete within the slots freed along
			// the way and reserve nothing.
			t.Log("first successful allocation allowance", stop)
		}
	}
	if !succeeded {
		t.Fatal("missing nested expansion success frontier")
	}
}
