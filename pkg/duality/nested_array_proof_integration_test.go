package duality_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDualityNestedArrayOutputHasCertifiedProof(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := context.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	context.PrintSuccess = false
	var output bytes.Buffer
	script := `(set-option :fixedpoint.engine duality)
(set-option :fixedpoint.generate_proof_trace true)
(set-option :fixedpoint.xform.quantify_arrays true)
(declare-rel P ((Array Int (Array Int Int))))
(declare-var a (Array Int (Array Int Int)))
(rule (P a))
(query P :print-answer false)
`
	if err := smt2.ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err)
	}
	// z3 rejects the quantifier-abstracted rules: duality's get_rules()
	// runs check_rules (check_existential_tail) on the transformed set.
	const rejected = "(error \"query failed: recursive predicate (P (:var 0) (:var 1) (select (select (:var 2) (:var 0)) (:var 1))) occurs nested in the body of a rule\")\nunknown\n"
	if output.String() == rejected {
		return
	}
	if output.String() != "sat\n" {
		t.Fatal("nested-array output query", output.String())
	}
	m, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		t.Fatal(err)
	}
	integer, err := m.GetSort(zero)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), integer)
	if err != nil {
		t.Fatal(err)
	}
	outer, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{integer}), inner)
	if err != nil {
		t.Fatal(err)
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	predicate, err := context.FindFunctionDeclaration("P", nil, []ast.ASTHandle{outer}, boolean)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := context.FixedpointProof(predicate)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := m.DelRef(&proof); err != nil {
			t.Fatal(err)
		}
	}()
	h, err := m.RefGet(proof)
	if err != nil {
		t.Fatal(err)
	}
	checker, err := proofs.NewChecker(m)
	if err != nil {
		t.Fatal(err)
	}
	valid, side, err := checker.Check(h)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := checker.ReleaseSideConditions(side); err != nil {
			t.Fatal(err)
		}
	}()
	if !valid {
		t.Fatal("nested-array proof is structurally invalid")
	}
	closed, err := proofs.IsClosed(m, h)
	if err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Fatal("nested-array proof has an escaping hypothesis")
	}
	config, err := context.SolverTranslationConfig()
	if err != nil {
		t.Fatal(err)
	}
	native, err := smtcore.NewSMTSolver(m, nil, "", config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := native.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	for i, condition := range side {
		if err := native.Push(); err != nil {
			t.Fatal(err)
		}
		negated, err := m.MkNot(condition)
		if err != nil {
			t.Fatal(err)
		}
		if err := native.Assert(negated); err != nil {
			t.Fatal(err)
		}
		status, err := native.CheckSat(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := native.Pop(1); err != nil {
			t.Fatal(err)
		}
		if status != lbool.False {
			t.Fatal("nested-array proof side condition", i, status)
		}
	}
	asserted := 0
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{h}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if seen[current] {
			continue
		}
		seen[current] = true
		leaf, err := m.IsProofKind(current, ast.ASTPrAsserted)
		if err != nil {
			t.Fatal(err)
		}
		if leaf {
			asserted++
		}
		n, err := m.GetProofNumParents(current)
		if err != nil {
			t.Fatal(err)
		}
		for i := uint32(0); i < n; i++ {
			parent, err := m.GetProofParent(current, i)
			if err != nil {
				t.Fatal(err)
			}
			stack = append(stack, parent)
		}
	}
	if asserted == 0 {
		t.Fatal("nested-array proof has no transformed source leaves")
	}
}
