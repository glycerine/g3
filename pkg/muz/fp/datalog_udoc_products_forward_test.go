package fp

// Source: dl_compiler.cpp one-positive-tail selection/assembly and
// dl_product_relation.cpp componentwise filtering, projection and rename.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type publicUDocForwardFixture struct {
	*publicUDocProductFixture
	z ast.ASTHandle
}

func newPublicUDocForwardFixture(t *testing.T, kind string) *publicUDocForwardFixture {
	t.Helper()
	m, globals := newPDRLifecycleServices(t)
	engine, err := NewPropositionalEngineWithServices(m, globals.Context, globals)
	fragmentMust(t, err)
	f := &publicUDocForwardFixture{publicUDocProductFixture: &publicUDocProductFixture{t: t, m: m, e: engine}}
	t.Cleanup(func() { fragmentMust(t, engine.Close()) })
	fragmentMust(t, globals.SourceSetName("fixedpoint.datalog.default_relation", "doc"))
	fragmentMust(t, globals.SourceSetName("fixedpoint.datalog.default_table", kind))
	fragmentMust(t, engine.SelectEngine("datalog"))
	prepareUDocProductGroundRules(f.publicUDocProductFixture)
	f.boolean = f.pin(m.MkBoolSort())
	f.bits = f.pin(m.BitVectorSort(2))
	f.yes = f.pin(m.MkTrue())
	f.no = f.pin(m.MkFalse())
	f.x = f.pin(m.MkVar(0, f.bits))
	f.b = f.pin(m.MkVar(1, f.boolean))
	f.z = f.pin(m.MkVar(2, f.bits))
	f.doc = f.declare("forward-D", f.bits)
	f.finite = f.declare("forward-T", f.boolean, f.bits, f.bits)
	f.head = f.declare("forward-H", f.bits, f.boolean, f.bits)
	for value := uint64(0); value < 4; value++ {
		f.values = append(f.values, f.pin(m.BitVectorNumeralUint64(value, 2)))
	}
	return f
}

func (f *publicUDocForwardFixture) declare(name string, sorts ...ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	symbol, err := f.m.Symbols.Intern(name)
	fragmentMust(f.t, err)
	declaration := f.pin(f.m.MkFuncDecl(symbol, sorts, f.boolean, ast.ASTFuncDeclInfo{}))
	fragmentMust(f.t, f.e.RegisterPredicate(declaration))
	return declaration
}

func (f *publicUDocForwardFixture) rule(name string, body, head ast.ASTHandle) ast.ASTHandle {
	f.t.Helper()
	formula := f.pin(f.m.MkImplies(body, head))
	fragmentMust(f.t, f.e.addRuleEager(formula, name))
	return formula
}

func (f *publicUDocForwardFixture) anchor() {
	f.t.Helper()
	body := f.pin(f.m.MkAnd(f.app(f.doc, f.x), f.app(f.finite, f.b, f.x, f.z)))
	f.rule("forward-anchor", body, f.app(f.head, f.x, f.b, f.z))
}

func (f *publicUDocForwardFixture) check(declaration ast.ASTHandle, wantStatus lbool.LBool, rows ...[]uint32) {
	f.t.Helper()
	status, err := f.e.QueryPredicate(declaration)
	fragmentMust(f.t, err)
	if status != wantStatus {
		f.t.Fatal("ordinary product forwarding status", status, wantStatus)
	}
	answer, err := f.e.Answer(declaration)
	fragmentMust(f.t, err)
	defer func() { fragmentMust(f.t, f.m.DelRef(&answer)) }()
	arity, err := f.m.GetArity(declaration)
	fragmentMust(f.t, err)
	want := make(map[string]bool)
	for _, row := range rows {
		if len(row) != int(arity) {
			f.t.Fatal("forwarding oracle row arity", row, arity)
		}
		want[fmt.Sprint(row)] = true
	}
	values := make([]ast.ASTHandle, arity)
	row := make([]uint32, arity)
	var enumerate func(uint32)
	enumerate = func(column uint32) {
		if column == arity {
			f.checkFormula(answer, values, want[fmt.Sprint(row)])
			return
		}
		sort, err := f.m.GetDomain(declaration, column)
		fragmentMust(f.t, err)
		domain := f.values
		if sort == f.boolean {
			domain = []ast.ASTHandle{f.no, f.yes}
		} else if sort != f.bits {
			f.t.Fatal("unexpected forwarding oracle sort")
		}
		for index, value := range domain {
			values[column], row[column] = value, uint32(index)
			enumerate(column + 1)
		}
	}
	enumerate(0)
}

