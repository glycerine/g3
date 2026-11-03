package cmd_context

// Source: src/muz/fp/datalog_parser.cpp wpa_parser_impl::finish_map_files.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestWPAFinishSortsAllocationFailureReleasesEverySort(t *testing.T) {
	const mappedSorts = 512
	failed, completed := false, false
	for stop := int64(0); stop < 1024 && !completed; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			c := contextFixture(t)
			p, err := newWPAParser(c, "")
			contextMust(t, err)
			t.Cleanup(func() { contextMust(t, p.rules.close()) })
			m := p.manager
			_, err = m.RegisterDatalogPlugin()
			contextMust(t, err)
			// Distinct names force fresh sorts across several physical AST
			// and reference pages. Interning is outside this sort campaign.
			names := make([]string, mappedSorts)
			for i := range names {
				names[i] = fmt.Sprintf("allocation-sort-%03d", i)
				p.sorts[names[i]] = &wpaSort{
					name: names[i], numbers: map[uint64]bool{7: true, 19: true},
				}
			}
			for _, name := range append(names, "BOOL", "SHORT") {
				_, err = m.Symbols.Intern(name)
				contextMust(t, err)
			}
			before := m.Stats()
			m.Budget().FailAfter(stop)
			cause := p.finishSorts()
			m.Budget().FailAfter(-1)
			if cause != nil {
				failed = true
				if !errors.Is(cause, arena.ErrInjectedFailure) {
					t.Fatal("unexpected sort construction failure", cause)
				}
			} else {
				completed = true
				if len(p.rules.sorts) != mappedSorts+2 {
					t.Fatal("successful construction lost sorts", len(p.rules.sorts))
				}
				for name, handle := range p.rules.sorts {
					size, finite, err := m.DatalogFiniteSortSize(handle)
					contextMust(t, err)
					want := uint64(3) // Two distinct mapped numbers plus zero capacity.
					if name == "BOOL" {
						want = 2
					} else if name == "SHORT" {
						want = 65536
					}
					if !finite || size != want {
						t.Fatal("successful construction changed the finite domain", name, finite, size, want)
					}
				}
			}
			contextMust(t, p.rules.close())
			after := m.Stats()
			if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
				t.Fatalf("sort construction left owners after cleanup: nodes %d/%d refs %d/%d infos %d/%d parameters %d/%d; cause %v", after.LiveNodes, before.LiveNodes, after.LiveReferences, before.LiveReferences, after.LiveInfos, before.LiveInfos, after.LiveParameters, before.LiveParameters, cause)
			}
		})
		if t.Failed() {
			break
		}
	}
	if !failed || !completed {
		t.Fatal("sort allocation campaign missed failure or completion", failed, completed)
	}
}
