package smt2

// Source: src/parsers/smt2/smt2parser.cpp parse_sexpr (a CPK_SEXPR command
// argument) and src/cmd_context/tactic_cmds.cpp sexpr2tactic, sexpr2probe
// and the mk_* combinators: a tactic argument is read as an s-expression
// first and converted when the command executes, so the combinators'
// texts carry the positions of the s-expression nodes (a composite's
// position is its closing ')').

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type tacticSExprKind uint8

const (
	sexprSymbol tacticSExprKind = iota
	sexprKeyword
	sexprString
	sexprNumeral
	sexprBitVector
	sexprComposite
)

type tacticSExpr struct {
	kind      tacticSExprKind
	text      string // symbol or keyword id, string contents
	number    unumeral.Rational
	numbers   *unumeral.RationalManager
	line, pos int
	children  []*tacticSExpr
}

func (n *tacticSExpr) release() (err error) {
	if n == nil {
		return nil
	}
	for _, child := range n.children {
		err = errors.Join(err, child.release())
	}
	if n.numbers != nil {
		err = errors.Join(err, n.numbers.Del(&n.number))
		n.numbers = nil
	}
	return err
}

func (n *tacticSExpr) errorf(format string, args ...any) error {
	return &ScannerError{Message: fmt.Sprintf(format, args...), Line: n.line, Column: n.pos}
}

func (n *tacticSExpr) isSymbol(text string) bool { return n.kind == sexprSymbol && n.text == text }

func (n *tacticSExpr) isNumeral() bool { return n.kind == sexprNumeral || n.kind == sexprBitVector }

// readTacticSExpr is parse_sexpr over the token stream.
func (p *commandParser) readTacticSExpr() (result *tacticSExpr, err error) {
	var stack []*tacticSExpr // open composites
	var done *tacticSExpr
	defer func() {
		if err != nil {
			for _, open := range stack {
				err = errors.Join(err, open.release())
			}
			err = errors.Join(err, done.release())
			result = nil
		}
	}()
	for {
		line, pos := p.scanner.Line(), p.scanner.Position()
		var node *tacticSExpr
		switch p.token {
		case LeftParenToken:
			stack = append(stack, &tacticSExpr{kind: sexprComposite})
		case RightParenToken:
			if len(stack) == 0 {
				return nil, fmt.Errorf("invalid s-expression, unexpected ')'")
			}
			top := stack[len(stack)-1]
			if len(top.children) == 0 {
				return nil, fmt.Errorf("invalid empty s-expression")
			}
			stack = stack[:len(stack)-1]
			top.line, top.pos = line, pos
			node = top
		case SymbolToken:
			node = &tacticSExpr{kind: sexprSymbol, text: p.scanner.ID(), line: line, pos: pos}
		case KeywordToken:
			node = &tacticSExpr{kind: sexprKeyword, text: p.scanner.ID(), line: line, pos: pos}
		case StringToken:
			node = &tacticSExpr{kind: sexprString, text: p.scanner.String(), line: line, pos: pos}
		case IntegerToken, DecimalToken, BitVectorToken:
			kind := sexprNumeral
			if p.token == BitVectorToken {
				kind = sexprBitVector
			}
			node = &tacticSExpr{kind: kind, number: unumeral.NewRational(), line: line, pos: pos}
			if e := p.scanner.Numbers().Copy(&node.number, p.scanner.Number()); e != nil {
				return nil, e
			}
			node.numbers = p.scanner.Numbers()
		case EOFToken:
			return nil, fmt.Errorf("invalid s-expression, unexpected end of file")
		default:
			return nil, fmt.Errorf("invalid s-expression, unexpected input")
		}
		if node != nil {
			if len(stack) == 0 {
				done = node
			} else {
				stack[len(stack)-1].children = append(stack[len(stack)-1].children, node)
			}
		}
		if err = p.advance(); err != nil {
			return nil, err
		}
		if len(stack) == 0 {
			return done, nil
		}
	}
}

// parseTactic reads a tactic s-expression and converts it, returning one
// counted reference.
func (p *commandParser) parseTactic() (result tactic.Tactic, err error) {
	n, err := p.readTacticSExpr()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, n.release()) }()
	return p.sexprToTactic(n)
}

