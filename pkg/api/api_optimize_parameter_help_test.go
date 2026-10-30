package api

import (
	"strings"
	"testing"
)

func TestAPIOptimizeParameterHelp(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	help, err := f.opt.ParameterHelp()
	astTestMust(t, err)
	for _, key := range []string{"priority", "maxsat_engine", "optsmt_engine"} {
		if !strings.Contains(help, key) {
			t.Fatal("help missing", key)
		}
	}
}
