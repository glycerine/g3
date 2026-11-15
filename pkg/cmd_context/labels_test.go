package cmd_context

// Source differential: src/cmd_context/basic_cmds.cpp labels_cmd.

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// labels_cmd writes each symbol with operator<<, without SMT2 quoting: z3
// 4.7.1 prints (labels a b) for the label |a b|. With no logic set, after a
// push the combined solver answers from the smt solver, which reports
// relevant labels. A first non-incremental check-sat, or a QF_UF context,
// answers from tactic2solver, whose get_labels is empty (AUDIT.md
// solver_misc_01-F1). Vector labels_unquoted.smt2 checks the
// same script against the C++.
func TestCommandContextWritesRelevantLabelsUnquoted(t *testing.T) {
	c := contextFixture(t)
	m, err := c.Manager()
	contextMust(t, err)
	b, err := m.MkBoolSort()
	contextMust(t, err)
	pName, err := m.Symbols.Intern("p")
	contextMust(t, err)
	qName, err := m.Symbols.Intern("q")
	contextMust(t, err)
	p, err := m.MkConst(pName, b)
	contextMust(t, err)
	q, err := m.MkConst(qName, b)
	contextMust(t, err)
	first, err := m.Symbols.Intern("first")
	contextMust(t, err)
	spaced, err := m.Symbols.Intern("a b")
	contextMust(t, err)
	left, err := m.MkLabel(true, []usymbol.Symbol{spaced}, p)
	contextMust(t, err)
	right, err := m.MkLabel(true, []usymbol.Symbol{first}, q)
	contextMust(t, err)
	disjunction, err := m.MkOr(left, right)
	contextMust(t, err)
	contextMust(t, c.Push(1))
	contextMust(t, c.Assert(disjunction, ""))
	status, err := c.CheckSat(nil)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatal("label status", status)
	}
	var output bytes.Buffer
	contextMust(t, c.WriteLabels(&output))
	if output.String() != "(labels a b)\n" {
		t.Fatalf("label output %q", output.String())
	}
}
