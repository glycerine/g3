package base

// Source: src/muz/base/rule_properties.h; src/muz/base/rule_properties.cpp.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrRuleProperties = errors.New("muz/base: invalid or closed rule properties")

type RulePropertyKind uint8

const (
	RulePropertyQuantifier RulePropertyKind = iota + 1
	RulePropertyUninterpreted
	RulePropertyNested
	RulePropertyInfiniteSort
	RulePropertyExistentialTail
	RulePropertyNegativePredicate
)

type RulePropertyError struct {
	Kind     RulePropertyKind
	RuleName string
	Symbol   string
	// SourceMessage preserves the source diagnostic while the offending rule
	// is still alive in the collected set.
	SourceMessage string
}

func (e *RulePropertyError) Error() string {
	suffix := ""
	if e.RuleName != "" {
		suffix = " in rule " + e.RuleName
	}
	switch e.Kind {
	case RulePropertyQuantifier:
		return "muz/base: cannot process quantifier" + suffix
	case RulePropertyUninterpreted:
		if e.Symbol != "" {
			return "muz/base: uninterpreted '" + e.Symbol + "'" + suffix
		}
		return "muz/base: uninterpreted symbol" + suffix
	case RulePropertyNested:
		return "muz/base: rule contains nested predicates" + suffix
	case RulePropertyInfiniteSort:
		return "muz/base: rule contains infinite sorts" + suffix
	case RulePropertyExistentialTail:
		return "muz/base: recursive predicate occurs nested in the body" + suffix
	case RulePropertyNegativePredicate:
		return "muz/base: rule contains negative predicate" + suffix
	default:
		return ErrRuleProperties.Error()
	}
}

// RuleProperties borrows the rules of the latest collected set.
// RuleManager supplies the source predicate classifier and AST manager.
type RuleProperties struct {
	self    *RuleProperties
	rules   *RuleManager
	manager *ast.ASTManager
	// quantifiers and uninterpFuns are m_quantifiers and m_uninterp_funs
	// (obj_map<quantifier, rule*> and obj_map<func_decl, rule*>): the check
	// reports the entry at begin(), so table order is observable. Num is an
	// index into collected, the rules of the latest collected set.
	quantifiers   *ast.ObjMap
	uninterpFuns  *ast.ObjMap
	collected     []*Rule
	nested        []*Rule
	negative      *Rule
	infinite      *Rule
	generateProof bool
	closed        bool
}

// SetGenerateProof enables source rule-property collection's creation of
// asserted proofs for rules that do not already have one.
func (p *RuleProperties) SetGenerateProof(enabled bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.generateProof = enabled
	return nil
}

func NewRuleProperties(rules *RuleManager) (*RuleProperties, error) {
	if rules == nil {
		return nil, ErrRuleProperties
	}
	if err := rules.open(); err != nil {
		return nil, err
	}
	p := &RuleProperties{rules: rules, manager: rules.manager, quantifiers: ast.NewObjMap(rules.manager), uninterpFuns: ast.NewObjMap(rules.manager)}
	p.self = p
	return p, nil
}

func (p *RuleProperties) open() error {
	if p == nil || p.self != p || p.closed || p.rules == nil || p.manager == nil {
		return ErrRuleProperties
	}
	return p.rules.open()
}

func (p *RuleProperties) clearState() {
	if p.quantifiers != nil {
		p.quantifiers.Reset()
	}
	if p.uninterpFuns != nil {
		p.uninterpFuns.Reset()
	}
	p.collected = nil
	p.nested = nil
	p.negative = nil
	p.infinite = nil
}

func (p *RuleProperties) Reset() error {
	if err := p.open(); err != nil {
		return err
	}
	p.clearState()
	return nil
}

func (p *RuleProperties) appendNested(rule *Rule) {
	if len(p.nested) == 0 || p.nested[len(p.nested)-1] != rule {
		p.nested = append(p.nested, rule)
	}
}

