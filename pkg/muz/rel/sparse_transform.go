package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (t *SparseTable) Project(removed []int) (_ *SparseTable, resultErr error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	// project_fn::operator()'s verbose_action _va("project").
	action, e := base.StartVerboseAction("project", base.VerboseActionDefaultLevel)
	if e != nil {
		return nil, e
	}
	defer func() { resultErr = errors.Join(resultErr, action.End()) }()
	if t.functional != 0 {
		return t.ProjectFunctional(removed)
	}
	sig, e := projectedSignature(t.signature, removed)
	if e != nil {
		return nil, e
	}
	out, e := NewSparseTable(sig, 0)
	if e != nil {
		return nil, e
	}
	remove := map[int]bool{}
	for _, i := range removed {
		remove[i] = true
	}
	facts, e := t.Facts()
	if e != nil {
		return nil, e
	}
	for _, f := range facts {
		p := make(Fact, 0, len(f)-len(removed))
		for i, v := range f {
			if !remove[i] {
				p = append(p, v)
			}
		}
		if _, e = out.Add(p); e != nil {
			return nil, e
		}
	}
	return out, nil
}
func (t *SparseTable) Rename(cycle []int) (_ *SparseTable, resultErr error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	// rename_fn::operator()'s verbose_action _va("rename").
	action, e := base.StartVerboseAction("rename", base.VerboseActionDefaultLevel)
	if e != nil {
		return nil, e
	}
	defer func() { resultErr = errors.Join(resultErr, action.End()) }()
	if t.functional != 0 {
		return t.RenameFunctional(cycle)
	}
	domains := t.signature.Domains()
	for _, i := range cycle {
		if i < 0 || i >= len(domains) {
			return nil, ErrTable
		}
	}
	if len(cycle) > 1 {
		tmp := domains[cycle[0]]
		for i := 1; i < len(cycle); i++ {
			domains[cycle[i-1]] = domains[cycle[i]]
		}
		domains[cycle[len(cycle)-1]] = tmp
	}
	sig, e := NewSignature(domains)
	if e != nil {
		return nil, e
	}
	out, e := NewSparseTable(sig, 0)
	if e != nil {
		return nil, e
	}
	facts, e := t.Facts()
	if e != nil {
		return nil, e
	}
	for _, f := range facts {
		g := append(Fact(nil), f...)
		if len(cycle) > 1 {
			tmp := g[cycle[0]]
			for i := 1; i < len(cycle); i++ {
				g[cycle[i-1]] = g[cycle[i]]
			}
			g[cycle[len(cycle)-1]] = tmp
		}
		if _, e = out.Add(g); e != nil {
			return nil, e
		}
	}
	return out, nil
}
