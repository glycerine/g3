package params

import udiag "github.com/glycerine/g3/pkg/util/diagnostic"

// NamedParamDescription is one entry in a declaration-generated parameter catalog.
type NamedParamDescription struct {
	Name                         string
	Kind                         ParamKind
	Description, Default, Module string
}

// ParamDescriptionGroup owns an ordered, immutable copy of its descriptions.
type ParamDescriptionGroup struct{ rows []NamedParamDescription }

func NewParamDescriptionGroup(entries ...NamedParamDescription) *ParamDescriptionGroup {
	return &ParamDescriptionGroup{rows: append([]NamedParamDescription(nil), entries...)}
}

// InsertGroup inserts missing descriptions in declaration order. Removing an
// entry invalidates the remembered groups so a later call can add it again.
func (d *ParamDescrs) InsertGroup(g *ParamDescriptionGroup) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := d.open(); e != nil {
		return e
	}
	if g == nil {
		return ErrParamDescrsPrecondition
	}
	if !udiag.DebugBuild {
		for _, known := range d.groups {
			if known == g {
				return nil
			}
		}
	}
	for i := range g.rows {
		r := &g.rows[i]
		k, e := d.manager.Intern(r.Name)
		if e != nil {
			return e
		}
		var module *string
		if r.Module != "" {
			module = &r.Module
		}
		if e = d.insert(k, r.Kind, &r.Description, &r.Default, module); e != nil {
			return e
		}
	}
	if !udiag.DebugBuild {
		copy(d.groups[1:], d.groups[:len(d.groups)-1])
		d.groups[0] = g
	}
	return nil
}
