package cmd_context

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWPANumericDomainsInternOriginalValuesOnFirstUse(t *testing.T) {
	directory := wpaSemanticDirectory(t, map[string]string{
		"program.rules": "P(b:BOOL,s:SHORT)\n",
		"P.rel":         "19 18446744073709551615\n7 42\n19 42\n",
	})
	context := contextFixture(t)
	contextMust(t, context.ParseWPADirectory(directory))
	rows := wpaSemanticSnapshots(t, context, "P")[0].Facts
	if !reflect.DeepEqual(rows, [][]uint64{{0, 0}, {1, 1}, {0, 1}}) {
		t.Fatal("numeric domain keys were used as raw or preassigned table indices", rows)
	}
}

func TestWPANumericDomainsRejectSymbolicRuleConstants(t *testing.T) {
	for _, name := range []string{"BOOL", "SHORT", "V"} {
		t.Run(name, func(t *testing.T) {
			directory := wpaSemanticDirectory(t, map[string]string{
				"V.map":         "7 seven\n",
				"program.rules": "P(x:" + name + ")\nP(7).\n",
			})
			context := contextFixture(t)
			globals, err := context.GlobalParameters()
			contextMust(t, err)
			contextMust(t, globals.SourceSetName("fixedpoint.datalog.use_map_names", "false"))
			if err = context.ParseWPADirectory(directory); err == nil {
				t.Fatal("numeric WPA domain accepted the source symbolic-rule overload")
			}
		})
	}
}

func TestWPAParserSnapshotsMapNameOptionBeforeLoading(t *testing.T) {
	directory := wpaSemanticDirectory(t, map[string]string{
		"V.map":         "7 alpha\n19 alpha\n",
		"program.rules": "P(x:V)\n",
		"P.rel":         "19\n7\n",
	})
	context := contextFixture(t)
	parser, err := newWPAParser(context, directory)
	contextMust(t, err)
	defer func() { contextMust(t, parser.rules.close()) }()
	globals, err := context.GlobalParameters()
	contextMust(t, err)
	contextMust(t, globals.SourceSetName("fixedpoint.datalog.use_map_names", "false"))
	contextMust(t, parser.readMaps())
	contextMust(t, parser.finishSorts())
	contextMust(t, parser.readRules())
	contextMust(t, parser.indexDeclarations())
	contextMust(t, parser.readRelations())
	rows := wpaSemanticSnapshots(t, context, "P")[0].Facts
	if !reflect.DeepEqual(rows, [][]uint64{{0}}) {
		t.Fatal("parser reread use_map_names after construction", rows)
	}
}

func TestWPAMapsWithSameBasenameMergeMembership(t *testing.T) {
	directory := wpaSemanticDirectory(t, map[string]string{
		"V.map":         "7 alpha\n",
		"program.rules": "P(x:V)\n",
		"P.rel":         "7\n19\n99\n",
	})
	nested := filepath.Join(directory, "nested")
	contextMust(t, os.Mkdir(nested, 0o700))
	contextMust(t, os.WriteFile(filepath.Join(nested, "V.map"), []byte("19 beta\n"), 0o600))
	context := contextFixture(t)
	contextMust(t, context.ParseWPADirectory(directory))
	rows := wpaSemanticSnapshots(t, context, "P")[0].Facts
	if !reflect.DeepEqual(rows, [][]uint64{{0}, {1}}) {
		t.Fatal("repeated sort maps lost membership or accepted an unknown input", rows)
	}
}
