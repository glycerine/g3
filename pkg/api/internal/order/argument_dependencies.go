package order

import (
	"errors"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

// A true tuple of a predicate can force argument target to be function(source).
// Discovery uses only universally quantified premises held by the current solver.
type argumentDependency struct {
	target, source int
	function       ast.ASTHandle
}

type guardedPredicate struct {
	head, selector ast.ASTHandle
	positive       bool
}

type predicateDependencies struct {
	m         *ast.ASTManager
	boolean   ast.ASTHandle
	rules     map[ast.ASTHandle][]argumentDependency
	order     []ast.ASTHandle
	selectors map[guardedPredicate][]argumentDependency
	children  map[ast.ASTHandle][]ast.ASTHandle
	work      int
	stopped   bool
	err       error
}

func discoverPredicateDependencies(l *enumerationLanguage, base []ast.ASTHandle) (*predicateDependencies, error) {
	d := &predicateDependencies{
		m: l.manager, boolean: l.boolean, rules: make(map[ast.ASTHandle][]argumentDependency),
		selectors: make(map[guardedPredicate][]argumentDependency), children: make(map[ast.ASTHandle][]ast.ASTHandle),
	}
	for _, h := range base {
		d.premise(h, 0, 0)
	}
	if d.err != nil || d.stopped || len(d.order) == 0 {
		return nil, d.err
	}
	d.selectors = nil
	return d, nil
}

func (d *predicateDependencies) visit() bool {
	if d.err != nil || d.stopped {
		return false
	}
	d.work++
	if d.work > 250_000 || d.work%128 == 0 && d.m.Limit.CancelFlag() {
		d.stopped = true
		return false
	}
	return true
}

func (d *predicateDependencies) header(h ast.ASTHandle) ast.ASTNodeHeader {
	if d.err != nil {
		return ast.ASTNodeHeader{}
	}
	var hd ast.ASTNodeHeader
	hd, d.err = d.m.NodeHeader(h)
	return hd
}

// The source solver owns the nodes until this rendering pass completes.
func (d *predicateDependencies) args(h ast.ASTHandle, n uint32) []ast.ASTHandle {
	if v, ok := d.children[h]; ok {
		return v
	}
	if d.err != nil || n > 128 {
		return nil
	}
	v := make([]ast.ASTHandle, n)
	for i := range v {
		v[i], d.err = d.m.GetArg(h, uint32(i))
		if d.err != nil {
			return nil
		}
	}
	d.children[h] = v
	return v
}

func (d *predicateDependencies) variable(h ast.ASTHandle, count int) int {
	if d.header(h).Kind != ast.ASTVar {
		return -1
	}
	v, err := d.m.GetVarIndex(h)
	if err != nil {
		d.err = err
		return -1
	}
	if v >= uint32(count) {
		return -1
	}
	return int(v)
}

func (d *predicateDependencies) premise(h ast.ASTHandle, n, depth int) {
	if depth > 80 || !d.visit() {
		return
	}
	hd := d.header(h)
	if hd.Kind == ast.ASTQuantifier {
		yes, err := d.m.IsForall(h)
		if err != nil {
			d.err = err
			return
		}
		if !yes {
			return
		}
		k, err := d.m.GetNumDeclarations(h)
		if err != nil {
			d.err = err
			return
		}
		if k > 16 || n+int(k) > 16 {
			return
		}
		body, err := d.m.GetQuantifierBody(h)
		if err != nil {
			d.err = err
			return
		}
		d.premise(body, n+int(k), depth+1)
		return
	}
	if hd.Kind != ast.ASTApp || hd.Family != d.m.BasicFamilyID() {
		return
	}
	if hd.DeclKind == ast.ASTOpAnd {
		for i := uint32(0); i < hd.NumArgs && !d.stopped && d.err == nil; i++ {
			a, err := d.m.GetArg(h, i)
			if err != nil {
				d.err = err
				return
			}
			d.premise(a, n, depth+1)
		}
		return
	}
	if n < 2 || hd.NumArgs != 2 {
		return
	}
	isEq := hd.DeclKind == ast.ASTOpEq || hd.DeclKind == ast.ASTOpIff
	if !isEq && hd.DeclKind != ast.ASTOpImplies {
		return
	}
	args := d.args(h, 2)
	if len(args) != 2 {
		return
	}
	s, err := d.m.GetSort(args[0])
	if err != nil {
		d.err = err
		return
	}
	if s != d.boolean {
		return
	}
	for i, lhs := range args {
		if i > 0 && !isEq {
			break
		}
		rhs := args[1-i]
		d.definition(lhs, ast.ASTHandle{}, true, rhs, n)
		hd := d.header(lhs)
		if !isEq || hd.Kind != ast.ASTApp || hd.Family != d.m.BasicFamilyID() || hd.DeclKind != ast.ASTOpAnd || hd.NumArgs != 2 {
			continue
		}
		pair := d.args(lhs, 2)
		for j, head := range pair {
			selector := pair[1-j]
			pos := true
			sh := d.header(selector)
			if sh.Kind == ast.ASTApp && sh.Family == d.m.BasicFamilyID() && sh.DeclKind == ast.ASTOpNot && sh.NumArgs == 1 {
				a := d.args(selector, 1)
				if len(a) != 1 {
					return
				}
				selector, pos = a[0], false
			}
			d.definition(head, selector, pos, rhs, n)
		}
	}
}

// P entails the right side of P => R or P = R. For P & S = R and
// P & !S = T, every tuple of P satisfies R or T. Thus only equalities
// shared by the two right sides apply to P. The head must be a permutation
// of all the bound variables, so a matching head also fixes their sorts.
func (d *predicateDependencies) definition(head, selector ast.ASTHandle, positive bool, rhs ast.ASTHandle, n int) {
	hd := d.header(head)
	if hd.Kind != ast.ASTApp || hd.Family != ast.NullFamilyID || int(hd.NumArgs) != n {
		return
	}
	args := d.args(head, hd.NumArgs)
	if len(args) != n {
		return
	}
	var positions [16]int
	var used uint32
	for i, a := range args {
		v := d.variable(a, n)
		if v < 0 || used&(1<<v) != 0 {
			return
		}
		used |= 1 << v
		positions[v] = i
	}
	rules := d.facts(rhs, true, n, false)
	if len(rules) == 0 {
		return
	}
	if !selector.IsNull() {
		d.selectors[guardedPredicate{head, selector, positive}] = rules
		opposite := d.selectors[guardedPredicate{head, selector, !positive}]
		var common []argumentDependency
		for _, p := range rules {
			if hasArgumentDependency(opposite, p) {
				common = append(common, p)
			}
		}
		rules = common
	}
	for _, p := range rules {
		p.target, p.source = positions[p.target], positions[p.source]
		prev := d.rules[hd.Decl]
		if hasArgumentDependency(prev, p) {
			continue
		}
		if len(prev) == 0 {
			d.order = append(d.order, hd.Decl)
		}
		d.rules[hd.Decl] = append(prev, p)
	}
}

func hasArgumentDependency(list []argumentDependency, dep argumentDependency) bool {
	for _, x := range list {
		if x == dep {
			return true
		}
	}
	return false
}

func (d *predicateDependencies) writePremises(out *strings.Builder, l *enumerationLanguage) {
	for _, decl := range d.order {
		f := l.functions[decl]
		n := len(f.domain)
		args := make([]string, n)
		binders := make([]string, n)
		for i, h := range f.domain {
			args[i] = "garg" + strconv.Itoa(i)
			sort := l.sorts[h].name
			if h == l.boolean {
				sort = "Bool"
			}
			binders[i] = "(" + args[i] + " " + sort + ")"
		}
		var eqs []string
		for _, p := range d.rules[decl] {
			eqs = append(eqs, "(= "+args[p.target]+" ("+l.functions[p.function].name+" "+args[p.source]+"))")
		}
		con := eqs[0]
		if len(eqs) > 1 {
			con = "(and " + strings.Join(eqs, " ") + ")"
		}
		out.WriteString("(assert (forall (" + strings.Join(binders, " ") + ") (=> (" + f.name + " " + strings.Join(args, " ") + ") " + con + ")))\n")
	}
}

func (d *predicateDependencies) reduce(l *enumerationLanguage, base []ast.ASTHandle, original string) (string, error) {
	w := enumerationWriter{language: l, dependencies: d}
	text, err := w.assertions(base)
	if err != nil {
		if errors.Is(err, inapplicable) {
			return original, nil
		}
		return "", err
	}
	if !w.changed || d.stopped {
		return original, nil
	}
	// The projected definitions are only equivalent over the inferred support.
	// Keep that support explicit so the predicate cannot gain arbitrary tuples.
	var out strings.Builder
	d.writePremises(&out, l)
	out.WriteString(text)
	if out.Len()+l.declarations.Len()+l.axiomBytes+10*len(l.axioms) > maxEnumerationBytes {
		return original, nil
	}
	return out.String(), nil
}
