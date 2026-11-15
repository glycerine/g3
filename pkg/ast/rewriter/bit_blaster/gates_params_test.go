package bit_blaster

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// AUDIT.md ast_rewriter_06-F1: blaster's bool_rewriter m_rewriter(m) reads
// the global rewriter module (here ite_extra_rules) and then overrides only
// flat and elim_and.
func TestBitBlasterRewriterHonorsIteExtraRules(t *testing.T) {
	m := gateManager(t)
	_, err := m.RegisterArithmeticPlugin()
	paramsMust(t, err)
	family, err := m.RegisterBitVectorPlugin()
	paramsMust(t, err)
	context, err := uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget(), Text: arena.VectorConfig{Descriptors: arena.Config{Budget: m.Budget()}}})
	paramsMust(t, err)
	t.Cleanup(func() { paramsMust(t, context.Close()) })
	catalog, err := uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	paramsMust(t, err)
	t.Cleanup(func() { paramsMust(t, catalog.Close()) })
	globals, err := uparams.NewGlobalParamsService(m.Symbols, context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: catalog})
	paramsMust(t, err)
	t.Cleanup(func() { paramsMust(t, globals.Close()) })
	paramsMust(t, globals.InitializeSource())
	paramsMust(t, globals.SourceSetName("rewriter.ite_extra_rules", "true"))

	b, err := m.MkBoolSort()
	paramsMust(t, err)
	var atoms [4]ast.ASTHandle
	for i := range atoms {
		atoms[i], err = m.MkConst(usymbol.Numerical(uint32(711+i)), b)
		paramsMust(t, err)
		gatePin(t, m, atoms[i])
	}
	c1, c2, t1, t2 := atoms[0], atoms[1], atoms[2], atoms[3]
	inner, err := m.MkIte(c2, t1, t2)
	paramsMust(t, err)
	gatePin(t, m, inner)
	out, err := m.NewRef(ast.ASTHandle{})
	paramsMust(t, err)
	defer func() { paramsMust(t, m.DelRef(&out)) }()

	plain, err := newRewriterConfig(m, family, nil, nil)
	paramsMust(t, err)
	paramsMust(t, plain.simplify(ast.ASTOpIte, out, c1, t1, inner))
	got, err := m.RefGet(out)
	paramsMust(t, err)
	if cond, e := m.GetArg(got, 0); e != nil || cond != c1 {
		t.Fatal("default blaster applied ite_extra_rules", got, e)
	}

	configured, err := newRewriterConfig(m, family, context, globals)
	paramsMust(t, err)
	if configured.boolRewriter.Flat() || !configured.boolRewriter.ElimAnd() || !configured.boolRewriter.IteExtraRules() {
		t.Fatal("blaster bool_rewriter configuration")
	}
	paramsMust(t, configured.simplify(ast.ASTOpIte, out, c1, t1, inner))
	got, err = m.RefGet(out)
	paramsMust(t, err)
	or, err := m.MkOr(c1, c2)
	paramsMust(t, err)
	gatePin(t, m, or)
	want, err := m.MkIte(or, t1, t2)
	paramsMust(t, err)
	gatePin(t, m, want)
	if got != want {
		t.Fatal("(ite c1 t1 (ite c2 t1 t2)) was not merged into (ite (or c1 c2) t1 t2)", got, want)
	}
}
