package fp

import (
	"fmt"
	"testing"
)

func TestBackgroundAssertionEnumerationReleasesFailedPrefix(t *testing.T) {
	failures, successes := 0, 0
	for allowance := int64(0); allowance < 24; allowance++ {
		t.Run(string(rune('A'+allowance)), func(t *testing.T) {
			f := newPDRLifecycleFixture(t)
			e, m := f.engine, f.manager
			for i := 0; i < 600; i++ {
				fragmentMust(t, e.AssertBackground(f.applications["p"]))
			}
			baseline := m.Stats().LiveReferences
			m.Budget().FailAfter(allowance)
			refs, err := e.BackgroundAssertions()
			m.Budget().FailAfter(-1)
			if err != nil {
				failures++
				if refs != nil {
					t.Fatal("failed enumeration published prefix")
				}
			} else {
				successes++
				if len(refs) != 600 {
					t.Fatal("assertion enumeration count", len(refs))
				}
				for i := range refs {
					fragmentMust(t, m.DelRef(&refs[i]))
				}
			}
			if got := m.Stats().LiveReferences; got != baseline {
				t.Fatal("enumeration leaked owning prefix", baseline, got, err)
			}
			if err != nil {
				refs, cause := e.BackgroundAssertions()
				fragmentMust(t, cause)
				for i := range refs {
					h, cause := m.RefGet(refs[i])
					fragmentMust(t, cause)
					if h != f.applications["p"] {
						t.Fatal("retry changed assertion")
					}
					fragmentMust(t, m.DelRef(&refs[i]))
				}
			}
		})
		if successes > 0 {
			break
		}
	}
	if failures == 0 || successes == 0 {
		t.Fatal("allocation scan missed refusal or success", failures, successes)
	}
}

func TestNamedRuleEnumerationReleasesFailedPrefix(t *testing.T) {
	failures, successes := 0, 0
	for allowance := int64(0); allowance < 32; allowance++ {
		t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			f := newDirectFactFixture(t)
			e, m := f.engine, f.manager
			for i := 0; i < 200; i++ {
				fragmentMust(t, e.AddRule(f.applications["p"], fmt.Sprintf("enumeration-%d", i)))
			}
			baseline := m.Stats().LiveReferences
			m.Budget().FailAfter(allowance)
			refs, names, err := e.NamedRules()
			m.Budget().FailAfter(-1)
			if err != nil {
				failures++
				if refs != nil || names != nil {
					t.Fatal("failed rule enumeration published prefix")
				}
			} else {
				successes++
				if len(refs) != 200 || len(names) != 200 {
					t.Fatal("rule enumeration count", len(refs), len(names))
				}
				for i := range refs {
					if names[i] != fmt.Sprintf("enumeration-%d", i) {
						t.Fatal("rule name order", i, names[i])
					}
					fragmentMust(t, m.DelRef(&refs[i]))
				}
			}
			if got := m.Stats().LiveReferences; got != baseline {
				t.Fatal("rule enumeration leaked owning prefix", baseline, got, err)
			}
			if err != nil {
				refs, _, cause := e.NamedRules()
				fragmentMust(t, cause)
				for i := range refs {
					fragmentMust(t, m.DelRef(&refs[i]))
				}
			}
		})
		if successes > 0 {
			break
		}
	}
	if failures == 0 || successes == 0 {
		t.Fatal("allocation scan missed refusal or success", failures, successes)
	}
}
