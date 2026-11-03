package cmd_context

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func wpaSemanticDirectory(t *testing.T, files map[string]string) string {
	t.Helper()
	directory := t.TempDir()
	for name, contents := range files {
		contextMust(t, os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600))
	}
	return directory
}

func wpaSemanticSnapshots(t *testing.T, context *Context, names ...string) []FixedpointRelationSnapshot {
	t.Helper()
	declarations := make([]ast.ASTHandle, len(names))
	for i, name := range names {
		var err error
		declarations[i], err = context.FindFunctionDeclaration(name, nil, nil, ast.ASTHandle{})
		contextMust(t, err)
	}
	status, err := context.QueryFixedpointRelations(declarations)
	contextMust(t, err)
	if status != lbool.True {
		t.Fatalf("WPA relation query = %v, want sat", status)
	}
	snapshots, err := context.FixedpointRelationSnapshots(declarations)
	contextMust(t, err)
	if len(snapshots) != len(names) {
		t.Fatalf("got %d WPA snapshots, want %d", len(snapshots), len(names))
	}
	return snapshots
}

func TestWPASemanticsRuleConstantsShareTupleNames(t *testing.T) {
	directory := wpaSemanticDirectory(t, map[string]string{
		"V.map":         "7 alpha\n19 beta\n",
		"program.rules": "P(x:V)\nMatch\nMatch :- P(\"alpha\").\n",
		"P.rel":         "7\n",
	})
	context := contextFixture(t)
	contextMust(t, context.ParseWPADirectory(directory))
	snapshots := wpaSemanticSnapshots(t, context, "P", "Match")
	if !reflect.DeepEqual(snapshots[0].Facts, [][]uint64{{0}}) {
		t.Fatalf("tuple did not use the rule constant's first domain index: %v", snapshots[0].Facts)
	}
	if len(snapshots[1].Facts) != 1 || len(snapshots[1].Facts[0]) != 0 {
		t.Fatalf("quoted alpha did not match mapped tuple: %v", snapshots[1].Facts)
	}
}

func TestWPASemanticsMapNameOptionControlsConstantIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, global, local string
		want                [][]uint64
	}{
		{name: "default-names", want: [][]uint64{{0}}},
		{name: "global-numbers", global: "false", want: [][]uint64{{0}, {1}}},
		{name: "local-numbers", local: "false", want: [][]uint64{{0}, {1}}},
		{name: "local-names-override-global", global: "false", local: "true", want: [][]uint64{{0}}},
		{name: "local-numbers-override-global", global: "true", local: "false", want: [][]uint64{{0}, {1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := wpaSemanticDirectory(t, map[string]string{
				"V.map":         "7 alpha\n19 alpha\n",
				"program.rules": "P(x:V)\n",
				"P.rel":         "19\n7\n",
			})
			context := contextFixture(t)
			if tc.global != "" {
				globals, err := context.GlobalParameters()
				contextMust(t, err)
				contextMust(t, globals.SourceSetName("fixedpoint.datalog.use_map_names", tc.global))
			}
			if tc.local != "" {
				parameters, err := context.config.Params.NewValue()
				contextMust(t, err)
				defer func() { contextMust(t, context.config.Params.Del(&parameters)) }()
				name := "datalog.use_map_names"
				contextMust(t, context.config.Params.SetBoolName(&parameters, &name, tc.local == "true"))
				contextMust(t, context.SetFixedpointParameters(&parameters))
			}
			contextMust(t, context.ParseWPADirectory(directory))
			snapshots := wpaSemanticSnapshots(t, context, "P")
			if !reflect.DeepEqual(snapshots[0].Facts, tc.want) {
				t.Fatalf("mapped constant identities = %v, want %v", snapshots[0].Facts, tc.want)
			}
		})
	}
}

func TestWPASemanticsZeroUsesSourceSymbol(t *testing.T) {
	directory := wpaSemanticDirectory(t, map[string]string{
		"V.map":         "0 mapped-zero\n7 <zero element>\n19 mapped-zero\n",
		"program.rules": "P(x:V)\nZero\nMappedZero\nZero :- P(\"<zero element>\").\nMappedZero :- P(\"mapped-zero\").\n",
		"P.rel":         "0\n7\n",
	})
	context := contextFixture(t)
	contextMust(t, context.ParseWPADirectory(directory))
	snapshots := wpaSemanticSnapshots(t, context, "P", "Zero", "MappedZero")
	if len(snapshots[0].Facts) != 1 || len(snapshots[1].Facts) != 1 || len(snapshots[2].Facts) != 0 {
		t.Fatalf("zero did not alias its source symbol independently of map-zero spelling: P=%v Zero=%v MappedZero=%v", snapshots[0].Facts, snapshots[1].Facts, snapshots[2].Facts)
	}
}

func TestWPASemanticsGlobalFirstNameDeterminesLaterSortConstants(t *testing.T) {
	directory := wpaSemanticDirectory(t, map[string]string{
		"V.map":         "7 alpha\n",
		"W.map":         "7 beta\n",
		"program.rules": "P(x:V)\nQ(x:W)\nAlphaV\nAlphaW\nBetaW\nAlphaV :- P(\"alpha\").\nAlphaW :- Q(\"alpha\").\nBetaW :- Q(\"beta\").\n",
		"P.rel":         "7\n",
		"Q.rel":         "7\n",
	})
	context := contextFixture(t)
	contextMust(t, context.ParseWPADirectory(directory))
	snapshots := wpaSemanticSnapshots(t, context, "AlphaV", "AlphaW", "BetaW")
	if len(snapshots[0].Facts) != 1 || len(snapshots[1].Facts) != 1 || len(snapshots[2].Facts) != 0 {
		t.Fatalf("first global spelling did not govern both sorts: AlphaV=%v AlphaW=%v BetaW=%v", snapshots[0].Facts, snapshots[1].Facts, snapshots[2].Facts)
	}
}

func TestWPASemanticsDomainCapacityCountsDistinctMappedNumbersAndZeroSlot(t *testing.T) {
	for _, overflow := range []bool{false, true} {
		name := "within-capacity"
		if overflow {
			name = "overflow"
		}
		t.Run(name, func(t *testing.T) {
			rules := "P(x:V)\nP(\"alpha\").\nP(\"beta\").\nP(\"gamma\").\n"
			if overflow {
				rules += "P(\"delta\").\n"
			}
			directory := wpaSemanticDirectory(t, map[string]string{
				"V.map":         "0 zero\n7 alpha\n7 alpha\n",
				"program.rules": rules,
			})
			context := contextFixture(t)
			err := context.ParseWPADirectory(directory)
			if overflow {
				if err == nil {
					t.Fatal("duplicate map rows enlarged the distinct-number domain capacity")
				}
				return
			}
			contextMust(t, err)
			snapshots := wpaSemanticSnapshots(t, context, "P")
			if len(snapshots[0].Facts) != 3 {
				t.Fatalf("distinct mapped numbers plus one should allow three constants: %v", snapshots[0].Facts)
			}
		})
	}
}

func TestWPASemanticsSpaceOnlyLineAddsNullaryTuple(t *testing.T) {
	directory := wpaSemanticDirectory(t, map[string]string{
		"program.rules": "P\n",
		"P.rel":         "   \n",
	})
	context := contextFixture(t)
	contextMust(t, context.ParseWPADirectory(directory))
	snapshots := wpaSemanticSnapshots(t, context, "P")
	if len(snapshots[0].Facts) != 1 || len(snapshots[0].Facts[0]) != 0 {
		t.Fatalf("space-only relation line did not add the nullary tuple: %v", snapshots[0].Facts)
	}
}
