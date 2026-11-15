package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func smtRegistryQuantifier(t *testing.T, m *ast.ASTManager, name string, weight int32) ast.ASTHandle {
	t.Helper()
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	variable, err := m.MkVar(0, boolean)
	astTestMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true, Sorts: []ast.ASTHandle{boolean}, Names: []symbol.Symbol{astTestSymbol(t, m, name+"-x")},
		Body: variable, Weight: weight, QID: astTestSymbol(t, m, name),
	})
	astTestMust(t, err)
	return quantifier
}

func TestSMTQuantifierRegistryStatsFingerprintQueueAndSourceGenerationForwarding(t *testing.T) {
	m := astTestManager(t)
	costs, err := NewSMTQICostModel(m, "max_top_generation", "cost", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, costs.Close()) })
	schedule := NewSMTQIQueueSchedule(m, 10, 20, false)
	registry, err := NewSMTQuantifierRegistry(m, schedule, costs, 20)
	astTestMust(t, err)
	quantifier := smtRegistryQuantifier(t, m, "registry-q", 2)
	astTestMust(t, registry.Add(quantifier, 5))
	if registry.Empty() || registry.NumQuantifiers() != 1 {
		t.Fatal("quantifier registration")
	}
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	_, first := smtEnodeConstant(t, m, 2110, boolean, 0)
	inserted, err := registry.AddInstance(quantifier, ast.ASTHandle{}, []*SMTEnode{first}, 3, 2, 99)
	astTestMust(t, err)
	if !inserted || len(schedule.newEntries) != 1 || schedule.newEntries[0].Generation != 5 || schedule.newEntries[0].Cost != 3 {
		t.Fatal("generation maximum or public max-top forwarding", inserted, schedule.newEntries)
	}
	stat, err := registry.Stat(quantifier)
	astTestMust(t, err)
	if stat.MaxGeneration() != 5 || stat.MaxCost() != 3 {
		t.Fatal("quantifier statistic updates", stat.MaxGeneration(), stat.MaxCost())
	}
	inserted, err = registry.AddInstance(quantifier, ast.ASTHandle{}, []*SMTEnode{first}, 3, 2, 99)
	astTestMust(t, err)
	if inserted || len(schedule.newEntries) != 1 {
		t.Fatal("duplicate fingerprint", inserted, schedule.newEntries)
	}
	stat.IncrementInstances()
	if stat.NumInstancesCurrentSearch() != 1 {
		t.Fatal("test setup current search")
	}
	astTestMust(t, registry.InitSearch())
	if stat.NumInstancesCurrentSearch() != 0 || registry.numInstances != 0 {
		t.Fatal("search initialization", stat.NumInstancesCurrentSearch(), registry.numInstances)
	}
}

func TestSMTQuantifierRegistrySourceInstanceLimitScopesAndStackDeletion(t *testing.T) {
	m := astTestManager(t)
	costs, err := NewSMTQICostModel(m, "0", "0", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, costs.Close()) })
	schedule := NewSMTQIQueueSchedule(m, 1, 1, false)
	registry, err := NewSMTQuantifierRegistry(m, schedule, costs, 0)
	astTestMust(t, err)
	firstQuantifier := smtRegistryQuantifier(t, m, "registry-first", 1)
	secondQuantifier := smtRegistryQuantifier(t, m, "registry-second", 1)
	astTestMust(t, registry.Add(firstQuantifier, 0))
	astTestMust(t, registry.Add(secondQuantifier, 0))
	boolean, err := m.MkBoolSort()
	astTestMust(t, err)
	_, first := smtEnodeConstant(t, m, 2120, boolean, 0)
	_, second := smtEnodeConstant(t, m, 2121, boolean, 0)
	inserted, err := registry.AddInstance(firstQuantifier, ast.ASTHandle{}, []*SMTEnode{first}, 0, 0, 0)
	astTestMust(t, err)
	if !inserted {
		t.Fatal("source > limit permits first instance at limit zero")
	}
	inserted, err = registry.AddInstance(firstQuantifier, ast.ASTHandle{}, []*SMTEnode{second}, 0, 0, 0)
	astTestMust(t, err)
	if inserted {
		t.Fatal("instance beyond source maximum")
	}
	_, err = schedule.ScheduleNew(nil)
	astTestMust(t, err)
	registry.maxInstances = 10
	astTestMust(t, registry.PushScope())
	inserted, err = registry.AddInstance(secondQuantifier, ast.ASTHandle{}, []*SMTEnode{second}, 0, 0, 0)
	astTestMust(t, err)
	if !inserted {
		t.Fatal("scoped fingerprint insertion")
	}
	_, err = schedule.ScheduleNew(nil)
	astTestMust(t, err)
	astTestMust(t, registry.PopScopes(1))
	inserted, err = registry.AddInstance(secondQuantifier, ast.ASTHandle{}, []*SMTEnode{second}, 0, 0, 0)
	astTestMust(t, err)
	if !inserted {
		t.Fatal("popped fingerprint was not removed")
	}
	if err := registry.Delete(firstQuantifier); err != ast.ErrASTDomain {
		t.Fatal("quantifier deletion stack order", err)
	}
	astTestMust(t, registry.Delete(secondQuantifier))
	astTestMust(t, registry.Delete(firstQuantifier))
	if !registry.Empty() {
		t.Fatal("quantifier registry not empty")
	}
}

// AUDIT.md smtcore_06-F9: del reports display_stats on verbose_stream()
// when qi.profile is set; qi_queue::instantiate reports every
// qi.profile_freq instances.
func TestSMTQuantifierRegistryProfileDisplaysStats(t *testing.T) {
	m := astTestManager(t)
	costs, err := NewSMTQICostModel(m, "0", "0", nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, costs.Close()) })
	schedule := NewSMTQIQueueSchedule(m, 1, 1, false)
	registry, err := NewSMTQuantifierRegistry(m, schedule, costs, 10)
	astTestMust(t, err)
	quiet := smtRegistryQuantifier(t, m, "k!14", 1)
	quantifier := smtRegistryQuantifier(t, m, "k!15", 1)
	astTestMust(t, registry.Add(quiet, 0))
	astTestMust(t, registry.Add(quantifier, 0))
	stat, err := registry.Stat(quantifier)
	astTestMust(t, err)
	stat.IncrementInstances()
	stat.UpdateMaxGeneration(2)
	stat.UpdateMaxCost(3)

	old, err := diagnostic.BorrowProcessVerboseStream()
	astTestMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	astTestMust(t, err)
	t.Cleanup(func() {
		astTestMust(t, diagnostic.SetProcessVerboseStream(old))
		astTestMust(t, stream.Close())
	})
	astTestMust(t, diagnostic.SetProcessVerboseStream(stream))
	captured := func() string {
		t.Helper()
		text, err := stream.String()
		astTestMust(t, err)
		return text
	}

	const line = "[quantifier_instances]       k!15 :      1 :   2 : 3\n"
	astTestMust(t, registry.profileInstance(quantifier, stat)) // default frequency UINT_MAX
	if got := captured(); got != "" {
		t.Fatal("profile report at the default frequency", got)
	}
	registry.profileFrequency = 1
	astTestMust(t, registry.profileInstance(quantifier, stat))
	if got := captured(); got != line {
		t.Fatalf("profile frequency report %q", got)
	}
	registry.profile = true
	astTestMust(t, registry.Delete(quantifier))
	// A quantifier without instances prints nothing.
	astTestMust(t, registry.Delete(quiet))
	if got := captured(); got != line+line {
		t.Fatalf("qi.profile deletion report %q", got)
	}
}
