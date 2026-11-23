package smtcore

import (
	"slices"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// A finite model table can depend on far fewer variables than its quantifier
// binds. An ordered decision diagram represents the body as a function on the
// same domains as the scalar evaluator. Equal branches share a node. When all
// branches agree the decision disappears entirely.
type tableDiagram struct {
	model   *tableModel
	domains [][]int
	nodes   []tableDecision
	buckets map[tableDecisionKey][]int
	results map[tableOperation]int
	work    int
	stopped bool
}

// Negative IDs are terminal model values. Positive IDs index nodes. Zero
// means the operation could not finish within its limits.
type tableDecision struct {
	level    int // declaration order
	children []int
}

type tableDecisionKey struct {
	level, first, last int
}

type tableOperation struct {
	op      int32
	a, b, c int
}

const (
	tableDiagramWork  = 100000
	tableDiagramNodes = 4096
)

func (d *tableDiagram) step() bool {
	if !d.stopped {
		d.work++
		d.stopped = d.work > tableDiagramWork || d.work%256 == 0 && d.model.manager.ResourceLimit().CancelFlag()
	}
	return !d.stopped
}

func (d *tableDiagram) decision(level int, children []int) int {
	if !d.step() {
		return 0
	}
	first := children[0]
	if first == 0 {
		d.stopped = true
		return 0
	}
	different := false
	for _, child := range children[1:] {
		if child == 0 {
			d.stopped = true
			return 0
		}
		different = different || child != first
	}
	if !different {
		return first
	}
	key := tableDecisionKey{level, first, children[len(children)-1]}
	for _, id := range d.buckets[key] {
		if slices.Equal(children, d.nodes[id].children) {
			return id
		}
	}
	if len(d.nodes) >= tableDiagramNodes {
		d.stopped = true
		return 0
	}
	id := len(d.nodes)
	d.nodes = append(d.nodes, tableDecision{level, children})
	d.buckets[key] = append(d.buckets[key], id)
	return id
}

func (d *tableDiagram) resultSort(id int) ast.ASTHandle {
	// Well-sorted applications and ITEs give all leaves the same sort.
	for id > 0 {
		id = d.nodes[id].children[0]
	}
	if id == 0 {
		return ast.ASTHandle{}
	}
	return d.model.sorts[-id]
}

func (d *tableDiagram) branch(id, level, position int) int {
	if id > 0 && d.nodes[id].level == level {
		return d.nodes[id].children[position]
	}
	return id
}

func (d *tableDiagram) equal(a, b int) int { return d.apply(ast.ASTOpEq, a, b, -tableTrue) }
func (d *tableDiagram) ite(condition, yes, no int) int {
	return d.apply(ast.ASTOpIte, condition, yes, no)
}

// Pointwise equality and if-then-else suffice for every supported operator.
// Split the operands together on the earliest declaration read by any of them.
func (d *tableDiagram) apply(op int32, a, b, c int) int {
	if !d.step() || a == 0 || b == 0 || c == 0 {
		return 0
	}
	switch op {
	case ast.ASTOpEq:
		if a == b {
			return -tableTrue
		}
		if a < 0 && b < 0 {
			return -tableFalse
		}
		if a > b {
			a, b = b, a
		}
	case ast.ASTOpIte:
		if a == -tableTrue || b == c {
			return b
		}
		if a == -tableFalse {
			return c
		}
		if b == -tableTrue && c == -tableFalse {
			return a
		}
	}
	key := tableOperation{op, a, b, c}
	if result := d.results[key]; result != 0 {
		return result
	}
	level := len(d.domains)
	for _, id := range [...]int{a, b, c} {
		if id > 0 && d.nodes[id].level < level {
			level = d.nodes[id].level
		}
	}
	if level == len(d.domains) {
		d.stopped = true
		return 0
	}
	children := make([]int, len(d.domains[level]))
	for i := range children {
		children[i] = d.apply(op, d.branch(a, level, i), d.branch(b, level, i), d.branch(c, level, i))
		if children[i] == 0 {
			return 0
		}
	}
	result := d.decision(level, children)
	d.results[key] = result
	return result
}

// memo belongs to this binding environment. A function's else expression has
// its own environment, with variable zero bound to its first actual argument.
func (d *tableDiagram) evaluate(id int, bindings []int, memo map[int]int, depth int) int {
	if !d.step() || depth > 64 {
		d.stopped = true
		return 0
	}
	if result := memo[id]; result != 0 {
		return result
	}
	term := &d.model.terms[id]
	arg := func(i int) int { return d.evaluate(term.args[i], bindings, memo, depth+1) }
	var result int
	switch term.op {
	case tableConstant:
		result = -term.index
	case tableVariable:
		if term.index >= len(bindings) || d.resultSort(bindings[term.index]) != term.sort {
			d.stopped = true
			return 0
		}
		result = bindings[term.index]
	case ast.ASTOpNot:
		result = d.ite(arg(0), -tableFalse, -tableTrue)
	case ast.ASTOpImplies:
		result = d.ite(arg(0), arg(1), -tableTrue)
	case ast.ASTOpIte:
		result = d.ite(arg(0), arg(1), arg(2))
	case ast.ASTOpIff, ast.ASTOpEq:
		result = d.equal(arg(0), arg(1))
	case ast.ASTOpXor:
		result = d.ite(d.equal(arg(0), arg(1)), -tableFalse, -tableTrue)
	case ast.ASTOpAnd, ast.ASTOpOr:
		conjunction := term.op == ast.ASTOpAnd
		result = -tableBool(conjunction)
		for i := range term.args {
			child := arg(i)
			if conjunction {
				result = d.ite(result, child, -tableFalse)
			} else {
				result = d.ite(result, -tableTrue, child)
			}
			if result == -tableBool(!conjunction) {
				break
			}
		}
	case ast.ASTOpDistinct:
		result = -tableTrue
		var previous [16]int
		for i := range term.args {
			previous[i] = arg(i)
			for _, other := range previous[:i] {
				result = d.ite(d.equal(previous[i], other), -tableFalse, result)
			}
		}
	case tableApplication:
		var args [tableMaxArity]int
		for i := range term.args {
			args[i] = arg(i)
		}
		fn := &d.model.functions[term.index]
		result = d.evaluate(fn.fallback, args[:len(term.args)], make(map[int]int), depth+1)
		for i := len(fn.entries) - 1; i >= 0; i-- {
			row := &fn.entries[i]
			if result == -row.value {
				continue
			}
			match := -tableTrue
			for j := range term.args {
				match = d.ite(match, d.equal(args[j], -row.keys[j]), -tableFalse)
				if match == -tableFalse {
					break
				}
			}
			result = d.ite(match, -row.value, result)
		}
	default:
		d.stopped = true
		return 0
	}
	memo[id] = result
	return result
}

// Select the first concrete falsifying tuple in the scalar checker's order.
// An abstract value represents unnamed elements and cannot be instantiated.
func (d *tableDiagram) findWitness(id, level int, bindings []int, exhausted map[int]bool) bool {
	if !d.step() || id == -tableTrue || exhausted[id] {
		return false
	}
	if level == len(d.domains) {
		return id == -tableFalse
	}
	for i, value := range d.domains[level] {
		if d.model.values[value].IsNull() {
			continue
		}
		bindings[len(bindings)-1-level] = value
		child := d.branch(id, level, i)
		if d.findWitness(child, level+1, bindings, exhausted) {
			return true
		}
		if child == id {
			break // this declaration does not affect the remaining choices
		}
	}
	if id > 0 && d.nodes[id].level == level {
		exhausted[id] = true
	}
	return false
}

func (g *tableModel) checkDiagram(root int, domains [][]int) (lbool.LBool, []ast.ASTHandle) {
	d := tableDiagram{
		model: g, domains: domains, nodes: make([]tableDecision, 1),
		buckets: make(map[tableDecisionKey][]int), results: make(map[tableOperation]int),
	}
	bindings := make([]int, len(domains))
	for i, domain := range domains {
		children := make([]int, len(domain))
		for j, value := range domain {
			children[j] = -value
		}
		bindings[len(bindings)-1-i] = d.decision(i, children)
	}
	id := d.evaluate(root, bindings, make(map[int]int), 0)
	if d.stopped || id == 0 || d.resultSort(id) != g.sorts[tableTrue] || g.manager.ResourceLimit().CancelFlag() {
		return lbool.Undef, nil
	}
	if id == -tableTrue {
		return lbool.False, nil // the negated body has no witness
	}
	if d.findWitness(id, 0, bindings, make(map[int]bool)) {
		g.work = tableDiagramWork
		if g.evaluate(root, bindings, 0) == tableFalse && !g.incomplete && g.work > 0 && !g.manager.ResourceLimit().CancelFlag() {
			return lbool.True, g.witness(bindings)
		}
	}
	return lbool.Undef, nil
}
