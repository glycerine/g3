package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	muzrel "github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// externalPluginLog is the C API reference program's callback state
// (Z3_fixedpoint_init, Z3_fixedpoint_set_reduce_assign_callback,
// Z3_fixedpoint_set_reduce_app_callback): each relation term is modelled by
// an emptiness flag, and every call is logged with its operator, decl
// parameters, arguments and outputs.
type externalPluginLog struct {
	t        *testing.T
	c        *SMTContext
	m        *ast.ASTManager
	out      strings.Builder
	nonempty map[ast.ASTHandle]bool
	fresh    int
	refs     []ast.ASTRef
}

var externalPluginOpNames = map[int32]string{
	ast.DatalogOpStore: "store", ast.DatalogOpEmpty: "empty", ast.DatalogOpIsEmpty: "is_empty",
	ast.DatalogOpJoin: "join", ast.DatalogOpUnion: "union", ast.DatalogOpWiden: "widen",
	ast.DatalogOpProject: "project", ast.DatalogOpFilter: "filter",
	ast.DatalogOpNegationFilter: "negation_filter", ast.DatalogOpRename: "rename",
	ast.DatalogOpComplement: "complement", ast.DatalogOpSelect: "select", ast.DatalogOpClone: "clone",
}

func (l *externalPluginLog) str(h ast.ASTHandle) string {
	s, err := l.c.ASTString(h, APIASTPrintSMTFull)
	astTestMust(l.t, err)
	return s
}

func (l *externalPluginLog) keep(h ast.ASTHandle) ast.ASTHandle {
	ref, err := l.m.NewRef(h)
	astTestMust(l.t, err)
	l.refs = append(l.refs, ref)
	return h
}

func (l *externalPluginLog) logDecl(what string, decl ast.ASTHandle, args []ast.ASTHandle) int32 {
	kind, err := l.m.GetDeclKind(decl)
	astTestMust(l.t, err)
	fmt.Fprintf(&l.out, "%s %s", what, externalPluginOpNames[kind])
	parameters, err := l.m.GetDeclParametersCopy(decl)
	astTestMust(l.t, err)
	for i := range parameters {
		pk, err := l.m.ParameterKind(parameters[i])
		astTestMust(l.t, err)
		switch pk {
		case ast.ASTParameterInt:
			v, err := l.m.ParameterInt(parameters[i])
			astTestMust(l.t, err)
			fmt.Fprintf(&l.out, " %d", v)
		case ast.ASTParameterAST:
			v, err := l.m.ParameterAST(parameters[i])
			astTestMust(l.t, err)
			if k, err := l.m.GetKind(v); err == nil && k == ast.ASTSort {
				s, err := l.c.SortString(v, APIASTPrintSMTFull)
				astTestMust(l.t, err)
				fmt.Fprintf(&l.out, " <%s>", s)
			} else {
				fmt.Fprintf(&l.out, " [%s]", l.str(v))
			}
		default:
			fmt.Fprintf(&l.out, " ?")
		}
		astTestMust(l.t, l.m.DelParameter(&parameters[i]))
	}
	fmt.Fprintf(&l.out, " |")
	for _, a := range args {
		fmt.Fprintf(&l.out, " %s", l.str(a))
	}
	return kind
}

