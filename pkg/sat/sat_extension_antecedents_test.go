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

// AUDIT.md sat_02-F4: resolve_conflict_core and get_max_lvl loop over the
// external antecedents fill_ext_antecedents publishes.
func TestSATExtensionAntecedentsReachConflictResolution(t *testing.T) {
	conflict := satConflictFixture(t, 4)
	p := conflict.analysis.propagation
	extension := &satExtensionFixture{antecedents: []Literal{MakeLiteral(0, false), MakeLiteral(1, false)}}
	p.extension = extension
	p.extensionPresent = true
	satConflictPush(t, conflict)
	satOK(t, p.assign(MakeLiteral(0, false), satJustification{}))
	satConflictPush(t, conflict)
	satOK(t, p.assign(MakeLiteral(1, false), satJustification{}))
	conflict.conflictLevel = 2
	marks := uint32(0)
	satOK(t, conflict.processConsequent(MakeLiteral(2, false), newSATExternalJustification(9), 0, &marks))
	marked, err := conflict.analysis.marked(1)
	satOK(t, err)
	if marks != 1 || !marked {
		t.Fatal("same-level external antecedent not marked", marks, marked)
	}
	lemma := satConflictLemma(t, conflict)
	found := false
	for _, l := range lemma {
		found = found || l == MakeLiteral(0, true)
	}
	if !found {
		t.Fatal("lower-level external antecedent missing from the lemma", lemma)
	}
	level, err := conflict.getMaxLevel(DefaultLiteral(), newSATExternalJustification(9))
	satOK(t, err)
	if level != 2 {
		t.Fatal("get_max_lvl ignored the external antecedents", level)
	}
}
