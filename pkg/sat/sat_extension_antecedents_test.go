package sat

import "testing"

func TestSATExtensionPublishesAntecedents(t *testing.T) {
	conflict := satConflictFixture(t, 2)
	analysis := conflict.analysis
	extension := &satExtensionFixture{antecedents: []Literal{MakeLiteral(0, false), MakeLiteral(1, true)}}
	analysis.propagation.extension = extension
	analysis.propagation.extensionPresent = true
	satOK(t, analysis.fillExtAntecedents(MakeLiteral(1, false), newSATExternalJustification(9)))
	if n, err := analysis.extAntecedents.Size(); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	for i, want := range extension.antecedents {
		got, err := analysis.extAntecedents.Read(uint32(i))
		satOK(t, err)
		if Literal(got) != want {
			t.Fatal(got, want)
		}
	}
}
