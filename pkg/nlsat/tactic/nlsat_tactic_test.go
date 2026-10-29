package tactic

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestNLSATTacticAdvertisesSourceParameterCatalog(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	value, err := NewNLSATTactic(fixture.manager, fixture.context, fixture.globals, nil, fixture.services)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, value.Close()) })
	descriptions, err := uparams.NewParamDescrs(fixture.manager.Symbols, uparams.ParamDescrsConfig{Budget: fixture.manager.Budget()})
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, descriptions.Close()) })
	compilerMust(t, value.CollectParamDescrs(descriptions))
	want := []struct {
		name, description string
		kind              uparams.ParamKind
	}{
		{"max_memory", "(default: infty) maximum amount of memory in megabytes.", uparams.ParamUInt},
		{"lazy", "how lazy the solver is.", uparams.ParamUInt},
		{"reorder", "reorder variables.", uparams.ParamBool},
		{"simplify_conflicts", "simplify conflicts using equalities before resolving them.", uparams.ParamBool},
		{"minimize_conflicts", "minimize conflicts", uparams.ParamBool},
		{"randomize", "randomize selection of a witness.", uparams.ParamBool},
		{"max_conflicts", "maximum number of conflicts.", uparams.ParamUInt},
		{"shuffle_vars", "use a random variable order.", uparams.ParamBool},
		{"inline_vars", "inline variables that can be isolated from equations", uparams.ParamBool},
		{"seed", "random seed.", uparams.ParamUInt},
		{"factor", "factor polynomials produced during conflict resolution.", uparams.ParamBool},
	}
	size, err := descriptions.Size()
	compilerMust(t, err)
	if size != uint32(len(want)) {
		t.Fatalf("descriptor count = %d, want %d", size, len(want))
	}
	for i, entry := range want {
		name, cause := descriptions.ParamName(uint32(i))
		compilerMust(t, cause)
		got, cause := fixture.manager.Symbols.String(name)
		compilerMust(t, cause)
		if got != entry.name {
			t.Fatalf("descriptor %d = %q, want %q", i, got, entry.name)
		}
		kind, cause := descriptions.Kind(name)
		compilerMust(t, cause)
		if kind != entry.kind {
			t.Fatalf("%s kind = %v, want %v", got, kind, entry.kind)
		}
		description, present, cause := descriptions.Description(name)
		compilerMust(t, cause)
		if !present || description != entry.description {
			t.Fatalf("%s description = %q, present %v, want %q", got, description, present, entry.description)
		}
	}
}

func TestNLSATTacticRetainsParametersAcrossTranslation(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	value, err := NewNLSATTactic(fixture.manager, fixture.context, fixture.globals, nil, fixture.services)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, value.Close()) })
	parameters, err := fixture.context.NewValue()
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, fixture.context.Del(&parameters)) })
	name, seed := "seed", uint32(23)
	compilerMust(t, fixture.context.SetUIntName(&parameters, &name, seed))
	compilerMust(t, value.UpdateParams(&parameters))
	translated, err := value.Translate(fixture.manager)
	compilerMust(t, err)
	copy, ok := translated.(*NLSATTactic)
	if !ok {
		t.Fatal("translated NLSAT tactic type")
	}
	t.Cleanup(func() { compilerMust(t, copy.Close()) })
	got, err := fixture.context.GetUIntName(&copy.params, &name, 0)
	compilerMust(t, err)
	if got != seed {
		t.Fatal("translated NLSAT seed", got)
	}
	compilerMust(t, value.Cleanup())
}

