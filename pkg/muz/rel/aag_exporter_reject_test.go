package rel_test

import (
	. "github.com/glycerine/g3/pkg/muz/rel"
	"strings"
	"testing"
)

func TestAAGExporterRejectsNonlinearClause(t *testing.T) {
	f := newExporterFixture(t, "P", "Q", "R")
	f.add(t, "P", "Q", "R")
	must(t, f.set.SetOutputPredicate(f.decl["P"]))
	x, e := NewAAGExporter(f.manager, f.set, nil)
	must(t, e)
	defer x.Close()
	e = x.Write(&discardWriter{})
	if e == nil || !strings.Contains(e.Error(), "non-linear") {
		t.Fatal(e)
	}
}

type discardWriter struct{}

func (*discardWriter) Write(p []byte) (int, error) { return len(p), nil }
