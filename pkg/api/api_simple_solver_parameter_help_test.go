package api

import (
	"strings"
	"testing"
)

func TestAPISimpleSolverParameterHelpIncludesContextAndKernelCatalogs(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_UF", nil)
	s, err := f.c.NewSimpleSolver()
	astTestMust(t, err)
	help, err := s.ParameterHelp()
	astTestMust(t, err)
	for _, text := range []string{"proof generation", "model generation", "random_seed"} {
		if !strings.Contains(help, text) {
			t.Fatal("missing parameter help", text)
		}
	}
	astTestMust(t, s.Close())
}
