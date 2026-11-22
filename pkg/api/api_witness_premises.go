package api

import "github.com/glycerine/g3/pkg/ast"

// The index borrows complete handle identities. It owns no reference in the
// source arena. Each use checks the ordered list of current premise roots.
type witnessPremises struct {
	base      []ast.ASTHandle
	rows      []witnessPremise
	frequency map[ast.ASTHandle]int
}

type witnessPremise struct {
	root           ast.ASTHandle
	symbols        []ast.ASTHandle
	ground         bool
	shortUniversal bool
	threshold      int
}

func (p *witnessPremises) matches(base []ast.ASTHandle) bool {
	if p == nil || len(base) != len(p.base) {
		return false
	}
	for i, h := range base {
		if h != p.base[i] {
			return false
		}
	}
	return true
}

// The same stamp covers a DAG walk within one positive conjunct. Count a
// declaration at most once in that conjunct, regardless of its occurrence count.
type witnessSymbolWalk struct {
	includeConstants bool
	seen             map[ast.ASTHandle]int
	declarations     map[ast.ASTHandle]int
	stack            []ast.ASTHandle
	visits, stamp    int
}

func (w *witnessSymbolWalk) collect(m *ast.ASTManager, root ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	if w.seen == nil {
		w.seen = make(map[ast.ASTHandle]int)
		w.declarations = make(map[ast.ASTHandle]int)
	}
	w.stamp++
	w.stack = append(w.stack[:0], root)
	var symbols []ast.ASTHandle
	for len(w.stack) > 0 {
		h := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
		if w.seen[h] == w.stamp {
			continue
		}
		w.seen[h] = w.stamp
		w.visits++
		if w.visits > 300_000 || w.visits%128 == 0 && m.ResourceLimit().CancelFlag() {
			return nil, false, nil
		}
		hd, err := m.NodeHeader(h)
		if err != nil {
			return nil, false, err
		}
		switch hd.Kind {
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return nil, false, err
			}
			w.stack = append(w.stack, body)
		case ast.ASTApp:
			if hd.NumArgs > 8192 || len(w.stack)+int(hd.NumArgs) > 16_384 {
				return nil, false, nil
			}
			if (hd.NumArgs > 0 || w.includeConstants) && hd.Family == ast.NullFamilyID && w.declarations[hd.Decl] != w.stamp {
				w.declarations[hd.Decl] = w.stamp
				symbols = append(symbols, hd.Decl)
			}
			for i := uint32(0); i < hd.NumArgs; i++ {
				child, err := m.GetArg(h, i)
				if err != nil {
					return nil, false, err
				}
				w.stack = append(w.stack, child)
			}
		}
	}
	return symbols, true, nil
}

func indexWitnessPremises(m *ast.ASTManager, base []ast.ASTHandle) (*witnessPremises, error) {
	index := &witnessPremises{frequency: make(map[ast.ASTHandle]int)}
	stack := make([]ast.ASTHandle, 0, len(base))
	for i := len(base) - 1; i >= 0; i-- {
		stack = append(stack, base[i])
	}
	visited := make(map[ast.ASTHandle]bool)
	var walk witnessSymbolWalk
	for len(stack) > 0 {
		root := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[root] {
			continue
		}
		visited[root] = true
		if len(visited) > 16_000 || m.ResourceLimit().CancelFlag() {
			return nil, nil
		}
		hd, err := m.NodeHeader(root)
		if err != nil {
			return nil, err
		}
		if hd.Kind == ast.ASTApp && hd.Family == m.BasicFamilyID() && hd.DeclKind == ast.ASTOpAnd {
			if hd.NumArgs > 8192 || len(stack)+int(hd.NumArgs) > 16_384 {
				return nil, nil
			}
			for i := hd.NumArgs; i > 0; i-- {
				child, err := m.GetArg(root, i-1)
				if err != nil {
					return nil, err
				}
				stack = append(stack, child)
			}
			continue
		}
		ground, err := m.IsGround(root)
		if err != nil {
			return nil, err
		}
		before := walk.visits
		symbols, complete, err := walk.collect(m, root)
		if !complete || err != nil {
			return nil, err
		}
		for _, h := range symbols {
			index.frequency[h]++
		}
		row := witnessPremise{root: root, symbols: symbols, ground: ground}
		// collect already counted the distinct terms in this conjunct, including
		// its binder and body. Trigger annotations do not add body terms.
		if hd.Kind == ast.ASTQuantifier && walk.visits-before <= 12 {
			row.shortUniversal, err = m.IsForall(root)
			if err != nil {
				return nil, err
			}
		}
		index.rows = append(index.rows, row)
	}
	for i, row := range index.rows {
		low := len(index.rows)
		for _, h := range row.symbols {
			low = min(low, index.frequency[h])
		}
		index.rows[i].threshold = 2 * low
	}
	index.base = append([]ast.ASTHandle(nil), base...)
	return index, nil
}

func (p *orderProofState) indexedPremises(m *ast.ASTManager, base []ast.ASTHandle) (*witnessPremises, error) {
	index := p.witnessPremises
	if !index.matches(base) {
		var err error
		index, err = indexWitnessPremises(m, base)
		p.witnessPremises = index
		if index == nil || err != nil {
			return nil, err
		}
	}
	return index, nil
}

// Start with the query's symbols. In each round add a premise when a shared
// symbol occurs in at most twice as many premises as its rarest symbol.
// Keep all ground conjuncts and preserve their order in the source.
func (p *orderProofState) selectWitnessPremises(m *ast.ASTManager, full *witnessInstancePlan) (*witnessInstancePlan, error) {
	fs := full.assertions
	base, goal := fs[:len(fs)-1], fs[len(fs)-1]
	index, err := p.indexedPremises(m, base)
	if err != nil || index == nil {
		return nil, err
	}
	var walk witnessSymbolWalk
	symbols, complete, err := walk.collect(m, goal)
	if !complete || err != nil || len(symbols) == 0 {
		return nil, err
	}
	active := make(map[ast.ASTHandle]bool, len(symbols))
	for _, h := range symbols {
		active[h] = true
	}
	keep := make([]bool, len(index.rows))
	for i, row := range index.rows {
		keep[i] = row.ground
	}
	for round := 0; round < 2; round++ {
		var added []int
		for i, row := range index.rows {
			if i%128 == 0 && m.ResourceLimit().CancelFlag() {
				return nil, nil
			}
			if keep[i] {
				continue
			}
			for _, h := range row.symbols {
				if active[h] && index.frequency[h] <= row.threshold {
					keep[i] = true
					added = append(added, i)
					break
				}
			}
		}
		if len(added) == 0 {
			break
		}
		for _, i := range added {
			for _, h := range index.rows[i].symbols {
				active[h] = true
			}
		}
	}
	var selected []ast.ASTHandle
	for i, row := range index.rows {
		if keep[i] {
			selected = append(selected, row.root)
		}
	}
	if len(selected) == 0 {
		return nil, nil
	}
	trial := *full
	trial.assertions = append(selected, goal)
	trial.source = fs
	trial.rounds = 1
	return &trial, nil
}
