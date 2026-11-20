package order

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// All identifiers are generated from integers in disjoint namespaces. No
// source symbol names or source SMT-LIB text enter the generated script.
func (t *inference) sortName(d int) string {
	if d == boolean {
		return "Bool"
	}
	return "s" + strconv.Itoa(t.root(d))
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
	var p Problem
	sorts := make([]orderSort, len(t.parents))
	for _, n := range t.terms {
		switch n.op {
		case "order":
			d := t.root(t.terms[n.args[0]].domain)
			sorts[d].ordered = true
			sorts[d].numeral(0)
		case "num":
			sorts[t.root(n.domain)].numeral(n.value)
		}
	}
	for i := range sorts {
		if t.root(i) == i {
			p.DeclarationCount++
			fmt.Fprintf(&out, "(declare-sort s%d 0)\n", i)
		}
	}
	for i, d := range t.signatures {
		p.DeclarationCount++
		fmt.Fprintf(&out, "(declare-fun f%d (", i)
		for _, a := range d.inputs {
			out.WriteString(t.sortName(a))
			out.WriteByte(' ')
		}
		fmt.Fprintf(&out, ") %s)\n", t.sortName(d.result))
	}
	for i, s := range sorts {
		if !s.ordered {
			continue
		}
		d, r := t.sortName(i), "lt"+strconv.Itoa(i)
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
			fmt.Fprintf(&out, "(declare-const %s %s)\n", constantName(i, v), t.sortName(i))
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

func (t *inference) writeTerm(b *strings.Builder, id int) {
	n := &t.terms[id]
	switch n.op {
	case "var":
		b.WriteByte('v')
		b.WriteString(strconv.Itoa(n.value))
		return
	case "num":
		b.WriteString(constantName(t.root(n.domain), n.value))
		return
	case "forall", "exists":
		b.WriteByte('(')
		b.WriteString(n.op)
		b.WriteString(" (")
		for _, v := range n.binders {
			fmt.Fprintf(b, "(v%d %s)", v.name, t.sortName(v.domain))
		}
		b.WriteString(") ")
		t.writeTerm(b, n.args[0])
		b.WriteByte(')')
		return
	case "order":
		left, right := n.args[0], n.args[1]
		d := t.root(t.terms[left].domain)
		if n.value == bvUGT || n.value == bvULE {
			left, right = right, left
		}
		negate := n.value == bvULE || n.value == bvUGE
		if negate {
			b.WriteString("(not ")
		}
		fmt.Fprintf(b, "(lt%d ", d)
		t.writeTerm(b, left)
		b.WriteByte(' ')
		t.writeTerm(b, right)
		b.WriteByte(')')
		if negate {
			b.WriteByte(')')
		}
		return
	}
	if len(n.args) > 0 {
		b.WriteByte('(')
	}
	if n.op == "fun" {
		b.WriteByte('f')
		b.WriteString(strconv.Itoa(n.value))
	} else {
		b.WriteString(n.op)
	}
	for _, a := range n.args {
		b.WriteByte(' ')
		t.writeTerm(b, a)
	}
	if len(n.args) > 0 {
		b.WriteByte(')')
	}
}
