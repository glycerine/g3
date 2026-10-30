package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDatalogFrontendSaturatesOnceAndCopiesAllRelationTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.dl")
	program := "D 2\n\nP(x:D)\nQ(x:D)\nP(\"a\").\nQ(X) :- P(X).\n"
	if err := os.WriteFile(path, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	frontend, err := NewDatalogFrontend(datalogFrontendContext(t), &output)
	if err != nil {
		t.Fatal(err)
	}
	if err = frontend.Run(path); err != nil {
		t.Fatal(err)
	}
	snapshots, err := frontend.RelationSnapshots()
	if err != nil || len(snapshots) != 2 || len(snapshots[0].Facts) != 1 || len(snapshots[1].Facts) != 1 {
		t.Fatalf("snapshots %#v error %v", snapshots, err)
	}
	snapshots[0].Facts[0][0] = 99
	again, err := frontend.RelationSnapshots()
	if err != nil || again[0].Facts[0][0] == 99 {
		t.Fatalf("aliased snapshots %#v error %v", again, err)
	}
}
