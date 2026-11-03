package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDatalogFrontendEngineSelectionPreservesOtherOptions(t *testing.T) {
	context := datalogFrontendContext(t)
	globals, err := context.GlobalParameters()
	if err != nil {
		t.Fatal(err)
	}
	if err = globals.SourceSetName("fixedpoint.engine", "spacer"); err != nil {
		t.Fatal(err)
	}
	parameters := globals.Context.EmptyValue()
	defer func() {
		if err := globals.Context.Del(&parameters); err != nil {
			t.Fatal(err)
		}
	}()
	for _, name := range []string{"datalog.use_map_names", "datalog.print.tuples"} {
		if err = globals.Context.SetBoolName(&parameters, &name, false); err != nil {
			t.Fatal(err)
		}
	}
	if err = context.SetFixedpointParameters(&parameters); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	for _, file := range []struct{ name, contents string }{
		{"V.map", "7 alpha\n19 alpha\n"},
		{"P.rules", "P(x:V) outputtuples\n"},
		{"P.rel", "19\n7\n"},
	} {
		if err = os.WriteFile(filepath.Join(directory, file.name), []byte(file.contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	frontend, err := NewDatalogFrontend(context, &output)
	if err != nil {
		t.Fatal(err)
	}
	if err = frontend.Run(directory); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "Time: ") || strings.Contains(output.String(), "Tuples in ") {
		t.Fatalf("engine choice replaced the local output option: %q", output.String())
	}
	rows, err := frontend.RelationSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !reflect.DeepEqual(rows[0].Facts, [][]uint64{{0}, {1}}) {
		t.Fatal("engine choice replaced the local numeric domain option", rows)
	}
	moduleEngine, err := globals.SourceGetValueName("fixedpoint.engine")
	if err != nil || moduleEngine != "spacer" {
		t.Fatal("frontend changed the module engine option", moduleEngine, err)
	}
}
