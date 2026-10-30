package smt2

import (
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
)

type consequenceRefusingWriter struct{ err error }

func (w consequenceRefusingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestGetConsequencesPropagatesStatusOutputFailure(t *testing.T) {
	c, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	refusal := errors.New("consequence output refused")
	err = ParseCommands(c, strings.NewReader(`(set-logic QF_UF)(get-consequences () ())`), consequenceRefusingWriter{refusal})
	if err == nil || !strings.Contains(err.Error(), refusal.Error()) {
		t.Fatal(err)
	}
}
