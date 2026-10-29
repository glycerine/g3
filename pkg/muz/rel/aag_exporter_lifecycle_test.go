package rel_test

import (
	"bytes"
	. "github.com/glycerine/g3/pkg/muz/rel"
	"testing"
)

func TestAAGExporterEmptyProgramAndLifecycle(t *testing.T) {
	f := newExporterFixture(t, "P")
	must(t, f.set.SetOutputPredicate(f.decl["P"]))
	x, e := NewAAGExporter(f.manager, f.set, nil)
	must(t, e)
	var out bytes.Buffer
	must(t, x.Write(&out))
	must(t, x.Close())
	must(t, x.Close())
	if e = x.Write(&out); e == nil {
		t.Fatal("closed exporter")
	}
}
