package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestDatalogFrontendDirectoryDispatchUsesWPA(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "V.map"), []byte("7 seven\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "p.rules"), []byte("P(x:V)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "P.rel"), []byte("7\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	frontend, err := NewDatalogFrontend(datalogFrontendContext(t), &output)
	if err != nil {
		t.Fatal(err)
	}
	if err = frontend.Run(directory); err != nil {
		t.Fatal(err)
	}
	results, err := frontend.Results()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Name != "P" || results[0].Status != lbool.True {
		t.Fatal(results)
	}
}
