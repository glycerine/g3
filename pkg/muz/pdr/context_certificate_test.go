package pdr_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type pdrCertificateFailureWriter struct{}

func (pdrCertificateFailureWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestContextDisplayCertificateSourceResultPresentations(t *testing.T) {
	unknown := newValidationBatchFixture(t, "display_certificate_unknown")
	var output bytes.Buffer
	if err := unknown.context.DisplayCertificate(&output, ast.ASTHandle{}, false); err != nil || output.String() != "unknown" {
		t.Fatal("unknown certificate", output.String(), err)
	}
	if err := unknown.context.DisplayCertificate(pdrCertificateFailureWriter{}, ast.ASTHandle{}, false); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("unknown certificate writer refusal", err)
	}

	reachable := newValidationBatchFixture(t, "display_certificate_reachable")
	status, err := reachable.context.Solve(reachable.declaration)
	pdrMust(t, err)
	if status != lbool.True {
		t.Fatal("reachable certificate setup", status)
	}
	output.Reset()
	pdrMust(t, reachable.context.DisplayCertificate(&output, reachable.declaration, false))
	if strings.TrimSpace(output.String()) != "true" {
		t.Fatalf("reachable certificate = %q", output.String())
	}
	if err = reachable.context.DisplayCertificate(pdrCertificateFailureWriter{}, reachable.declaration, false); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("reachable certificate writer refusal", err)
	}
	if err = reachable.context.DisplayCertificate(&output, ast.ASTHandle{}, false); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("certificate accepted a stale query", err)
	}

	unreachable := newValidationBatchFixture(t, "display_certificate_unreachable")
	falsity, err := unreachable.manager.MkFalse()
	pdrMust(t, err)
	rules := updateBatchRuleSet(t, unreachable, unreachable.declaration, falsity)
	boolean, err := unreachable.manager.MkBoolSort()
	pdrMust(t, err)
	other := pdrFunction(t, unreachable.manager, "display_certificate_invariant", nil, boolean)
	pdrMust(t, unreachable.ruleManager.RegisterPredicate(other))
	otherHead, err := unreachable.manager.MkApp(other, nil)
	pdrMust(t, err)
	otherRule, err := unreachable.ruleManager.NewRule(otherHead, nil, nil, "invariant-fact")
	pdrMust(t, err)
	pdrMust(t, rules.Add(otherRule))
	pdrMust(t, otherRule.Close())
	defer func() { pdrMust(t, rules.Close()) }()
	pdrMust(t, unreachable.context.UpdateRules(rules))
	status, err = unreachable.context.Solve(unreachable.declaration)
	pdrMust(t, err)
	if status != lbool.False {
		t.Fatal("unreachable certificate setup", status)
	}
	output.Reset()
	pdrMust(t, unreachable.context.DisplayCertificate(&output, unreachable.declaration, false))
	if !strings.Contains(output.String(), "(define-fun display_certificate_invariant () Bool") || !strings.Contains(output.String(), "true") {
		t.Fatalf("unreachable model certificate = %q", output.String())
	}
	levels, err := unreachable.context.NumLevels(other)
	pdrMust(t, err)
	for level := 0; level <= int(levels)+1; level++ {
		pdrMust(t, unreachable.context.AddCover(level, other, falsity))
	}
	output.Reset()
	pdrMust(t, unreachable.context.DisplayCertificate(&output, unreachable.declaration, false))
	if !strings.Contains(output.String(), "false") {
		t.Fatalf("finite-frame certificate ignored finite cover = %q", output.String())
	}
	// context::get_model and display_certificate both materialize
	// get_level_property(m_inductive_lvl), not the infinity-only cover.
	inductive, err := unreachable.context.Model(unreachable.declaration)
	pdrMust(t, err)
	defer func() { pdrMust(t, inductive.Close()) }()
	globals, err := unreachable.command.GlobalParameters()
	pdrMust(t, err)
	var modelOutput bytes.Buffer
	pdrMust(t, inductive.Core().WriteSMT2(&modelOutput, globals.Context, globals, 0))
	if !strings.Contains(modelOutput.String(), "false") {
		t.Fatalf("inductive model ignored finite cover = %q", modelOutput.String())
	}
}

// TestPDRProofTraceInstantiatesRules: model_search::get_proof_trace
// instantiates each node's rule with mk_instantiate before resolving its
// children (AUDIT.md muz_pdr_02-F2). The root proof's second premise is the
// derivation of the ground fact (inv 2), not a quantified rule.
func TestPDRProofTraceInstantiatesRules(t *testing.T) {
	command, err := cmd_context.NewStandaloneContext()
	pdrMust(t, err)
	defer func() { pdrMust(t, command.Close()) }()
	script := `(set-option :fixedpoint.engine pdr)
(set-option :fixedpoint.generate_proof_trace true)
(declare-rel inv (Int))
(declare-rel err ())
(declare-var x Int)
(rule (=> (= x 0) (inv x)))
(rule (=> (and (inv x) (< x 3)) (inv (+ x 1))))
(rule (=> (and (inv x) (= x 2)) err))
(query err)
`
	var output bytes.Buffer
	pdrMust(t, smt2.ParseCommands(command, strings.NewReader(script), &output))
	if output.String() != "sat\n" {
		t.Fatalf("query output = %q", output.String())
	}
	manager, err := command.Manager()
	pdrMust(t, err)
	predicates, err := command.FixedpointPredicates()
	pdrMust(t, err)
	var inv, goal ast.ASTHandle
	for _, declaration := range predicates {
		name, x := manager.GetASTName(declaration)
		pdrMust(t, x)
		text, x := manager.Symbols.String(name)
		pdrMust(t, x)
		switch text {
		case "inv":
			inv = declaration
		case "err":
			goal = declaration
		}
	}
	if inv.IsNull() || goal.IsNull() {
		t.Fatal("missing declarations")
	}
	proof, err := command.FixedpointProof(goal)
	pdrMust(t, err)
	defer func() { pdrMust(t, manager.DelRef(&proof)) }()
	root, err := manager.RefGet(proof)
	pdrMust(t, err)
	hyper, err := manager.IsProofKind(root, ast.ASTPrHyperResolve)
	pdrMust(t, err)
	count, err := manager.GetProofNumParents(root)
	pdrMust(t, err)
	if !hyper || count != 2 {
		t.Fatal("root is not a two-premise hyper-resolution", hyper, count)
	}
	second, err := manager.GetProofParent(root, 1)
	pdrMust(t, err)
	fact, err := manager.GetProofFact(second)
	pdrMust(t, err)
	two, err := manager.ArithmeticIntegerNumeralText("2")
	pdrMust(t, err)
	want, err := manager.MkApp(inv, []ast.ASTHandle{two})
	pdrMust(t, err)
	if fact != want {
		var text bytes.Buffer
		pdrMust(t, ast.DisplayLegacySMT(&text, manager, fact, 0, nil))
		t.Fatal("second premise does not conclude (inv 2):", text.String())
	}
}
