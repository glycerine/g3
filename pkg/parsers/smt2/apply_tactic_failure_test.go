package smt2

import (
	"strings"
	"testing"
)

func TestSMT2ApplyTacticFailureIsCommandOutput(t *testing.T) {
	output, err := runApplyTacticScript(t, `(apply fail)(echo "continued")`)
	if err != nil {
		t.Fatal(err, output)
	}
	if !strings.Contains(output, `(error "tactic failed: fail tactic")`) || !strings.Contains(output, `continued`) {
		t.Fatal(output)
	}
}

func TestSMT2ApplyTacticRejectsMalformedOptions(t *testing.T) {
	for _, command := range []string{`(apply)`, `(apply skip :unknown true)`, `(apply skip :print 1)`, `(apply skip :timeout true)`} {
		if output, err := runApplyTacticScript(t, command); err == nil {
			t.Fatal("accepted", command, output)
		}
	}
}

func TestSMT2ApplyTacticAcceptsSourceOptions(t *testing.T) {
	output, err := runApplyTacticScript(t, `(apply skip :print false :print_proof true :timeout 10 :rlimit 0 :max_memory 0)`)
	if err != nil {
		t.Fatal(err, output)
	}
	if output != "" {
		t.Fatal(output)
	}
}