func TestNLSATTacticPublishesSatModel(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	tacticValue, err := NewNLSATTactic(fixture.manager, fixture.context, fixture.globals, nil, fixture.services)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, tacticValue.Close()) })
	half, err := fixture.manager.ArithmeticRealNumeralText("1/2")
	compilerMust(t, err)
	one, err := fixture.manager.ArithmeticRealNumeralText("1")
	compilerMust(t, err)
	sort, err := fixture.manager.GetSort(one)
	compilerMust(t, err)
	x := compilerConst(t, fixture.manager, "tactic-x", sort)
	declaration, err := fixture.manager.GetDecl(x)
	compilerMust(t, err)
	sum, err := fixture.manager.MkArithmeticAdd(x, half)
	compilerMust(t, err)
	equality, err := fixture.manager.MkEq(sum, one)
	compilerMust(t, err)
	goal := compilerGoal(t, fixture.manager, equality)
	var result basetactic.TacticResult
	compilerMust(t, tacticValue.Apply(goal, &result))
	t.Cleanup(func() { compilerMust(t, result.Close()) })
	if !result.DecidedSat() || result.ModelConverter == nil {
		t.Fatalf("SAT result: decided=%v, converter=%T", result.DecidedSat(), result.ModelConverter)
	}
	var output basetactic.ModelRef
	compilerMust(t, result.ModelConverter.ApplyUnindexed(&output))
	defer func() { compilerMust(t, output.Close()) }()
	modelValue, err := output.Get()
	compilerMust(t, err)
	interpretation, err := modelValue.ConstantInterpretation(declaration)
	compilerMust(t, err)
	text, integer, matched, err := fixture.manager.ArithmeticNumeralText(interpretation)
	compilerMust(t, err)
	if !matched || integer || text != "1/2" {
		t.Fatalf("tactic model x = %q, integer=%v, matched=%v", text, integer, matched)
	}
}

func TestNLSATTacticPublishesUnsatCore(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	tacticValue, err := NewNLSATTactic(fixture.manager, fixture.context, fixture.globals, nil, fixture.services)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, tacticValue.Close()) })
	zero, err := fixture.manager.ArithmeticRealNumeralText("0")
	compilerMust(t, err)
	one, err := fixture.manager.ArithmeticRealNumeralText("1")
	compilerMust(t, err)
	sort, err := fixture.manager.GetSort(zero)
	compilerMust(t, err)
	x := compilerConst(t, fixture.manager, "core-x", sort)
	upper, err := fixture.manager.MkArithmeticLE(x, zero)
	compilerMust(t, err)
	lower, err := fixture.manager.MkArithmeticGE(x, one)
	compilerMust(t, err)
	booleanSort, err := fixture.manager.MkBoolSort()
	compilerMust(t, err)
	labels := []ast.ASTHandle{
		compilerConst(t, fixture.manager, "core-upper", booleanSort),
		compilerConst(t, fixture.manager, "core-lower", booleanSort),
	}
	goal, err := basetactic.NewGoal(fixture.manager, false, false, true)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, goal.Close()) })
	for i, formula := range []ast.ASTHandle{upper, lower} {
		dependency, cause := fixture.manager.DependencyLeaf(labels[i])
		compilerMust(t, cause)
		compilerMust(t, goal.AssertWithProof(formula, ast.ASTHandle{}, dependency))
	}
	var result basetactic.TacticResult
	compilerMust(t, tacticValue.Apply(goal, &result))
	t.Cleanup(func() { compilerMust(t, result.Close()) })
	if !result.DecidedUnsat() {
		t.Fatal("NLSAT tactic did not decide contradiction")
	}
	leaves, err := fixture.manager.DependencyLinearize(result.Core, nil)
	compilerMust(t, err)
	if len(leaves) != 2 {
		t.Fatalf("tactic core = %#v", leaves)
	}
	statistics := tacticValue.CollectStatistics()
	if len(statistics) != 5 || statistics["nlsat conflicts"] == 0 || statistics["nlsat stages"] == 0 {
		t.Fatal("NLSAT tactic statistics", statistics)
	}
	compilerMust(t, tacticValue.ResetStatistics())
	if statistics = tacticValue.CollectStatistics(); len(statistics) != 0 {
		t.Fatal("NLSAT tactic statistics reset", statistics)
	}
}

func TestNLSATTacticRejectsProofGoal(t *testing.T) {
	fixture := newGoalCompilerFixture(t)
	compilerMust(t, fixture.manager.ToggleProofMode(ast.ASTProofEnabled))
	tacticValue, err := NewNLSATTactic(fixture.manager, fixture.context, fixture.globals, nil, fixture.services)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, tacticValue.Close()) })
	booleanSort, err := fixture.manager.MkBoolSort()
	compilerMust(t, err)
	p := compilerConst(t, fixture.manager, "proof-p", booleanSort)
	goal, err := basetactic.NewGoal(fixture.manager, true, false, false)
	compilerMust(t, err)
	t.Cleanup(func() { compilerMust(t, goal.Close()) })
	compilerMust(t, goal.Assert(p))
	var result basetactic.TacticResult
	if err = tacticValue.Apply(goal, &result); err == nil || err.Error() != "nlsat does not support proof production" {
		t.Fatalf("proof guard = %v", err)
	}
}