func (l *externalPluginLog) callbacks() muzrel.ExternalRelationCallbacks {
	return muzrel.ExternalRelationCallbacks{
		ReduceAssign: func(decl ast.ASTHandle, args, outs []ast.ASTHandle) error {
			kind := l.logDecl("assign", decl, args)
			fmt.Fprintf(&l.out, " ->")
			for _, o := range outs {
				fmt.Fprintf(&l.out, " %s", l.str(o))
			}
			fmt.Fprintf(&l.out, "\n")
			switch kind {
			case ast.DatalogOpEmpty:
				l.nonempty[l.keep(outs[0])] = false
			case ast.DatalogOpStore:
				l.nonempty[l.keep(outs[0])] = true
			case ast.DatalogOpClone:
				l.nonempty[l.keep(outs[0])] = l.nonempty[args[0]]
			case ast.DatalogOpUnion, ast.DatalogOpWiden:
				was, src := l.nonempty[args[0]], l.nonempty[args[1]]
				l.nonempty[l.keep(outs[0])] = was || src
				if len(outs) == 2 {
					l.nonempty[l.keep(outs[1])] = src && !was
				}
			}
			return nil
		},
		Reduce: func(decl ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
			kind := l.logDecl("reduce", decl, args)
			var result ast.ASTHandle
			var err error
			switch kind {
			case ast.DatalogOpIsEmpty:
				if l.nonempty[args[0]] {
					result, err = l.m.MkFalse()
				} else {
					result, err = l.m.MkTrue()
				}
			case ast.DatalogOpSelect:
				if l.nonempty[args[0]] {
					result, err = l.m.MkTrue()
				} else {
					result, err = l.m.MkFalse()
				}
			case ast.DatalogOpJoin, ast.DatalogOpProject, ast.DatalogOpRename, ast.DatalogOpComplement:
				name, nerr := l.m.Symbols.Intern(fmt.Sprintf("X%d", l.fresh))
				astTestMust(l.t, nerr)
				l.fresh++
				rng, rerr := l.m.GetRange(decl)
				astTestMust(l.t, rerr)
				result, err = l.m.MkConst(name, rng)
				astTestMust(l.t, err)
				v := kind == ast.DatalogOpComplement || l.nonempty[args[0]]
				if kind == ast.DatalogOpJoin {
					v = v && l.nonempty[args[1]]
				}
				l.nonempty[l.keep(result)] = v
			}
			astTestMust(l.t, err)
			if result == (ast.ASTHandle{}) {
				fmt.Fprintf(&l.out, " => null\n")
			} else {
				l.keep(result)
				fmt.Fprintf(&l.out, " => %s\n", l.str(result))
			}
			return result, nil
		},
	}
}

func (l *externalPluginLog) close() {
	for i := range l.refs {
		astTestMust(l.t, l.m.DelRef(&l.refs[i]))
	}
}

