package opt

// Source: src/opt/opt_params.hpp values consumed by engine dispatch.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestMaxSMTConfigDecodesLocalEngineAndMaxResControls(t *testing.T) {
	context, globals, local, budget := optParamsFixture(t)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: context.SymbolsField, Numerals: context.NumeralsField, Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	modelConfig := model.ModelEvaluatorConfig{Context: context, Globals: globals, Params: local}
	engineKey := "maxsat_engine"
	pd, err := manager.Symbols.Intern("pd-maxres")
	if err != nil {
		t.Fatal(err)
	}
	if err = modelConfig.Context.SetSymbolName(modelConfig.Params, &engineKey, pd); err != nil {
		t.Fatal(err)
	}
	hillKey := "maxres.hill_climb"
	if err = modelConfig.Context.SetBoolName(modelConfig.Params, &hillKey, false); err != nil {
		t.Fatal(err)
	}
	upperKey := "maxres.add_upper_bound_block"
	if err = modelConfig.Context.SetBoolName(modelConfig.Params, &upperKey, true); err != nil {
		t.Fatal(err)
	}
	slsKey := "enable_sls"
	if err = modelConfig.Context.SetBoolName(modelConfig.Params, &slsKey, true); err != nil {
		t.Fatal(err)
	}
	correctionKey := "maxres.max_correction_set_size"
	if err = modelConfig.Context.SetUIntName(modelConfig.Params, &correctionKey, 7); err != nil {
		t.Fatal(err)
	}
	config, err := loadMaxSMTConfig(&ArithmeticSolver{manager: manager, config: smtcore.SMTSolverConfig{Params: modelConfig.Context, Globals: modelConfig.Globals, ModelEvaluator: modelConfig}})
	if err != nil {
		t.Fatal(err)
	}
	if config.engine != "pd-maxres" || !config.enableSLS || config.hillClimb || !config.addUpperBoundBlock || config.maxCorrectionSetSize != 7 || !config.pivotOnCorrectionSet {
		t.Fatal("decoded MaxSMT config", config)
	}
}
