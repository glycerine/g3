package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOPBFrontendFileStatisticsAndDocumentCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.opb")
	if err := os.WriteFile(path, []byte("1 x1 = 1;"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	f, err := NewOPBFrontend(opbFrontendContext(t), &output, OPBFrontendOptions{DisplayStatistics: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.RunFile(path); err != nil {
		t.Fatal(err)
	}
	if text := output.String(); !strings.HasPrefix(text, "sat\n") || !strings.Contains(text, ":total-time") {
		t.Fatal(text)
	}
	document, err := f.Document()
	if err != nil || len(document.Constraints) != 1 {
		t.Fatal(document, err)
	}
	document.Constraints[0].Bound.SetInt64(9)
	again, _ := f.Document()
	if again.Constraints[0].Bound.String() != "1" {
		t.Fatal("document accessor aliases frontend")
	}
}
