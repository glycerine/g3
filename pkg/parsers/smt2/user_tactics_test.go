package smt2

import (
	"strings"
	"testing"
)

func TestSMT2UserTacticDeclarationLookupDisplayAndApply(t *testing.T) {
	script := `(declare-tactic custom (and-then simplify skip))
(declare-tactic alias custom)
(declare-tactic custom skip)
(get-user-tactics)
(declare-const p Bool)
(assert p)
(apply alias)`
	output, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, output)
	}
	// get_user_tactics_cmd separates entries with "\n " (z3 4.7.1 itself
	// segfaults here, AUDIT.md cmd_context_02).
	wantDeclarations := "((declare-tactic custom skip)\n (declare-tactic alias custom))\n"
	if !strings.HasPrefix(output, wantDeclarations) {
		t.Fatalf("user tactic display = %q, want prefix %q", output, wantDeclarations)
	}
	if !strings.Contains(output, "(goal\n  p\n  :precision precise :depth ") {
		t.Fatal("aliased user tactic was not applied", output)
	}
}

func TestSMT2UserTacticDeclarationValidatesBeforePublishing(t *testing.T) {
	output, err := runApplyTacticScript(t, `(declare-tactic invalid no-such-tactic)(get-user-tactics)`)
	if err == nil || !strings.Contains(err.Error(), "invalid tactic, unknown tactic") {
		t.Fatal("unknown tactic declaration accepted", output, err)
	}
	if strings.Contains(output, "invalid") {
		t.Fatal("failed tactic declaration was published", output)
	}
}