// sexprToTactic is sexpr2tactic; it returns one counted reference.
func (p *commandParser) sexprToTactic(n *tacticSExpr) (result tactic.Tactic, err error) {
	var children []tactic.Tactic
	defer func() {
		for _, child := range children {
			err = errors.Join(err, child.DecRef())
		}
	}()
	switch n.kind {
	case sexprSymbol:
		name := n.text
		if !cmd_context.IsBuiltinTactic(name) {
			if body, found := p.context.UserTacticBody(name); found {
				return p.parseDeclaredTactic(name, body)
			}
		}
		result, err = p.context.NewBuiltinTacticWithOutput(name, p.out)
		if err != nil && strings.HasPrefix(err.Error(), "invalid tactic, unknown tactic ") {
			return nil, n.errorf("%s", err.Error())
		}
	case sexprComposite:
		result, err = p.sexprToCombinator(n, &children)
	default:
		return nil, n.errorf("invalid tactic, unexpected input")
	}
	if err != nil {
		return nil, err
	}
	if err = result.IncRef(); err != nil {
		return nil, err
	}
	return result, nil
}

// sexprToCombinator is sexpr2tactic's composite case and the mk_*
// combinators. Converted children are appended to children, which the
// caller releases.
func (p *commandParser) sexprToCombinator(n *tacticSExpr, children *[]tactic.Tactic) (result tactic.Tactic, err error) {
	child := func(i int) (tactic.Tactic, error) {
		value, e := p.sexprToTactic(n.children[i])
		if e != nil {
			return nil, e
		}
		*children = append(*children, value)
		return value, nil
	}
	all := func() ([]tactic.Tactic, error) {
		var values []tactic.Tactic
		for i := 1; i < len(n.children); i++ {
			value, e := child(i)
			if e != nil {
				return nil, e
			}
			values = append(values, value)
		}
		return values, nil
	}
	unsignedChild := func(i int) (uint32, bool, error) {
		c := n.children[i]
		if !c.isNumeral() {
			return 0, false, nil
		}
		ok, e := c.numbers.IsUnsigned(&c.number)
		if e != nil || !ok {
			return 0, false, e
		}
		value, e := c.numbers.Unsigned(&c.number)
		return value, e == nil, e
	}
	count := len(n.children)
	head := n.children[0]
	if head.kind != sexprSymbol {
		return nil, n.errorf("invalid tactic, symbol expected")
	}
	switch name := head.text; name {
	case "and-then", "then", "or-else", "par", "par-or", "par-then":
		label := map[string]string{"and-then": "and-then", "then": "and-then", "or-else": "or-else", "par": "par-or", "par-or": "par-or", "par-then": "par-then"}[name]
		if count < 2 {
			return nil, n.errorf("invalid %s combinator, at least one argument expected", label)
		}
		if count == 2 {
			return child(1)
		}
		values, e := all()
		if e != nil {
			return nil, e
		}
		switch label {
		case "and-then":
			return tactic.NewAndThenTactic(values...)
		case "or-else":
			return tactic.NewOrElseTactic(values...)
		case "par-then":
			return tactic.NewParAndThenTactic(values...)
		default:
			return tactic.NewParTactic(values...)
		}
	case "try-for":
		if count != 3 {
			return nil, n.errorf("invalid try-for combinator, two arguments expected")
		}
		timeout, ok, e := unsignedChild(2)
		if e != nil {
			return nil, e
		}
		if !ok {
			return nil, n.errorf("invalid try-for combinator, second argument must be an unsigned integer")
		}
		t, e := child(1)
		if e != nil {
			return nil, e
		}
		return tactic.NewTryForTactic(t, timeout)
	case "repeat", "fail-if-branching":
		if count != 3 && count != 2 {
			return nil, n.errorf("invalid %s combinator, one or two arguments expected", name)
		}
		bound := ^uint32(0)
		if name == "fail-if-branching" {
			bound = 1
		}
		if count == 3 {
			value, ok, e := unsignedChild(2)
			if e != nil {
				return nil, e
			}
			if !ok {
				return nil, n.errorf("invalid %s combinator, second argument must be an unsigned integer", name)
			}
			bound = value
		}
		t, e := child(1)
		if e != nil {
			return nil, e
		}
		if name == "repeat" {
			return tactic.NewRepeatTactic(t, bound)
		}
		return tactic.NewFailIfBranchingTactic(t, bound)
	case "if", "ite", "cond", "when", "fail-if":
		want := map[string]int{"fail-if": 2, "when": 3}[name]
		if want == 0 {
			want = 4
		}
		if count != want {
			switch name {
			case "fail-if":
				return nil, n.errorf("invalid fail-if tactic, one argument expected")
			case "when":
				return nil, n.errorf("invalid when combinator, two arguments expected")
			default:
				return nil, n.errorf("invalid if/conditional combinator, three arguments expected")
			}
		}
		probe, e := p.sexprToProbe(n.children[1])
		if e != nil {
			return nil, e
		}
		defer func() { err = errors.Join(err, probe.DecRef()) }()
		if name == "fail-if" {
			return tactic.NewFailIfTactic(probe)
		}
		t, e := child(2)
		if e != nil {
			return nil, e
		}
		if name == "when" {
			return tactic.NewWhenTactic(probe, t)
		}
		f, e := child(3)
		if e != nil {
			return nil, e
		}
		return tactic.NewCondTactic(probe, t, f)
	case "!", "using-params", "with":
		return p.sexprUsingParams(n, child)
	case "echo":
		return p.sexprEcho(n)
	case "if-no-proofs", "if-no-models", "if-no-unsat-cores", "skip-if-failed":
		if count != 2 {
			return nil, n.errorf("invalid %s combinator, one argument expected", name)
		}
		t, e := child(1)
		if e != nil {
			return nil, e
		}
		switch name {
		case "if-no-proofs":
			return tactic.NewIfNoProofsTactic(t)
		case "if-no-models":
			return tactic.NewIfNoModelsTactic(t)
		case "if-no-unsat-cores":
			return tactic.NewIfNoUnsatCoresTactic(t)
		default:
			return tactic.NewSkipIfFailedTactic(t)
		}
	default:
		return nil, n.errorf("invalid tactic, unknown tactic combinator %s", name)
	}
}

