package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSMTContextRunWCNFFile(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	path := filepath.Join(t.TempDir(), "sample.wcnf")
	if err := os.WriteFile(path, []byte("p wcnf 1 1 5\n5 1 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := f.c.RunWCNFFile(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Output != "sat\n" || len(r.Document.Clauses) != 1 {
		t.Fatal(r.Output, r.Document)
	}
	if _, err = f.c.RunWCNFFile(filepath.Join(t.TempDir(), "missing.wcnf"), nil); err == nil {
		t.Fatal("missing file accepted")
	}
}