func (f *publicUDocForwardFixture) checkProduct(kind string, declarations ...ast.ASTHandle) {
	f.t.Helper()
	c := f.e.udocDatalog
	if c == nil || c.products == nil {
		f.t.Fatal("ordinary forwarding used finite fallback")
	}
	for _, declaration := range declarations {
		product := c.products.result[declaration]
		if product == nil || udocProductFiniteKind(product.Table()) != kind {
			f.t.Fatal("ordinary forwarding lost actual product or concrete table", declaration)
		}
	}
}

func TestDatalogUDocProductsOrdinaryForwardChainProjectionAndOwnership(t *testing.T) {
	for _, kind := range []string{"hashtable", "sparse", "lazy_sparse"} {
		t.Run(kind, func(t *testing.T) {
			f := newPublicUDocForwardFixture(t, kind)
			identity := f.declare("forward-I", f.bits, f.boolean, f.bits)
			permuted := f.declare("forward-S", f.boolean, f.bits, f.bits)
			projected := f.declare("forward-R", f.bits, f.boolean)
			// Every consumer precedes its source in the submitted rule order.
			projectRule := f.rule("forward-project", f.app(permuted, f.b, f.z, f.x), f.app(projected, f.z, f.b))
			f.rule("forward-permute", f.app(identity, f.x, f.b, f.z), f.app(permuted, f.b, f.z, f.x))
			f.rule("forward-identity", f.app(f.head, f.x, f.b, f.z), f.app(identity, f.x, f.b, f.z))
			f.anchor()
			fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{1}))
			fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, 2, 3}))
			// DOC constrains the common key to 1, while the actual table
			// constrains it to 2. Identity/permutation keep the false formula
			// with the source's coarse true status; component projection forgets
			// the conflicting key and produces the concrete tuple (3, true).
			for _, declaration := range []ast.ASTHandle{f.head, identity, permuted} {
				f.check(declaration, lbool.True)
				f.checkProduct(kind, declaration)
			}
			f.check(projected, lbool.True, []uint32{3, 1})
			f.checkProduct(kind, f.head, identity, permuted, projected)
			owner := f.e.udocDatalog
			if owner.products.result[identity] == owner.products.result[f.head] || owner.products.result[permuted].Table() == owner.products.result[identity].Table() {
				t.Fatal("ordinary forwarding aliased source product storage")
			}
			status, err := f.e.QueryExpression(f.app(projected, f.values[3], f.yes))
			fragmentMust(t, err)
			if status != lbool.True || f.e.queryClone == nil || f.e.queryClone.udocDatalog == nil || f.e.queryClone.udocDatalog.products.result[projected] == owner.products.result[projected] {
				t.Fatal("ordinary forwarding expression query lost independent product owner", status)
			}
			answer, err := f.e.ExpressionAnswer()
			fragmentMust(t, err)
			fragmentMust(t, f.e.clearQueryResources())
			f.checkFormula(answer, nil, true)
			fragmentMust(t, f.m.DelRef(&answer))
			fragmentMust(t, f.e.UpdateRule(projectRule, "forward-project"))
			f.check(projected, lbool.True, []uint32{3, 1})
			fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{0, 2, 0}))
			f.check(projected, lbool.True, []uint32{3, 1}, []uint32{0, 0})
			f.check(f.head, lbool.True)
			if count, err := f.e.predicates[f.finite].table.Len(); err != nil || count != 2 {
				t.Fatal("forwarding queries mutated finite source facts", count, err)
			}
		})
	}
}