// collectExpression is for_each_expr_core<rule_properties, expr_sparse_mark,
// MarkAll=true, IgnorePatterns=false>: a post-order walk whose visited mark
// is shared by every rule of the collected set.
func (p *RuleProperties) collectExpression(ruleIndex int, root ast.ASTHandle, visited map[ast.ASTHandle]bool) error {
	if visited[root] {
		return nil
	}
	visited[root] = true
	type frame struct {
		expr ast.ASTHandle
		next uint32
	}
	stack := []frame{{expr: root}}
	for len(stack) != 0 {
		top := len(stack) - 1
		current := stack[top].expr
		kind, err := p.manager.GetKind(current)
		if err != nil {
			return err
		}
		var children uint32
		switch kind {
		case ast.ASTVar:
			stack = stack[:top]
			continue
		case ast.ASTApp:
			if children, err = p.manager.GetNumArgs(current); err != nil {
				return err
			}
		case ast.ASTQuantifier:
			patterns, e := p.manager.GetNumPatterns(current)
			if e != nil {
				return e
			}
			noPatterns, e := p.manager.GetNumNoPatterns(current)
			if e != nil {
				return e
			}
			children = 1 + patterns + noPatterns
		default:
			return ErrRuleProperties
		}
		pushed := false
		for !pushed && stack[top].next < children {
			index := stack[top].next
			stack[top].next++
			child, e := p.quantifierOrAppChild(current, kind, index)
			if e != nil {
				return e
			}
			if visited[child] {
				continue
			}
			visited[child] = true
			childKind, e := p.manager.GetKind(child)
			if e != nil {
				return e
			}
			switch childKind {
			case ast.ASTVar:
			case ast.ASTApp:
				if kind == ast.ASTApp {
					n, x := p.manager.GetNumArgs(child)
					if x != nil {
						return x
					}
					if n == 0 {
						if x = p.collectApp(ruleIndex, child); x != nil {
							return x
						}
						continue
					}
				}
				stack = append(stack, frame{expr: child})
				pushed = true
			default:
				stack = append(stack, frame{expr: child})
				pushed = true
			}
		}
		if pushed {
			continue
		}
		stack = stack[:top]
		if kind == ast.ASTApp {
			err = p.collectApp(ruleIndex, current)
		} else {
			err = p.quantifiers.InsertNum(current, uint64(ruleIndex))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *RuleProperties) quantifierOrAppChild(e ast.ASTHandle, kind ast.ASTKind, index uint32) (ast.ASTHandle, error) {
	if kind == ast.ASTApp {
		return p.manager.GetArg(e, index)
	}
	if index == 0 {
		return p.manager.GetQuantifierBody(e)
	}
	patterns, err := p.manager.GetNumPatterns(e)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if index <= patterns {
		return p.manager.GetPattern(e, index-1)
	}
	return p.manager.GetNoPattern(e, index-1-patterns)
}

// collectApp is rule_properties::operator()(app*).
func (p *RuleProperties) collectApp(ruleIndex int, expression ast.ASTHandle) error {
	predicate, err := p.rules.isPredicate(expression)
	if err != nil {
		return err
	}
	if predicate {
		p.appendNested(p.collected[ruleIndex])
		return nil
	}
	declaration, err := p.manager.GetDecl(expression)
	if err != nil {
		return err
	}
	family, err := p.manager.GetFamilyID(declaration)
	if err != nil {
		return err
	}
	if family == ast.NullFamilyID {
		rangeSort, x := p.manager.GetRange(declaration)
		if x != nil {
			return x
		}
		isRuleSort, x := p.manager.IsDatalogRuleSort(rangeSort)
		if x != nil {
			return x
		}
		if !isRuleSort {
			return p.uninterpFuns.InsertNum(declaration, uint64(ruleIndex))
		}
		return nil
	}
	if datatypeFamily, x := p.manager.DatatypeFamilyID(); x == nil && family == datatypeFamily {
		declKind, x := p.manager.GetDeclKind(declaration)
		if x != nil {
			return x
		}
		if declKind == ast.ASTDatatypeAccessor {
			argument, x := p.manager.GetArg(expression, 0)
			if x != nil {
				return x
			}
			sort, x := p.manager.GetSort(argument)
			if x != nil {
				return x
			}
			constructors, x := p.manager.DatatypeConstructors(sort)
			if x != nil {
				return x
			}
			if len(constructors) > 1 {
				return p.uninterpFuns.InsertNum(declaration, uint64(ruleIndex))
			}
		}
	}
	return nil
}

func (p *RuleProperties) Collect(set *RuleSet) error {
	if err := p.open(); err != nil {
		return err
	}
	if set == nil || set.manager != p.rules {
		return ErrRuleProperties
	}
	if err := p.Reset(); err != nil {
		return err
	}
	count, err := set.Len()
	if err != nil {
		return err
	}
	visited := make(map[ast.ASTHandle]bool)
	p.collected = append([]*Rule(nil), set.rules[:count]...)
	for i := 0; i < count; i++ {
		// Source rule_properties keeps raw rule pointers: the collected set
		// owns its rules, and collection takes no references.
		rule := set.rules[i]
		if p.negative == nil {
			negative, x := rule.HasNegation()
			if x != nil {
				return x
			}
			if negative {
				p.negative = rule
			}
		}
		uninterpreted, e := rule.UninterpretedTailSize()
		if e != nil {
			return e
		}
		tails, e := rule.TailSize()
		if e != nil {
			return e
		}
		for j := uninterpreted; j < tails; j++ {
			tail, _, x := rule.Tail(j)
			if x != nil {
				return x
			}
			if x = p.collectExpression(i, tail, visited); x != nil {
				return x
			}
		}
		if p.generateProof {
			if e = p.rules.MkRuleAssertedProof(rule); e != nil {
				return e
			}
		}
		if p.infinite == nil {
			head, x := rule.HeadDeclaration()
			if x != nil {
				return x
			}
			arity, x := p.manager.GetArity(head)
			if x != nil {
				return x
			}
			for column := uint32(0); column < arity; column++ {
				sort, x := p.manager.GetDomain(head, column)
				if x != nil {
					return x
				}
				size, x := p.manager.GetSortSize(sort)
				if x != nil {
					return x
				}
				ruleSort, x := p.manager.IsDatalogRuleSort(sort)
				if x != nil {
					return x
				}
				if !size.IsFinite() && !ruleSort {
					p.infinite = rule
					break
				}
			}
		}
	}
	return nil
}

func rulePropertyName(rule *Rule) string {
	if rule == nil {
		return ""
	}
	name, _ := rule.NameText()
	return name
}

func (p *RuleProperties) CheckQuantifierFree() error {
	if err := p.open(); err != nil {
		return err
	}
	if entries := p.quantifiers.Entries(); len(entries) != 0 {
		rule := p.collected[entries[0].Num]
		var display strings.Builder
		if err := p.rules.WriteRuleDisplay(&display, rule); err != nil {
			return err
		}
		return &RulePropertyError{Kind: RulePropertyQuantifier, RuleName: rulePropertyName(rule), SourceMessage: "cannot process quantifier in rule " + display.String()}
	}
	return nil
}

func (p *RuleProperties) CheckUninterpretedFree() error {
	if err := p.open(); err != nil {
		return err
	}
	entries := p.uninterpFuns.Entries()
	if len(entries) == 0 {
		return nil
	}
	rule := p.collected[entries[0].Num]
	name := ""
	if symbol, err := p.manager.GetASTName(entries[0].Key); err == nil {
		name, _ = p.manager.Symbols.Display(symbol)
	}
	var display strings.Builder
	if err := p.rules.WriteRuleDisplay(&display, rule); err != nil {
		return err
	}
	return &RulePropertyError{Kind: RulePropertyUninterpreted, RuleName: rulePropertyName(rule), Symbol: name, SourceMessage: "Uninterpreted '" + name + "' in " + display.String()}
}

func (p *RuleProperties) CheckNestedFree() error {
	if err := p.open(); err != nil {
		return err
	}
	if len(p.nested) != 0 {
		var display strings.Builder
		if err := p.rules.WriteRuleDisplay(&display, p.nested[0]); err != nil {
			return err
		}
		return &RulePropertyError{Kind: RulePropertyNested, RuleName: rulePropertyName(p.nested[0]), SourceMessage: "Rule contains nested predicates " + display.String()}
	}
	return nil
}

func (p *RuleProperties) CheckInfiniteSorts() error {
	if err := p.open(); err != nil {
		return err
	}
	if p.infinite != nil {
		var display strings.Builder
		if err := p.rules.WriteRuleDisplay(&display, p.infinite); err != nil {
			return err
		}
		return &RulePropertyError{Kind: RulePropertyInfiniteSort, RuleName: rulePropertyName(p.infinite), SourceMessage: "Rule contains infinite sorts in rule " + display.String()}
	}
	return nil
}

func (p *RuleProperties) CheckNegativePredicates() error {
	if err := p.open(); err != nil {
		return err
	}
	if p.negative != nil {
		var display strings.Builder
		if err := p.rules.WriteRuleDisplay(&display, p.negative); err != nil {
			return err
		}
		return &RulePropertyError{Kind: RulePropertyNegativePredicate, RuleName: rulePropertyName(p.negative), SourceMessage: "Rule contains negative predicate " + display.String()}
	}
	return nil
}

func (p *RuleProperties) containsPredicate(root ast.ASTHandle) (bool, error) {
	stack := []ast.ASTHandle{root}
	seen := make(map[ast.ASTHandle]bool)
	for len(stack) != 0 {
		i := len(stack) - 1
		e := stack[i]
		stack = stack[:i]
		if seen[e] {
			continue
		}
		seen[e] = true
		kind, err := p.manager.GetKind(e)
		if err != nil {
			return false, err
		}
		switch kind {
		case ast.ASTApp:
			predicate, x := p.rules.isPredicate(e)
			if x != nil {
				return false, x
			}
			if predicate {
				return true, nil
			}
			n, x := p.manager.GetNumArgs(e)
			if x != nil {
				return false, x
			}
			for j := uint32(0); j < n; j++ {
				child, y := p.manager.GetArg(e, j)
				if y != nil {
					return false, y
				}
				stack = append(stack, child)
			}
		case ast.ASTQuantifier:
			body, x := p.manager.GetQuantifierBody(e)
			if x != nil {
				return false, x
			}
			stack = append(stack, body)
		}
	}
	return false, nil
}

func (p *RuleProperties) CheckExistentialTail() error {
	if err := p.open(); err != nil {
		return err
	}
	// Source appends all interpreted tails before consuming the shared LIFO
	// worklist, then checks its candidate expressions in insertion order.
	type candidate struct {
		expr ast.ASTHandle
		rule *Rule
	}
	var todo, toCheck []candidate
	for _, rule := range p.nested {
		uninterpreted, err := rule.UninterpretedTailSize()
		if err != nil {
			return err
		}
		tails, err := rule.TailSize()
		if err != nil {
			return err
		}
		for i := uninterpreted; i < tails; i++ {
			tail, _, err := rule.Tail(i)
			if err != nil {
				return err
			}
			todo = append(todo, candidate{tail, rule})
		}
	}
	visited := make(map[ast.ASTHandle]bool)
	for len(todo) != 0 {
		i := len(todo) - 1
		item := todo[i]
		todo = todo[:i]
		e := item.expr
		if visited[e] {
			continue
		}
		visited[e] = true
		kind, err := p.manager.GetKind(e)
		if err != nil {
			return err
		}
		if kind == ast.ASTQuantifier {
			body, err := p.manager.GetQuantifierBody(e)
			if err != nil {
				return err
			}
			toCheck = append(toCheck, candidate{body, item.rule})
			continue
		}
		if kind != ast.ASTApp {
			toCheck = append(toCheck, item)
			continue
		}
		predicate, err := p.rules.isPredicate(e)
		if err != nil {
			return err
		}
		if predicate {
			continue
		}
		op, err := p.manager.GetDeclKind(e)
		if err != nil {
			return err
		}
		n, err := p.manager.GetNumArgs(e)
		if err != nil {
			return err
		}
		switch op {
		case ast.ASTOpAnd, ast.ASTOpOr:
			for j := uint32(0); j < n; j++ {
				child, err := p.manager.GetArg(e, j)
				if err != nil {
					return err
				}
				todo = append(todo, candidate{child, item.rule})
			}
		case ast.ASTOpImplies:
			if n != 2 {
				return fmt.Errorf("%w: malformed implication", ErrRuleProperties)
			}
			left, err := p.manager.GetArg(e, 0)
			if err != nil {
				return err
			}
			right, err := p.manager.GetArg(e, 1)
			if err != nil {
				return err
			}
			toCheck = append(toCheck, candidate{left, item.rule})
			todo = append(todo, candidate{right, item.rule})
		case ast.ASTOpEq, ast.ASTOpIff:
			if n != 2 {
				toCheck = append(toCheck, item)
				continue
			}
			left, err := p.manager.GetArg(e, 0)
			if err != nil {
				return err
			}
			right, err := p.manager.GetArg(e, 1)
			if err != nil {
				return err
			}
			if p.manager.IsTrue(left) {
				todo = append(todo, candidate{right, item.rule})
			} else if p.manager.IsTrue(right) {
				todo = append(todo, candidate{left, item.rule})
			} else {
				toCheck = append(toCheck, item)
			}
		default:
			toCheck = append(toCheck, item)
		}
	}
	for _, item := range toCheck {
		found, err := p.containsPredicate(item.expr)
		if err != nil {
			return err
		}
		if found {
			display, err := p.rules.ruleHeadSourceDisplay(item.expr)
			if err != nil {
				return err
			}
			return &RulePropertyError{Kind: RulePropertyExistentialTail, RuleName: rulePropertyName(item.rule), SourceMessage: "recursive predicate " + display + " occurs nested in the body of a rule"}
		}
	}
	return nil
}

func (p *RuleProperties) Close() error {
	if p == nil || p.self != p {
		return ErrRuleProperties
	}
	if p.closed {
		return nil
	}
	if err := p.Reset(); err != nil {
		return err
	}
	p.closed = true
	p.rules = nil
	p.manager = nil
	return nil
}
