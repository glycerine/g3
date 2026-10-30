package qe

import "testing"

func TestBitVectorPluginBranchCountUsesFullWidth(t *testing.T) {
	for _, test := range []struct {
		width uint32
		want  string
	}{{1, "2"}, {8, "256"}, {65, "36893488147419103232"}} {
		t.Run(test.want, func(t *testing.T) {
			f := newBVPluginFixture(t, test.width)
			branches := f.rational("0")
			handled, err := f.plugin.GetNumBranches(f.contains, f.formula, branches)
			if err != nil || !handled {
				t.Fatalf("handled=%v err=%v", handled, err)
			}
			got, err := f.manager.NumeralService().String(branches)
			if err != nil || got != test.want {
				t.Fatalf("branches=%q err=%v", got, err)
			}
		})
	}
}
