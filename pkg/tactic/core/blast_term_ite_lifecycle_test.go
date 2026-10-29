package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp standalone helper,
// statistics, cleanup, parameter update, and translation.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestBlastTermITEStandaloneStatisticsCleanupAndTranslation(t *testing.T) {
	f := newBlastTermITEFixture(t, false)
	choice, err := f.m.MkIte(f.cond, f.one, f.two)
	simplifyMust(t, err)
	app, err := f.m.MkApp(f.f, []ast.ASTHandle{choice})
	simplifyMust(t, err)
	formula, err := f.m.MkEq(app, f.one)
	simplifyMust(t, err)
	standalone, err := BlastTermITE(f.m, formula)
	simplifyMust(t, err)
	if standalone == formula {
		t.Fatal("standalone helper did not blast term ITE")
	}
	goal := f.goal(t, formula, false, false)
	var result tactic.TacticResult
	t.Cleanup(func() { simplifyMust(t, result.Close()) })
	simplifyMust(t, f.runner.Apply(goal, &result))
	stats := f.runner.CollectStatistics()
	if stats["blast-term-ite-consts"] != 2 {
		t.Fatal("expansion statistic", stats)
	}
	simplifyMust(t, f.runner.Cleanup())
	if f.runner.CollectStatistics() != nil {
		t.Fatal("cleanup did not replace source implementation state")
	}
	p, err := f.runner.context.NewValue()
	simplifyMust(t, err)
	t.Cleanup(func() { simplifyMust(t, f.runner.context.Del(&p)) })
	key := "max_memory"
	simplifyMust(t, f.runner.context.SetUIntName(&p, &key, 7))
	simplifyMust(t, f.runner.UpdateParams(&p))
	copy, err := f.runner.Translate(f.m)
	simplifyMust(t, err)
	translated := copy.(*BlastTermITETactic)
	if translated.maxMemory != f.runner.maxMemory {
		t.Fatal("translation lost parameter snapshot")
	}
	simplifyMust(t, translated.Close())
}
