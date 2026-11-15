package cmd_context

// Source: src/muz/fp/datalog_parser.cpp wpa_parser_impl::finish_map_files.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
				entry := p.ensureSortContent(names[i])
				entry.numbers[7], entry.numbers[19] = true, true
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

// AUDIT.md cmd_context_02-F11: finish_map_files registers BOOL, SHORT, then
// the map sorts in m_sort_contents (map<symbol, ...>) table order, not in
// alphabetical or file order.
func TestWPAFinishSortsFollowsSortContentsTableOrder(t *testing.T) {
	names := []string{"V", "W", "Apple", "Zed", "M3", "Q", "Kappa", "B", "Node", "Edge", "Heap", "Var"}
	directory := t.TempDir()
	for _, name := range names {
		contextMust(t, os.WriteFile(filepath.Join(directory, name+".map"), []byte("1 one\n"), 0o600))
	}
	c := contextFixture(t)
	p, err := newWPAParser(c, directory)
	contextMust(t, err)
	t.Cleanup(func() { contextMust(t, p.rules.close()) })
	contextMust(t, p.readMaps())
	contextMust(t, p.finishSorts())
	var order []string
	for _, e := range p.sortOrder.entries() {
		order = append(order, e.name)
	}
	sorted := append([]string(nil), order...)
	sort.Strings(sorted)
	if reflect.DeepEqual(order, sorted) {
		t.Fatal("test names do not distinguish table order from alphabetical order", order)
	}
	var previous uint32
	for i, name := range append([]string{"BOOL", "SHORT"}, order...) {
		id, err := p.manager.GetASTID(p.rules.sorts[name])
		contextMust(t, err)
		if i > 0 && id <= previous {
			t.Fatalf("sort %s (id %d) was not registered after its predecessor (id %d); table order %v", name, id, previous, order)
		}
		previous = id
	}
}
