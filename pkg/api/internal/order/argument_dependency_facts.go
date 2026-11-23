package order

import (
	"math/bits"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

type argumentFactKey struct {
	h     ast.ASTHandle
	truth bool
}
type argumentFactScan struct {
	d             *predicateDependencies
	vars          int
	usePredicates bool
	facts         []argumentDependency
	memo          map[argumentFactKey]uint64
}

func (d *predicateDependencies) facts(h ast.ASTHandle, truth bool, n int, usePredicates bool) []argumentDependency {
	s := argumentFactScan{d: d, vars: n, usePredicates: usePredicates, memo: make(map[argumentFactKey]uint64)}
	mask := s.implied(h, truth, 0)
	var out []argumentDependency
	for mask != 0 {
		j := bits.TrailingZeros64(mask)
		out = append(out, s.facts[j])
		mask &= mask - 1
	}
	return out
}

func (s *argumentFactScan) bit(p argumentDependency) uint64 {
	for i, v := range s.facts {
		if v == p {
			return 1 << i
		}
	}
	i := len(s.facts)
	if i == 64 {
		return 0
	}
	s.facts = append(s.facts, p)
	return 1 << i
}

func (s *argumentFactScan) equality(x, y ast.ASTHandle) uint64 {
	d := s.d
	v := d.variable(x, s.vars)
	if v < 0 {
		return 0
	}
	hd := d.header(y)
	if hd.Kind != ast.ASTApp || hd.Family != ast.NullFamilyID || hd.NumArgs != 1 {
		return 0
	}
	args := d.args(y, 1)
	if len(args) != 1 {
		return 0
	}
	u := d.variable(args[0], s.vars)
	if u < 0 || u == v {
		return 0
	}
	return s.bit(argumentDependency{v, u, hd.Decl})
}

// A conjunction establishes all equalities established by its conjuncts.
// Across alternatives an equality holds only if every alternative proves it.
// A bit denotes one exact equality between two variables and a declaration.
func (s *argumentFactScan) implied(h ast.ASTHandle, yes bool, depth int) uint64 {
	d := s.d
	if depth > 72 || len(s.memo) >= 4096 || !d.visit() {
		return 0
	}
	key := argumentFactKey{h, yes}
	if mask, ok := s.memo[key]; ok {
		return mask
	}
	hd := d.header(h)
	if hd.Kind != ast.ASTApp || hd.NumArgs == 0 || hd.NumArgs > 128 {
		return 0
	}
	args := d.args(h, hd.NumArgs)
	if len(args) == 0 {
		return 0
	}
	var mask uint64
	recur := func(i int, t bool) uint64 { return s.implied(args[i], t, depth+1) }
	if hd.Family == ast.NullFamilyID && yes && s.usePredicates {
		for _, p := range d.rules[hd.Decl] {
			v, u := d.variable(args[p.target], s.vars), d.variable(args[p.source], s.vars)
			if v >= 0 && u >= 0 && v != u {
				mask |= s.bit(argumentDependency{v, u, p.function})
			}
		}
	}
	if hd.Family == d.m.BasicFamilyID() {
		switch hd.DeclKind {
		case ast.ASTOpNot:
			if len(args) == 1 {
				mask = recur(0, !yes)
			}
		case ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpImplies:
			all := yes == (hd.DeclKind == ast.ASTOpAnd)
			mask = recur(0, yes != (hd.DeclKind == ast.ASTOpImplies))
			for i := 1; i < len(args); i++ {
				if all {
					mask |= recur(i, yes)
				} else if mask != 0 {
					mask &= recur(i, yes)
				} else {
					break
				}
			}
		case ast.ASTOpIte:
			if len(args) == 3 {
				left := recur(0, true) | recur(1, yes)
				mask = left & (recur(0, false) | recur(2, yes))
			}
		case ast.ASTOpIff, ast.ASTOpEq:
			if len(args) != 2 {
				break
			}
			if yes {
				mask = s.equality(args[0], args[1]) | s.equality(args[1], args[0])
			}
			if d.err != nil {
				return 0
			}
			sort, err := d.m.GetSort(args[0])
			if err != nil {
				d.err = err
				return 0
			}
			if sort == d.boolean {
				first := recur(0, true) | recur(1, yes)
				mask |= first & (recur(0, false) | recur(1, !yes))
			}
		}
	}
	s.memo[key] = mask
	return mask
}

// A counterexample to a universal, or a witness for an existential, must
// satisfy each inferred equality. Substituting a determined variable by its
// defining term preserves that quantifier whenever the dependencies hold.
func (w *enumerationWriter) dependencyBindings(body ast.ASTHandle, n uint32, universal bool) map[int]string {
	d := w.dependencies
	if d == nil || n > 16 {
		return nil
	}
	rules := d.facts(body, !universal, int(n), true)
	base := len(w.scope)
	var needed uint32
	var chosen map[int]string
	for _, p := range rules {
		target := base + int(n) - 1 - p.target
		source := base + int(n) - 1 - p.source
		if needed&(1<<p.target) != 0 || chosen[target] != "" || chosen[source] != "" {
			continue
		}
		f, ok := w.language.functions[p.function]
		if !ok {
			continue
		}
		if chosen == nil {
			chosen = make(map[int]string)
		}
		chosen[target] = "(" + f.name + " x" + strconv.Itoa(source) + ")"
		needed |= 1 << p.source
	}
	return chosen
}