// runExternalPluginMode mirrors the reference program's modes:
// 0 transitive closure over external relations (bit-vector columns),
// 1 Int predicates represented by product(external_relation,
// interval_relation) with API facts feeding a bit-vector head,
// 2 as 0 with a negated predicate, 3 as 0 with
// datalog.default_relation=external_relation and no representation,
// 4 as 1 with a join of two product relations, 5 filter_identical and
// filter_equal (select) on external relations.
func runExternalPluginMode(t *testing.T, mode int) string {
	t.Helper()
	c, err := NewSMTContext()
	astTestMust(t, err)
	defer func() { astTestMust(t, c.Close()) }()
	m, err := c.Manager()
	astTestMust(t, err)
	l := &externalPluginLog{t: t, c: c, m: m, nonempty: make(map[ast.ASTHandle]bool)}
	defer l.close()
	var pins []ast.ASTRef
	defer func() {
		for i := range pins {
			astTestMust(t, m.DelRef(&pins[i]))
		}
	}()
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		astTestMust(t, err)
		ref, err := m.NewRef(h)
		astTestMust(t, err)
		pins = append(pins, ref)
		return h
	}
	astTestMust(t, c.SetFixedpointEngine("datalog"))
	if mode == 3 || mode >= 6 {
		astTestMust(t, c.SetFixedpointSymbol("fixedpoint.datalog.default_relation", "external_relation"))
	}
	astTestMust(t, c.SetFixedpointExternalRelationCallbacks(l.callbacks()))
	errCode := func(err error) int {
		if err != nil {
			return 12
		}
		return 0
	}
	repr := func(d ast.ASTHandle, product bool) {
		kinds := []string{"external_relation"}
		if product {
			kinds = append(kinds, "interval_relation")
		}
		fmt.Fprintf(&l.out, "repr err %d\n", errCode(c.SetFixedpointPredicateRepresentation(d, kinds...)))
	}
	var factErr error
	fact := func(d ast.ASTHandle, values ...uint32) {
		if err := c.AddFixedpointTableFact(d, values); err != nil && factErr == nil {
			factErr = err
		}
	}
	rule := func(body, head ast.ASTHandle, sorts []ast.ASTHandle, name string) {
		f := pin(c.Implies(body, head))
		if len(sorts) > 0 {
			names := make([]string, len(sorts))
			for i := range names {
				names[i] = fmt.Sprintf("v%d", i)
			}
			f = pin(c.Forall(sorts, names, f))
		}
		fmt.Fprintf(&l.out, "rule %s err %d\n", name, errCode(c.AddFixedpointRule(f, name)))
	}
	query := func(d ast.ASTHandle) {
		status, err := c.QueryFixedpointRelations([]ast.ASTHandle{d})
		code := 0
		if err != nil {
			status, code = lbool.Undef, 12
		}
		fmt.Fprintf(&l.out, "result %d err %d\n", int(status), code)
		if err != nil {
			fmt.Fprintf(&l.out, "msg %s\n", err.Error())
		}
		if status == lbool.Undef && err == nil {
			reason, rerr := c.FixedpointReasonUnknown()
			astTestMust(t, rerr)
			fmt.Fprintf(&l.out, "reason %s\n", reason)
		} else if err == nil {
			answer, aerr := c.FixedpointAnswer(d)
			astTestMust(t, aerr)
			h, aerr := m.RefGet(answer)
			astTestMust(t, aerr)
			fmt.Fprintf(&l.out, "answer %s\n", l.str(h))
			astTestMust(t, m.DelRef(&answer))
		}
	}
	boolean := pin(m.MkBoolSort())
	_ = boolean
	bv := pin(c.BitVectorSort(2))
	integer := pin(c.IntegerSort())
	app := func(d ast.ASTHandle, args ...ast.ASTHandle) ast.ASTHandle { return pin(m.MkApp(d, args)) }
	// expressionQueries is the program's Z3_fixedpoint_query loop.
	expressionQueries := func(queries ...ast.ASTHandle) {
		for _, q := range queries {
			status, err := c.command.QueryFixedpointExpression(q)
			code := 0
			if err != nil {
				status, code = lbool.Undef, 12
			}
			fmt.Fprintf(&l.out, "result %d err %d\n", int(status), code)
			if err == nil && status != lbool.Undef {
				answer, aerr := c.command.FixedpointExpressionAnswer()
				astTestMust(t, aerr)
				h, aerr := m.RefGet(answer)
				astTestMust(t, aerr)
				fmt.Fprintf(&l.out, "answer %s\n", l.str(h))
				astTestMust(t, m.DelRef(&answer))
			}
		}
	}
	switch mode {
	case 8:
		e := pin(c.DeclareRelation("e", []ast.ASTHandle{bv, bv}))
		p := pin(c.DeclareRelation("p", []ast.ASTHandle{bv}))
		r := pin(c.DeclareRelation("r", []ast.ASTHandle{bv}))
		fact(e, 0, 1)
		fact(e, 1, 2)
		fmt.Fprintf(&l.out, "facts err %d\n", errCode(factErr))
		x, y := pin(c.BoundVariable(1, bv)), pin(c.BoundVariable(0, bv))
		rule(app(e, x, y), app(p, y), []ast.ASTHandle{bv, bv}, "succ")
		query(p)
		xb := pin(c.BoundVariable(0, bv))
		zero, one := pin(c.Numeral("0", bv)), pin(c.Numeral("1", bv))
		expressionQueries(pin(c.Exists([]ast.ASTHandle{bv}, []string{"x"}, app(p, xb))), app(p, pin(c.Numeral("2", bv))))
		factErr = nil
		fact(e, 2, 3)
		fmt.Fprintf(&l.out, "fact err %d\n", errCode(factErr))
		rule(app(p, xb), app(r, xb), []ast.ASTHandle{bv}, "copy")
		expressionQueries(pin(c.Exists([]ast.ASTHandle{bv}, []string{"x"}, app(r, xb))), app(e, zero, one), app(p, zero))
	case 6, 7:
		p := pin(c.DeclareRelation("p", []ast.ASTHandle{boolean}))
		other := pin(c.DeclareRelation("other", []ast.ASTHandle{boolean}))
		kp, ko := "tr_hashtable", "tr_sparse"
		if mode == 7 {
			kp, ko = ko, kp
		}
		astTestMust(t, c.SetFixedpointPredicateRepresentation(p, kp))
		astTestMust(t, c.SetFixedpointPredicateRepresentation(other, ko))
		fact(other, 1)
		fact(p, 1)
		fmt.Fprintf(&l.out, "facts err %d\n", errCode(factErr))
		x := pin(c.BoundVariable(0, boolean))
		q1 := pin(c.Exists([]ast.ASTHandle{boolean}, []string{"x"}, app(p, x)))
		q2 := pin(c.Exists([]ast.ASTHandle{boolean}, []string{"x"}, pin(c.And(app(p, x), app(other, x)))))
		expressionQueries(q1, q2, app(p, pin(m.MkTrue())))
	case 5:
		e := pin(c.DeclareRelation("e", []ast.ASTHandle{bv, bv}))
		q := pin(c.DeclareRelation("q", []ast.ASTHandle{bv}))
		r := pin(c.DeclareRelation("r", []ast.ASTHandle{bv}))
		repr(e, false)
		repr(q, false)
		repr(r, false)
		fact(e, 0, 0)
		fact(e, 0, 1)
		fact(e, 2, 2)
		fmt.Fprintf(&l.out, "facts err %d\n", errCode(factErr))
		x := pin(c.BoundVariable(0, bv))
		rule(app(e, x, x), app(q, x), []ast.ASTHandle{bv}, "diag")
		rule(app(e, pin(c.Numeral("0", bv)), x), app(r, x), []ast.ASTHandle{bv}, "row")
		query(q)
		query(r)
	case 0, 2, 3:
		e := pin(c.DeclareRelation("e", []ast.ASTHandle{bv, bv}))
		p := pin(c.DeclareRelation("p", []ast.ASTHandle{bv, bv}))
		cc := pin(c.DeclareRelation("c", []ast.ASTHandle{bv}))
		if mode != 3 {
			repr(e, false)
			repr(p, false)
			repr(cc, false)
		}
		fact(e, 0, 1)
		fact(e, 1, 2)
		fmt.Fprintf(&l.out, "facts err %d\n", errCode(factErr))
		x, y, z := pin(c.BoundVariable(2, bv)), pin(c.BoundVariable(1, bv)), pin(c.BoundVariable(0, bv))
		vvv := []ast.ASTHandle{bv, bv, bv}
		rule(app(e, x, y), app(p, x, y), vvv, "base")
		rule(pin(c.And(app(p, x, y), app(e, y, z))), app(p, x, z), vvv, "step")
		if mode == 2 {
			rule(pin(c.And(app(e, x, y), pin(c.Not(app(p, y, x))))), app(cc, x), vvv, "neg")
			query(cc)
		} else {
			query(p)
		}
	case 1, 4:
		a := pin(c.DeclareRelation("a", []ast.ASTHandle{integer}))
		b := pin(c.DeclareRelation("b", []ast.ASTHandle{bv}))
		d := pin(c.DeclareRelation("d", []ast.ASTHandle{integer, integer}))
		repr(a, true)
		repr(b, false)
		repr(d, true)
		fact(a, 3)
		fact(a, 5)
		fact(d, 3, 4)
		fact(d, 5, 1)
		fmt.Fprintf(&l.out, "facts err %d\n", errCode(factErr))
		x, z, xi := pin(c.BoundVariable(1, integer)), pin(c.BoundVariable(0, integer)), pin(c.BoundVariable(0, integer))
		one, two := pin(c.Numeral("1", bv)), pin(c.Numeral("2", bv))
		num := func(v int64) ast.ASTHandle { return pin(c.IntNumeral(v, integer)) }
		if mode == 1 {
			rule(pin(c.And(app(a, xi), pin(c.ArithmeticGT(xi, num(4))))), app(b, one), []ast.ASTHandle{integer}, "hi")
			rule(pin(c.And(app(a, xi), pin(c.ArithmeticLT(xi, num(3))))), app(b, two), []ast.ASTHandle{integer}, "lo")
		} else {
			ii := []ast.ASTHandle{integer, integer}
			rule(pin(c.And(app(a, x), app(d, x, z), pin(c.ArithmeticGT(z, x)))), app(b, one), ii, "up")
			rule(pin(c.And(app(a, x), app(d, x, z), pin(c.ArithmeticLT(z, x)))), app(b, two), ii, "down")
		}
		query(b)
		query(a)
		query(d)
	}
	return l.out.String()
}

