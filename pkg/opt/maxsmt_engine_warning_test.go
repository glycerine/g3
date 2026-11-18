package opt

import (
	"bytes"
	"testing"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// AUDIT.md opt_01-F7 / opt_02-F6: maxsmt::operator() warns before falling
// back to maxres for an unrecognized opt.maxsat_engine.
func TestSourceMaxSMTEngineWarnsForUnknownName(t *testing.T) {
	warnings := diagnostic.BorrowProcessWarningService()
	previous := warnings.WarningStream()
	var out bytes.Buffer
	if err := warnings.SetWarningStream(&out); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = warnings.SetWarningStream(previous) }()
	for _, name := range []string{"", "maxres", "pd-maxres", "wmax", "sortmax"} {
		if got, err := sourceMaxSMTEngine(name); err != nil || got != name {
			t.Fatal(name, got, err)
		}
	}
	if out.Len() != 0 {
		t.Fatalf("known engines warned: %q", out.String())
	}
	if got, err := sourceMaxSMTEngine("wmaxres"); err != nil || got != "maxres" {
		t.Fatal(got, err)
	}
	if want := "WARNING: solver wmaxres is not recognized, using default 'maxres'\n"; out.String() != want {
		t.Fatalf("warning %q, want %q", out.String(), want)
	}
}