// sexprUsingParams is mk_using_params.
func (p *commandParser) sexprUsingParams(n *tacticSExpr, child func(int) (tactic.Tactic, error)) (result tactic.Tactic, err error) {
	count := len(n.children)
	if count < 2 {
		return nil, n.errorf("invalid using-params combinator, at least one argument expected")
	}
	t, err := child(1)
	if err != nil || count == 2 {
		return t, err
	}
	globals, err := p.context.GlobalParameters()
	if err != nil {
		return nil, err
	}
	c := globals.Context
	settings, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, c.Del(&settings)) }()
	m, err := p.astManager()
	if err != nil {
		return nil, err
	}
	descriptions, err := uparams.NewParamDescrs(m.Symbols, uparams.ParamDescrsConfig{Budget: m.Budget()})
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, descriptions.Close()) }()
	if err = t.CollectParamDescrs(descriptions); err != nil {
		return nil, err
	}
	for i := 2; i < count; {
		keyword := n.children[i]
		i++
		if keyword.kind != sexprKeyword {
			return nil, keyword.errorf("invalid using-params combinator, keyword expected")
		}
		if i == count {
			return nil, keyword.errorf("invalid using-params combinator, parameter value expected")
		}
		key := strings.TrimPrefix(keyword.text, ":")
		key = uparams.NormParamName(&key)
		value := n.children[i]
		i++
		symbol, e := m.Symbols.Intern(key)
		if e != nil {
			return nil, e
		}
		kind, e := descriptions.KindInModule(&symbol)
		if e != nil {
			return nil, e
		}
		if key, e = m.Symbols.String(symbol); e != nil {
			return nil, e
		}
		if err = p.sexprParameter(c, &settings, key, kind, value); err != nil {
			return nil, err
		}
	}
	return tactic.NewUsingParamsTactic(c, t, &settings)
}

// sexprParameter is mk_using_params' switch on the parameter kind.
func (p *commandParser) sexprParameter(c *uparams.ParamsContext, settings *uparams.ParamsRef, key string, kind uparams.ParamKind, v *tacticSExpr) (err error) {
	switch kind {
	case uparams.ParamInvalid:
		return v.errorf("invalid using-params combinator, unknown parameter %s", key)
	case uparams.ParamBool:
		if v.kind != sexprSymbol || v.text != "true" && v.text != "false" {
			return v.errorf("invalid parameter value, true or false expected")
		}
		return c.SetBoolName(settings, &key, v.text == "true")
	case uparams.ParamUInt:
		ok := false
		var value uint32
		if v.isNumeral() {
			if ok, err = v.numbers.IsUnsigned(&v.number); err != nil {
				return err
			}
			if ok {
				if value, err = v.numbers.Unsigned(&v.number); err != nil {
					return err
				}
			}
		}
		if !ok {
			return v.errorf("invalid parameter value, unsigned integer expected")
		}
		return c.SetUIntName(settings, &key, value)
	case uparams.ParamNumeral:
		if !v.isNumeral() {
			return v.errorf("invalid parameter value, numeral expected")
		}
		r, e := c.Numerals().NewValue()
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, c.Numerals().Del(&r)) }()
		if err = c.Numerals().Manager.Copy(&r.Raw, &v.number); err != nil {
			return err
		}
		return c.SetRatName(settings, &key, &r)
	case uparams.ParamSymbol:
		if v.kind != sexprSymbol {
			return v.errorf("invalid parameter value, symbol expected")
		}
		sym, e := c.SymbolsField.Intern(v.text)
		if e != nil {
			return e
		}
		return c.SetSymbolName(settings, &key, sym)
	case uparams.ParamDouble:
		if !v.isNumeral() {
			return v.errorf("invalid parameter value, numeral expected")
		}
		value, e := v.numbers.Float64(&v.number)
		if e != nil {
			return e
		}
		return c.SetDoubleName(settings, &key, value)
	default:
		return fmt.Errorf("invalid using-params combinator, unsupported parameter kind")
	}
}