func TestDatalogUDocProductsOrdinaryForwardRepeatedVariablesAndConstants(t *testing.T) {
	f := newPublicUDocForwardFixture(t, "hashtable")
	repeated := f.declare("forward-repeated", f.bits, f.boolean)
	constant := f.declare("forward-constant", f.bits, f.boolean)
	projected := f.declare("forward-projected", f.bits, f.boolean)
	f.rule("forward-repeat-rule", f.app(f.head, f.x, f.b, f.x), f.app(repeated, f.x, f.b))
	f.rule("forward-constant-rule", f.app(f.head, f.x, f.b, f.values[2]), f.app(constant, f.x, f.b))
	f.rule("forward-projected-rule", f.app(f.head, f.x, f.b, f.z), f.app(projected, f.z, f.b))
	f.anchor()
	fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{1}))
	fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, 1, 1}))
	fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{0, 1, 2}))
	f.check(repeated, lbool.True, []uint32{1, 1})
	f.check(constant, lbool.True, []uint32{1, 0})
	f.check(projected, lbool.True, []uint32{1, 1}, []uint32{2, 0})
	f.check(f.head, lbool.True, []uint32{1, 1, 1}, []uint32{1, 0, 2})
	f.checkProduct("hashtable", f.head, repeated, constant, projected)
}

func TestDatalogUDocProductsOrdinaryForwardUnsupportedProgramsKeepFiniteFallback(t *testing.T) {
	for _, boundary := range []string{"seeded-target", "cycle", "multiple-definitions", "bool-only-target"} {
		t.Run(boundary, func(t *testing.T) {
			f := newPublicUDocForwardFixture(t, "hashtable")
			var target ast.ASTHandle
			wantStatus := lbool.True
			var rows [][]uint32
			switch boundary {
			case "seeded-target":
				target = f.declare("forward-seeded", f.bits, f.boolean)
				f.rule("forward-seeded-rule", f.app(f.head, f.x, f.b, f.z), f.app(target, f.z, f.b))
				fragmentMust(t, f.e.AddTableFact(target, []uint32{0, 0}))
				rows = [][]uint32{{0, 0}}
			case "cycle":
				target = f.declare("forward-cycle-R", f.bits, f.boolean)
				other := f.declare("forward-cycle-S", f.bits, f.boolean)
				f.rule("forward-cycle-first", f.app(other, f.x, f.b), f.app(target, f.x, f.b))
				f.rule("forward-cycle-second", f.app(target, f.x, f.b), f.app(other, f.x, f.b))
				wantStatus = lbool.False
			case "multiple-definitions":
				target = f.declare("forward-multiple", f.bits, f.boolean)
				for _, name := range []string{"forward-first", "forward-second"} {
					f.rule(name, f.app(f.head, f.x, f.b, f.z), f.app(target, f.z, f.b))
				}
				rows = [][]uint32{{3, 1}}
			case "bool-only-target":
				target = f.declare("forward-bool", f.boolean)
				f.rule("forward-bool-rule", f.app(f.head, f.x, f.b, f.z), f.app(target, f.b))
				rows = [][]uint32{{1}}
			}
			f.anchor()
			fragmentMust(t, f.e.AddTableFact(f.doc, []uint32{1}))
			key := uint32(1)
			if boundary == "seeded-target" {
				key = 2 // Exact finite fallback derives no extra projected row.
			}
			fragmentMust(t, f.e.AddTableFact(f.finite, []uint32{1, key, 3}))
			f.check(target, wantStatus, rows...)
			if f.e.udocDatalog != nil {
				t.Fatal("unsupported ordinary forwarding program abandoned finite fallback")
			}
		})
	}
}
