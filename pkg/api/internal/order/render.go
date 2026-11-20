package order

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// All identifiers are generated from integers in disjoint namespaces. No
// source symbol names or source SMT-LIB text enter the generated script.
func (t *inference) sortName(d domainID) string {
	if d == boolean {
		return "Bool"
	}
	return t.sortNames[d]
}

func constantName(d, v int) string { return "n" + strconv.Itoa(d) + "_" + strconv.Itoa(v) }

type orderSort struct {
	ordered  bool
	numerals map[int]bool
}

func (s *orderSort) numeral(value int) {
	if s.numerals == nil {
		s.numerals = make(map[int]bool)
	}
	s.numerals[value] = true
}

// Problem is a closed SMT-LIB script with one complete assertion per line.
// Premises end just before the last assertion, so a batch can keep a proven
// query's exact prefix while independently checking each new suffix.
type Problem struct {
	Text                                string
	AssertionsStart, LastAssertionStart int
	DeclarationCount                    int
}

func (p Problem) Declarations() string { return p.Text[:p.AssertionsStart] }
func (p Problem) Assertions() string   { return p.Text[p.AssertionsStart:] }
func (p Problem) Premises() string     { return p.Text[p.AssertionsStart:p.LastAssertionStart] }

func (t *inference) render() Problem {
	var out, axioms strings.Builder
	out.Grow(8 * len(t.terms))
	var p Problem
	// unify always links the larger root to the smaller one. Every root
	// spelling is available when its descendants are visited in index order.
	t.sortNames = make([]string, len(t.parents))
	for i := range t.parents {
		root := t.root(domainID(i))
		if root == domainID(i) {
			t.sortNames[i] = "s" + strconv.Itoa(i)
		} else {
			t.sortNames[i] = t.sortNames[root]
		}
	}
	sorts := make([]orderSort, len(t.parents))
	for _, n := range t.terms {
		switch n.op {
		case opOrder:
			d := t.root(t.terms[t.arguments(&n)[0]].domain)
			sorts[d].ordered = true
			sorts[d].numeral(0)
		case opNumeral:
			sorts[t.root(n.domain)].numeral(int(n.value))
		}
	}
	for i := range sorts {
		if t.root(domainID(i)) == domainID(i) {
			p.DeclarationCount++
			out.WriteString("(declare-sort ")
			out.WriteString(t.sortName(domainID(i)))
			out.WriteString(" 0)\n")
		}
	}
	for i, d := range t.signatures {
		p.DeclarationCount++
		out.WriteString("(declare-fun f")
		out.WriteString(strconv.Itoa(i))
		out.WriteString(" (")
		for _, a := range d.inputs {
			out.WriteString(t.sortName(a))
			out.WriteByte(' ')
		}
		out.WriteString(") ")
		out.WriteString(t.sortName(d.result))
		out.WriteString(")\n")
	}
	for i, s := range sorts {
		if !s.ordered {
			continue
		}
		d, r := t.sortName(domainID(i)), "lt"+strconv.Itoa(i)
		p.DeclarationCount++
		fmt.Fprintf(&out, "(declare-fun %s (%s %s) Bool)\n", r, d, d)
		fmt.Fprintf(&axioms, "(assert (forall ((x %s)) (not (%s x x))))\n", d, r)
		fmt.Fprintf(&axioms, "(assert (forall ((x %s)(y %s)) (or (= x y) (%s x y) (%s y x))))\n", d, d, r, r)
		fmt.Fprintf(&axioms, "(assert (forall ((x %s)(y %s)(z %s)) (=> (and (%s x y) (%s y z)) (%s x z))))\n", d, d, d, r, r, r)
		fmt.Fprintf(&axioms, "(assert (forall ((x %s)) (not (%s x %s))))\n", d, r, constantName(i, 0))
	}
	for i, s := range sorts {
		if len(s.numerals) == 0 {
			continue
		}
		values := make([]int, 0, len(s.numerals))
		for v := range s.numerals {
			values = append(values, v)
		}
		sort.Ints(values)
		for j, v := range values {
			p.DeclarationCount++
			fmt.Fprintf(&out, "(declare-const %s %s)\n", constantName(i, v), t.sortName(domainID(i)))
			if j > 0 && s.ordered {
				fmt.Fprintf(&axioms, "(assert (lt%d %s %s))\n", i, constantName(i, values[j-1]), constantName(i, v))
			}
		}
		if len(values) > 1 {
			axioms.WriteString("(assert (distinct")
			for _, v := range values {
				axioms.WriteByte(' ')
				axioms.WriteString(constantName(i, v))
			}
			axioms.WriteString("))\n")
		}
	}
	p.AssertionsStart = out.Len()
	out.WriteString(axioms.String())
	for _, n := range t.assertions {
		p.LastAssertionStart = out.Len()
		out.WriteString("(assert ")
		t.writeTerm(&out, n)
		out.WriteString(")\n")
	}
	p.Text = out.String()
	return p
}

func (t *inference) writeTerm(b *strings.Builder, id int32) {
	n := &t.terms[id]
	switch n.op {
	case opVariable:
		b.WriteByte('v')
		b.WriteString(strconv.Itoa(int(n.value)))
		return
	case opNumeral:
		b.WriteString(constantName(int(t.root(n.domain)), int(n.value)))
		return
	case opForall, opExists:
		b.WriteByte('(')
		b.WriteString(operatorNames[n.op])
		b.WriteString(" (")
		for _, v := range t.declarations[n.start : n.start+n.count] {
			b.WriteString("(v")
			b.WriteString(strconv.Itoa(int(v.name)))
			b.WriteByte(' ')
			b.WriteString(t.sortName(v.domain))
			b.WriteByte(')')
		}
		b.WriteString(") ")
		t.writeTerm(b, n.value)
		b.WriteByte(')')
		return
	case opOrder:
		args := t.arguments(n)
		left, right := args[0], args[1]
		d := t.root(t.terms[left].domain)
		if n.value == bvUGT || n.value == bvULE {
			left, right = right, left
		}
		negate := n.value == bvULE || n.value == bvUGE
		if negate {
			b.WriteString("(not ")
		}
		b.WriteString("(lt")
		b.WriteString(strconv.Itoa(int(d)))
		b.WriteByte(' ')
		t.writeTerm(b, left)
		b.WriteByte(' ')
		t.writeTerm(b, right)
		b.WriteByte(')')
		if negate {
			b.WriteByte(')')
		}
		return
	}
	args := t.arguments(n)
	if len(args) > 0 {
		b.WriteByte('(')
	}
	if n.op == opFunction {
		b.WriteByte('f')
		b.WriteString(strconv.Itoa(int(n.value)))
	} else {
		b.WriteString(operatorNames[n.op])
	}
	for _, a := range args {
		b.WriteByte(' ')
		t.writeTerm(b, a)
	}
	if len(args) > 0 {
		b.WriteByte(')')
	}
}