// TestAPIFixedpointExternalRelationReusedContext is the reference program's
// mode 8: external_relation is the favourite relation plugin and three
// expression queries run on one context. Each query's head predicate is
// registered after the previous evaluation; as in z3, its relation is made
// (with the external "empty" callback) only when the query's evaluation
// needs it.
func TestAPIFixedpointExternalRelationReusedContext(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("test_vectors", "external_relation", "mode_8.txt"))
	astTestMust(t, err)
	if got := runExternalPluginMode(t, 8); got != string(want) {
		t.Errorf("mode 8:\n%s\nwant\n%s", got, want)
	}
}

// TestAPIFixedpointExternalRelationPlugin: AUDIT.md FUZZ-F5 (relation
// plugins). external_relation_plugin runs in the relation-plugin framework
// as Z3_fixedpoint_init registers it: SetFixedpointExternalRelationCallbacks
// is fixedpoint_context::set_state plus the reduce callbacks, and
// SetFixedpointPredicateRepresentation(external_relation[,
// interval_relation]) gives the predicate an external relation or a
// product_relation over one. The expected logs are the fixed fork's C API
// reference program's output (every reduce/reduce_assign call with its
// operator, parameters, arguments and outputs, the relation names T!n, the
// answers, and z3's error when a product relation's to_formula conjoins the
// Table-sorted external term).
func TestAPIFixedpointExternalRelationPlugin(t *testing.T) {
	for mode := 0; mode < len(externalPluginExpected); mode++ {
		got := runExternalPluginMode(t, mode)
		if got != externalPluginExpected[mode] {
			t.Errorf("mode %d:\n%s\nwant\n%s", mode, got, externalPluginExpected[mode])
		}
	}
}

