package realclosure

import (
	"bytes"
	"strings"
	"testing"
)

func TestRankedAPICompactAndPrettyDisplay(t *testing.T) {
	f := newRankedValueFixture(t, true)
	var compact bytes.Buffer
	if err := f.base.store.displayCompactRankedValue(&compact, f.gamma, false); err != nil {
		t.Fatal(err)
	}
	text := compact.String()
	alphaName, gammaName := rankedRootName(f.alpha.root, false), rankedRootName(f.gamma.root, false)
	alphaAt, gammaAt := strings.Index(text, "; "+alphaName+" := "), strings.Index(text, "; "+gammaName+" := ")
	if alphaAt < 0 || gammaAt < 0 || alphaAt >= gammaAt || !strings.Contains(text, "root(") {
		t.Fatal("compact dependency definitions", text)
	}
	var pretty bytes.Buffer
	if err := f.base.store.displayCompactRankedValue(&pretty, f.gamma, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pretty.String(), "&alpha;<sub>") || !strings.Contains(pretty.String(), "<sup>2</sup>") {
		t.Fatal("pretty nested display", pretty.String())
	}
}
