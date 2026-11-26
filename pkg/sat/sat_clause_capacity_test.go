package sat

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSATClauseCapacityProbe(t *testing.T) {
	for _, tc := range []struct {
		name          string
		groups        [][4]int
		missingPair   bool
		contradiction bool
	}{
		{
			name:          "insufficient total capacity",
			groups:        [][4]int{{0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3}},
			contradiction: true,
		},
		{
			name:          "insufficient capacity in a subset",
			groups:        [][4]int{{0, 0, 1, 1}, {0, 1, 1, 0}, {1, 1, 0, 0}, {2, 3, 4, 4}, {2, 3, 4, 4}},
			contradiction: true,
		},
		{
			name:        "incomplete exclusion group",
			groups:      [][4]int{{0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3}},
			missingPair: true,
		},
		{
			name:   "matching exists",
			groups: [][4]int{{0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3}, {0, 1, 2, 3}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := satSolverCheckFixture(t)
			var groups [5][]Literal
			for i, row := range tc.groups {
				var clause [4]Literal
				for j, group := range row {
					v, err := s.MkVar(false, true)
					satOK(t, err)
					lit := MakeLiteral(v, (i+j)%2 != 0)
					clause[j] = lit
					groups[group] = append(groups[group], lit)
				}
				satOK(t, s.MkClause(clause[:]))
			}
			for g, group := range groups {
				for i, lit := range group {
					for j, other := range group[:i] {
						if tc.missingPair && g == 0 && i == 1 && j == 0 {
							continue
						}
						satOK(t, s.MkClause([]Literal{lit.Negated(), other.Negated()}))
					}
				}
			}
			s.Limit.Cancel()
			got, err := s.ProbeClauseCapacity()
			satOK(t, err)
			if got {
				t.Fatal("canceled probe returned a verdict")
			}
			s.Limit.ResetCancel()
			satOK(t, s.Limit.Push(1))
			got, err = s.ProbeClauseCapacity()
			s.Limit.Pop()
			satOK(t, err)
			if got {
				t.Fatal("resource-limited probe returned a verdict")
			}

			steps := s.Limit.Count()
			for i := 0; i < 2; i++ {
				got, err = s.ProbeClauseCapacity()
				satOK(t, err)
				if got != tc.contradiction || s.Limit.Count() != steps {
					t.Fatalf("probe %t, want %t, steps %d (before %d)", got, tc.contradiction, s.Limit.Count(), steps)
				}
			}
			verdict, err := s.Check(nil)
			satOK(t, err)
			if verdict != lbool.FromBool(!tc.contradiction) {
				t.Fatalf("ordinary verdict %v", verdict)
			}
			satFullSolverRuntimeAudit(t, s)
		})
	}
}

func TestSATClauseCapacityExhaustiveExclusions(t *testing.T) {
	// Three rows each choose one of two members. Exhaust the undirected
	// exclusion graphs and use the eight possible joint choices as an oracle.
	const members = 6
	var edges [members * (members - 1) / 2][2]int
	pos := 0
	for a := 0; a < members; a++ {
		for b := a + 1; b < members; b++ {
			edges[pos] = [2]int{a, b}
			pos++
		}
	}
	offsets := []int{0, 2, 4, 6}
	for mask := 0; mask < 1<<len(edges); mask++ {
		var links [members][capacityWords]uint64
		for i, pair := range edges {
			if mask&(1<<i) != 0 {
				a, b := pair[0], pair[1]
				links[a][0] |= 1 << b
				links[b][0] |= 1 << a
			}
		}
		if !clauseCapacityDeficient(links[:], offsets) {
			continue
		}
		for a := 0; a < 2; a++ {
			for b := 2; b < 4; b++ {
				for c := 4; c < 6; c++ {
					if links[a][0]&((1<<b)|(1<<c)) == 0 && links[b][0]&(1<<c) == 0 {
						t.Fatalf("mask %#x rejected valid choice (%d, %d, %d)", mask, a, b, c)
					}
				}
			}
		}
	}
}
