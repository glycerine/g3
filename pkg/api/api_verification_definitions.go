package api

import "github.com/glycerine/g3/pkg/ast"

// Each row is one positive conjunct of the original assertions. The indexes
// borrow handles while the exact ordered base matches witnessPremises. They
// guide selection only. The private solver checks the original selected terms.
type premiseDefinitions struct {
	base      *witnessPremises
	heads     []ast.ASTHandle
	symbols   [][]ast.ASTHandle
	frequency map[ast.ASTHandle]int
}

func (p *orderProofState) indexDefinitions(m *ast.ASTManager, base *witnessPremises) (*premiseDefinitions, error) {
	if d := p.definitions; d != nil && d.base == base {
		return d, nil
	}
	d := &premiseDefinitions{
		base: base, heads: make([]ast.ASTHandle, len(base.rows)), symbols: make([][]ast.ASTHandle, len(base.rows)),
		frequency: make(map[ast.ASTHandle]int, len(base.frequency)),
	}
	walk := witnessSymbolWalk{includeConstants: true}
	for i, r := range base.rows {
		symbols, ok, err := walk.collect(m, r.root)
		if err != nil || !ok {
			return nil, err
		}
		d.symbols[i] = symbols
		for _, h := range symbols {
			d.frequency[h]++
		}
		d.heads[i], ok, err = definitionHead(m, r.root, &walk)
		if err != nil || !ok {
			return nil, err
		}
	}
	p.definitions = d
	return d, nil
}

// Recognize a definition of a constant or of f(x1,...,xn) where the
// arguments are precisely the universally bound variables, in any order.
func definedApplication(m *ast.ASTManager, term ast.ASTHandle, bound uint32) (ast.ASTHandle, error) {
	hd, err := m.NodeHeader(term)
	if err != nil || hd.Kind != ast.ASTApp || hd.Family != ast.NullFamilyID || hd.NumArgs != bound {
		return ast.ASTHandle{}, err
	}
	var seen uint32
	for i := uint32(0); i < bound; i++ {
		arg, err := m.GetArg(term, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		v, err := m.NodeHeader(arg)
		if err != nil || v.Kind != ast.ASTVar {
			return ast.ASTHandle{}, err
		}
		index, err := m.GetVarIndex(arg)
		if err != nil || index >= bound || seen&(1<<index) != 0 {
			return ast.ASTHandle{}, err
		}
		seen |= 1 << index
	}
	return hd.Decl, nil
}

func definitionHead(m *ast.ASTManager, term ast.ASTHandle, walk *witnessSymbolWalk) (ast.ASTHandle, bool, error) {
	var bound uint32
	hd, err := m.NodeHeader(term)
	for err == nil && hd.Kind == ast.ASTQuantifier {
		all, err := m.IsForall(term)
		if err != nil || !all {
			return ast.ASTHandle{}, true, err
		}
		n, err := m.GetNumDeclarations(term)
		if err != nil || n > 32-bound {
			return ast.ASTHandle{}, true, err
		}
		bound += n
		term, err = m.GetQuantifierBody(term)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		hd, err = m.NodeHeader(term)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
	}
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if hd.Kind == ast.ASTApp && hd.Family == m.BasicFamilyID() {
		switch {
		case hd.DeclKind == ast.ASTOpNot && hd.NumArgs == 1:
			term, err = m.GetArg(term, 0)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
		case hd.DeclKind == ast.ASTOpEq && hd.NumArgs == 2:
			for i := uint32(0); i < 2; i++ {
				lhs, err := m.GetArg(term, i)
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				head, err := definedApplication(m, lhs, bound)
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				if head.IsNull() {
					continue
				}
				rhs, err := m.GetArg(term, 1-i)
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
				symbols, ok, err := walk.collect(m, rhs)
				if err != nil || !ok {
					return ast.ASTHandle{}, false, err
				}
				if !containsDeclaration(symbols, head) {
					return head, true, nil
				}
			}
			return ast.ASTHandle{}, true, nil
		}
	}
	head, err := definedApplication(m, term, bound)
	return head, true, err
}

func containsDeclaration(symbols []ast.ASTHandle, h ast.ASTHandle) bool {
	for _, s := range symbols {
		if s == h {
			return true
		}
	}
	return false
}

// sharedRarest accepts a constraint when one of its least frequent symbols
// occurs in the current goal or an already selected definition.
func (d *premiseDefinitions) sharedRarest(row []ast.ASTHandle, active map[ast.ASTHandle]bool, goal []ast.ASTHandle) bool {
	minimum := len(d.base.rows) + 2
	shared := false
	for _, h := range row {
		frequency := d.frequency[h]
		if containsDeclaration(goal, h) {
			frequency++
		}
		if frequency < minimum {
			minimum, shared = frequency, active[h]
		} else if frequency == minimum && active[h] {
			shared = true
		}
	}
	return shared
}

// Follow definitions backwards from the goal, one complete layer at a time.
// The wider pass also admits related constraints and positive ground facts.
func (d *premiseDefinitions) selectRows(m *ast.ASTManager, goal []ast.ASTHandle, wider bool, maximum int) []ast.ASTHandle {
	if maximum <= 0 {
		return nil
	}
	depth := 4
	if wider {
		depth = 2
	}
	active := make(map[ast.ASTHandle]bool, len(goal))
	for _, h := range goal {
		active[h] = true
	}
	selected := make([]bool, len(d.base.rows))
	var added []int
	total := 0
	for level := 0; level < depth; level++ {
		added = added[:0]
		for i, head := range d.heads {
			if selected[i] {
				continue
			}
			if i%128 == 0 && m.ResourceLimit().CancelFlag() {
				return nil
			}
			if !active[head] && !(wider && head.IsNull() && d.sharedRarest(d.symbols[i], active, goal)) {
				continue
			}
			selected[i] = true
			added = append(added, i)
			total++
			if total > maximum {
				return nil
			}
		}
		if len(added) == 0 {
			break
		}
		for _, i := range added {
			for _, h := range d.symbols[i] {
				active[h] = true
			}
		}
	}
	if total == 0 {
		return nil
	}
	result := make([]ast.ASTHandle, 0, total+1)
	for i, row := range d.base.rows {
		if selected[i] || wider && row.ground {
			result = append(result, row.root)
		}
	}
	if len(result) > maximum || len(result)*2 >= len(d.base.rows) {
		return nil
	}
	return result
}