var externalPluginExpected = []string{
	0: `repr err 0
repr err 0
repr err 0
assign empty <Table> | -> T!0
assign store | T!0 #b00 #b01 -> T!0
assign store | T!0 #b01 #b10 -> T!0
facts err 0
rule base err 0
rule step err 0
assign empty <Table> | -> T!1
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
assign clone | T!0 -> T!2
assign empty <Table> | -> T!3
assign union | T!3 T!2 -> T!3
assign clone | T!3 -> T!4
reduce is_empty | T!4 => false
reduce join 0 1 | T!2 T!4 => X0
reduce project 0 3 | X0 => X1
reduce is_empty | X1 => false
reduce rename 0 1 | X1 => X2
assign empty <Table> | -> T!5
assign union | T!3 X2 -> T!3 T!5
reduce is_empty | T!5 => true
reduce is_empty | T!3 => false
result 1 err 0
answer T!3
`,
	1: `repr err 0
repr err 0
repr err 0
assign empty <Table> | -> T!0
assign store | T!0 3 -> T!0
assign store | T!0 5 -> T!0
assign empty <Table> | -> T!1
assign store | T!1 3 4 -> T!1
assign store | T!1 5 1 -> T!1
facts err 0
rule hi err 0
rule lo err 0
assign empty <Table> | -> T!2
reduce is_empty | T!2 => true
reduce is_empty | T!0 => false
reduce is_empty | T!0 => false
reduce is_empty | T!2 => true
reduce is_empty | T!1 => false
reduce is_empty | T!0 => false
reduce is_empty | T!2 => true
reduce is_empty | T!1 => false
reduce is_empty | T!2 => true
reduce is_empty | T!0 => false
assign clone | T!0 -> T!3
assign clone | T!3 -> T!4
assign filter [(not (<= (:var 0) 4))] | T!4 -> T!4
reduce project 0 | T!4 => X0
reduce is_empty | X0 => false
assign empty <Table> | -> T!5
assign store | T!5 #b01 -> T!5
reduce join | X0 T!5 => X1
reduce is_empty | X1 => false
assign empty <Table> | -> T!6
assign union | T!6 X1 -> T!6
assign clone | T!3 -> T!7
assign filter [(not (<= 3 (:var 0)))] | T!7 -> T!7
reduce project 0 | T!7 => X2
reduce is_empty | X2 => false
assign empty <Table> | -> T!8
assign store | T!8 #b10 -> T!8
reduce join | X2 T!8 => X3
reduce is_empty | X3 => false
assign union | T!6 X3 -> T!6
reduce is_empty | T!6 => false
result 1 err 0
answer T!6
reduce is_empty | T!6 => false
reduce is_empty | T!3 => false
reduce is_empty | T!6 => false
reduce is_empty | T!1 => false
reduce is_empty | T!3 => false
result 0 err 12
msg Sort mismatch at argument #1 for function (declare-fun and (Bool Bool) Bool) supplied sort is Table
reduce is_empty | T!6 => false
reduce is_empty | T!3 => false
reduce is_empty | T!6 => false
reduce is_empty | T!1 => false
reduce is_empty | T!1 => false
result 0 err 12
msg Sort mismatch at argument #1 for function (declare-fun and (Bool Bool) Bool) supplied sort is Table
`,
	2: `repr err 0
repr err 0
repr err 0
assign empty <Table> | -> T!0
assign store | T!0 #b00 #b01 -> T!0
assign store | T!0 #b01 #b10 -> T!0
facts err 0
rule base err 0
rule step err 0
rule neg err 0
assign empty <Table> | -> T!1
reduce is_empty | T!1 => true
assign empty <Table> | -> T!2
reduce is_empty | T!2 => true
reduce is_empty | T!0 => false
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
reduce is_empty | T!2 => true
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
reduce is_empty | T!2 => true
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
assign clone | T!0 -> T!3
reduce is_empty | T!2 => true
assign empty <Table> | -> T!4
assign union | T!4 T!3 -> T!4
assign clone | T!4 -> T!5
reduce is_empty | T!5 => false
reduce join 0 1 | T!3 T!5 => X0
reduce project 0 3 | X0 => X1
reduce is_empty | X1 => false
reduce rename 0 1 | X1 => X2
assign empty <Table> | -> T!6
assign union | T!4 X2 -> T!4 T!6
reduce is_empty | T!6 => true
assign clone | T!3 -> T!7
assign negation_filter 0 1 1 0 | T!7 T!4 -> T!7
reduce is_empty | T!7 => false
reduce project 1 | T!7 => X3
assign empty <Table> | -> T!8
assign union | T!8 X3 -> T!8
reduce is_empty | T!8 => false
result 1 err 0
answer T!8
`,
	3: `assign empty <Table> | -> T!0
assign store | T!0 #b00 #b01 -> T!0
assign store | T!0 #b01 #b10 -> T!0
facts err 0
rule base err 0
rule step err 0
assign empty <Table> | -> T!1
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
assign clone | T!0 -> T!2
assign empty <Table> | -> T!3
assign union | T!3 T!2 -> T!3
assign clone | T!3 -> T!4
reduce is_empty | T!4 => false
reduce join 0 1 | T!2 T!4 => X0
reduce project 0 3 | X0 => X1
reduce is_empty | X1 => false
reduce rename 0 1 | X1 => X2
assign empty <Table> | -> T!5
assign union | T!3 X2 -> T!3 T!5
reduce is_empty | T!5 => true
reduce is_empty | T!3 => false
result 1 err 0
answer T!3
`,
	4: `repr err 0
repr err 0
repr err 0
assign empty <Table> | -> T!0
assign store | T!0 3 -> T!0
assign store | T!0 5 -> T!0
assign empty <Table> | -> T!1
assign store | T!1 3 4 -> T!1
assign store | T!1 5 1 -> T!1
facts err 0
rule up err 0
rule down err 0
reduce is_empty | T!0 => false
assign empty <Table> | -> T!2
reduce is_empty | T!2 => true
reduce is_empty | T!0 => false
reduce is_empty | T!0 => false
reduce is_empty | T!2 => true
reduce is_empty | T!1 => false
reduce is_empty | T!0 => false
reduce is_empty | T!2 => true
reduce is_empty | T!1 => false
reduce is_empty | T!2 => true
reduce is_empty | T!0 => false
assign clone | T!0 -> T!3
reduce is_empty | T!1 => false
assign clone | T!1 -> T!4
reduce join 0 0 | T!3 T!4 => X0
reduce project 0 | X0 => X1
reduce is_empty | X1 => false
assign clone | X1 -> T!5
assign filter [(not (<= (:var 1) (:var 0)))] | T!5 -> T!5
reduce project 0 1 | T!5 => X2
reduce is_empty | X2 => false
assign empty <Table> | -> T!6
assign store | T!6 #b01 -> T!6
reduce join | X2 T!6 => X3
reduce is_empty | X3 => false
assign empty <Table> | -> T!7
assign union | T!7 X3 -> T!7
reduce join 0 0 | T!3 T!4 => X4
reduce project 0 | X4 => X5
reduce is_empty | X5 => false
assign clone | X5 -> T!8
assign filter [(not (<= (:var 0) (:var 1)))] | T!8 -> T!8
reduce project 0 1 | T!8 => X6
reduce is_empty | X6 => false
assign empty <Table> | -> T!9
assign store | T!9 #b10 -> T!9
reduce join | X6 T!9 => X7
reduce is_empty | X7 => false
assign union | T!7 X7 -> T!7
reduce is_empty | T!7 => false
result 1 err 0
answer T!7
reduce is_empty | T!3 => false
reduce is_empty | T!7 => false
reduce is_empty | T!3 => false
reduce is_empty | T!7 => false
reduce is_empty | T!4 => false
reduce is_empty | T!3 => false
result 0 err 12
msg Sort mismatch at argument #1 for function (declare-fun and (Bool Bool) Bool) supplied sort is Table
reduce is_empty | T!3 => false
reduce is_empty | T!7 => false
reduce is_empty | T!3 => false
reduce is_empty | T!7 => false
reduce is_empty | T!4 => false
reduce is_empty | T!4 => false
result 0 err 12
msg Sort mismatch at argument #1 for function (declare-fun and (Bool Bool) Bool) supplied sort is Table
`,
	5: `repr err 0
repr err 0
repr err 0
assign empty <Table> | -> T!0
assign store | T!0 #b00 #b00 -> T!0
assign store | T!0 #b00 #b01 -> T!0
assign store | T!0 #b10 #b10 -> T!0
facts err 0
rule diag err 0
rule row err 0
assign empty <Table> | -> T!1
reduce is_empty | T!1 => true
assign empty <Table> | -> T!2
reduce is_empty | T!2 => true
reduce is_empty | T!0 => false
reduce is_empty | T!0 => false
reduce is_empty | T!2 => true
reduce is_empty | T!1 => true
reduce is_empty | T!0 => false
reduce is_empty | T!2 => true
reduce is_empty | T!1 => true
assign empty <Table> | -> T!4
reduce is_empty | T!4 => true
reduce is_empty | T!0 => false
assign clone | T!0 -> T!5
reduce is_empty | T!2 => true
assign clone | T!5 -> T!6
assign filter [(= (:var 0) (:var 1))] | T!6 -> T!6
reduce is_empty | T!6 => false
reduce project 0 | T!6 => X0
assign empty <Table> | -> T!7
assign union | T!7 X0 -> T!7
assign empty <Table> | -> T!8
assign union | T!8 T!7 -> T!8
reduce is_empty | T!8 => false
result 1 err 0
answer T!8
reduce is_empty | T!1 => true
reduce is_empty | T!8 => false
reduce is_empty | T!5 => false
reduce is_empty | T!5 => false
reduce is_empty | T!8 => false
reduce is_empty | T!1 => true
reduce is_empty | T!5 => false
reduce is_empty | T!8 => false
reduce is_empty | T!1 => true
assign empty <Table> | -> T!10
reduce is_empty | T!10 => true
reduce is_empty | T!5 => false
assign clone | T!5 -> T!11
reduce is_empty | T!1 => true
assign clone | T!11 -> T!12
assign filter [(= (:var 0) #b00)] | T!12 -> T!12
reduce project 0 | T!12 => X1
reduce is_empty | X1 => false
assign empty <Table> | -> T!13
assign union | T!13 X1 -> T!13
assign empty <Table> | -> T!14
assign union | T!14 T!13 -> T!14
reduce is_empty | T!14 => false
result 1 err 0
answer T!14
`,
}