// sexprEcho is mk_echo.
func (p *commandParser) sexprEcho(n *tacticSExpr) (result tactic.Tactic, err error) {
	count := len(n.children)
	if count < 2 {
		return nil, n.errorf("invalid echo tactic, must have at least one argument")
	}
	var steps []tactic.Tactic
	defer func() {
		for _, step := range steps {
			err = errors.Join(err, step.DecRef())
		}
	}()
	for i := 1; i < count; i++ {
		current := n.children[i]
		last := i == count-1
		var step tactic.Tactic
		if current.kind == sexprString {
			step, err = cmd_context.NewEchoTactic(p.context, current.text, last)
		} else {
			probe, e := p.sexprToProbe(current)
			if e != nil {
				return nil, e
			}
			step, err = cmd_context.NewProbeValueTactic(p.context, nil, probe, last)
			err = errors.Join(err, probe.DecRef())
		}
		if err != nil {
			return nil, err
		}
		if err = step.IncRef(); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	if len(steps) == 1 {
		return steps[0], nil
	}
	return tactic.NewAndThenTactic(steps...)
}

// sexprToProbe is sexpr2probe; it returns one counted reference.
func (p *commandParser) sexprToProbe(n *tacticSExpr) (result tactic.ProbeEvaluator, err error) {
	manager, err := p.astManager()
	if err != nil {
		return nil, err
	}
	var arguments []tactic.ProbeEvaluator
	defer func() {
		for _, argument := range arguments {
			err = errors.Join(err, argument.DecRef())
		}
	}()
	argument := func(i int) (tactic.ProbeEvaluator, error) {
		value, e := p.sexprToProbe(n.children[i])
		if e != nil {
			return nil, e
		}
		arguments = append(arguments, value)
		return value, nil
	}
	switch n.kind {
	case sexprSymbol:
		result, err = p.sourceBuiltinProbe(n.text, manager)
		if err == nil && result == nil {
			return nil, n.errorf("invalid probe, unknown builtin probe %s", n.text)
		}
	case sexprNumeral, sexprBitVector:
		fits, e := n.numbers.IsInt32(&n.number)
		if e != nil {
			return nil, e
		}
		if !fits {
			return nil, n.errorf("invalid probe, constant is too big to fit in a fixed size integer")
		}
		value, e := n.numbers.Int64(&n.number)
		if e != nil {
			return nil, e
		}
		result, err = tactic.NewProbe(manager.Budget(), tactic.ProbeConstant, float64(value))
	case sexprComposite:
		head := n.children[0]
		if head.kind != sexprSymbol {
			return nil, n.errorf("invalid probe, symbol expected")
		}
		count := len(n.children)
		if head.text == "not" {
			if count != 2 {
				return nil, n.errorf("invalid probe expression, one argument expected")
			}
			a, e := argument(1)
			if e != nil {
				return nil, e
			}
			result, err = tactic.NewNotProbe(manager.Budget(), a)
			break
		}
		kind, found := sourceProbeOperator(head.text)
		if !found {
			return nil, n.errorf("invalid probe, unknown probe expression %s", head.text)
		}
		// MK_BIN_PROBE: =, <=, <, >=, >, =>, / and - take exactly two
		// arguments.
		binary := kind >= tactic.ProbeEqual && kind <= tactic.ProbeGreater ||
			kind == tactic.ProbeImplies || kind == tactic.ProbeDivide || kind == tactic.ProbeSubtract
		if binary {
			if count != 3 {
				return nil, n.errorf("invalid probe expression, two arguments expected")
			}
			a, e := argument(1)
			if e != nil {
				return nil, e
			}
			b, e := argument(2)
			if e != nil {
				return nil, e
			}
			result, err = tactic.NewBinaryProbe(manager.Budget(), kind, a, b)
			break
		}
		// MK_NARY_PROBE (FORK FIX #40: each argument folds in once).
		if count < 2 {
			return nil, n.errorf("invalid probe, at least one argument expected")
		}
		result, err = argument(1)
		for i := 2; err == nil && i < count; i++ {
			next, e := argument(i)
			if e != nil {
				return nil, e
			}
			result, err = tactic.NewBinaryProbe(manager.Budget(), kind, result, next)
		}
	default:
		return nil, n.errorf("invalid probe, unexpected input")
	}
	if err != nil {
		return nil, err
	}
	if err = result.IncRef(); err != nil {
		return nil, err
	}
	return result, nil
}
