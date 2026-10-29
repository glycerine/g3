package realclosure

import (
	"bytes"
	"testing"
)

func TestRankedAPIDisplayExpression(t *testing.T) {
	f := newRankedValueFixture(t, true)
	sum, err := f.base.store.addRankedRetainedValues(f.gamma, f.alpha, true)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := f.base.store.displayRankedExpression(&out, sum, true, false); err != nil {
		t.Fatal(err)
	}
	want := rankedRootName(f.gamma.root, false) + " + " + rankedRootName(f.alpha.root, false)
	if out.String() != want {
		t.Fatalf("expression %q want %q", out.String(), want)
	}
	f.releaseValues(sum)
}
