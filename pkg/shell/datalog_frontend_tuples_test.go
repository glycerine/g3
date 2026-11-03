package shell

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func datalogTupleFile(t *testing.T, program string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tuples.dl")
	if err := os.WriteFile(path, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func datalogTupleOutputBeforeStatistics(t *testing.T, output, want string) {
	t.Helper()
	if !strings.HasPrefix(output, want+"Time: ") || !strings.Contains(output[len(want):], "\nParsing: ") {
		t.Fatalf("tuple output before statistics = %q, want tuple prefix %q", output, want)
	}
}

func TestDatalogFrontendTuplesOrdinaryNamesAndPragmas(t *testing.T) {
	path := datalogTupleFile(t, "D 3\n\nHidden(x:D)\nP(x:D) printtuples\nEmpty(x:D) outputtuples\nTruth outputtuples\nHidden(\"alpha\").\nP(X) :- Hidden(X).\nTruth.\n")
	var output bytes.Buffer
	frontend, err := NewDatalogFrontend(datalogFrontendContext(t), &output)
	if err != nil {
		t.Fatal(err)
	}
	if err = frontend.Run(path); err != nil {
		t.Fatal(err)
	}
	want := "Tuples in P: \n\t(x=alpha(0))\nTuples in Empty: \nTuples in Truth: \n\t()\n"
	datalogTupleOutputBeforeStatistics(t, output.String(), want)
	results, err := frontend.Results()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 || results[0].Name != "Hidden" {
		t.Fatalf("tuple selection changed all-predicate results: %v", results)
	}
	var repeated bytes.Buffer
	if err = frontend.WriteTuples(&repeated); err != nil {
		t.Fatal(err)
	}
	if repeated.String() != want {
		t.Fatalf("repeated tuple output = %q, want %q", repeated.String(), want)
	}
}

func TestDatalogFrontendTuplesWPAMappedAndNumericNames(t *testing.T) {
	for _, tc := range []struct {
		name, option, want string
	}{
		{name: "source-default-names", want: "Tuples in P: \n\t(x=alpha(0))\n"},
		{name: "numeric-identities", option: "false", want: "Tuples in P: \n\t(x=19(0))\n\t(x=7(1))\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			for name, contents := range map[string]string{
				"V.map":         "7 alpha\n19 alpha\n",
				"program.rules": "P(x:V) printtuples\nHidden(x:V)\n",
				"P.rel":         "19\n7\n",
			} {
				if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			context := datalogFrontendContext(t)
			if tc.option != "" {
				globals, err := context.GlobalParameters()
				if err != nil {
					t.Fatal(err)
				}
				if err = globals.SourceSetName("fixedpoint.datalog.use_map_names", tc.option); err != nil {
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
			datalogTupleOutputBeforeStatistics(t, output.String(), tc.want)
		})
	}
}

func TestDatalogFrontendTuplesModuleParameterSuppressesOutput(t *testing.T) {
	context := datalogFrontendContext(t)
	globals, err := context.GlobalParameters()
	if err != nil {
		t.Fatal(err)
	}
	if err = globals.SourceSetName("fixedpoint.datalog.print.tuples", "false"); err != nil {
		t.Fatal(err)
	}
	path := datalogTupleFile(t, "D 1\n\nP(x:D) printtuples\nP(\"alpha\").\n")
	var output bytes.Buffer
	frontend, err := NewDatalogFrontend(context, &output)
	if err != nil {
		t.Fatal(err)
	}
	if err = frontend.Run(path); err != nil {
		t.Fatal(err)
	}
	datalogTupleOutputBeforeStatistics(t, output.String(), "")
	if strings.Contains(output.String(), "Tuples in ") {
		t.Fatalf("disabled tuple output = %q", output.String())
	}
	snapshots, err := frontend.RelationSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 || !reflect.DeepEqual(snapshots[0].Facts, [][]uint64{{0}}) {
		t.Fatalf("disabled display changed saturated rows: %v", snapshots)
	}
}

type datalogTupleRefusingWriter struct{ remaining int }

func (w *datalogTupleRefusingWriter) Write(data []byte) (int, error) {
	if len(data) > w.remaining {
		n := w.remaining
		w.remaining = 0
		return n, io.ErrClosedPipe
	}
	w.remaining -= len(data)
	return len(data), nil
}

func TestDatalogFrontendTuplesWriterRefusalPreservesResults(t *testing.T) {
	path := datalogTupleFile(t, "D 1\n\nP(x:D) outputtuples\nP(\"alpha\").\n")
	frontend, err := NewDatalogFrontend(datalogFrontendContext(t), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err = frontend.Load(path); err != nil {
		t.Fatal(err)
	}
	if err = frontend.Saturate(); err != nil {
		t.Fatal(err)
	}
	beforeResults, err := frontend.Results()
	if err != nil {
		t.Fatal(err)
	}
	beforeRows, err := frontend.RelationSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	want := "Tuples in P: \n\t(x=alpha(0))\n"
	for _, limit := range []int{0, len("Tuples in P: \n\t(x=al")} {
		if err = frontend.WriteTuples(&datalogTupleRefusingWriter{remaining: limit}); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("writer refusal at byte %d = %v", limit, err)
		}
		afterResults, err := frontend.Results()
		if err != nil {
			t.Fatal(err)
		}
		afterRows, err := frontend.RelationSnapshots()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(afterResults, beforeResults) || !reflect.DeepEqual(afterRows, beforeRows) {
			t.Fatalf("writer refusal changed results: %v, rows: %v", afterResults, afterRows)
		}
		var retry bytes.Buffer
		if err = frontend.WriteTuples(&retry); err != nil {
			t.Fatal(err)
		}
		if retry.String() != want {
			t.Fatalf("read-only tuple retry = %q, want %q", retry.String(), want)
		}
	}
}
