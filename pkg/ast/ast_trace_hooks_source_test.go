package ast

// Source: src/ast/ast.cpp:2114-2128,2259-2261,2368-2373;
// src/util/symbol.h:97-110.

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/arena"

	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestASTTraceFactoriesOnlyActualNewNodesAndLiteralArgumentChains(t *testing.T) {
	sy, q, _ := astTestOwners(t)
	b := arena.NewBudget(1 << 28)
	name := astTracePath(t)
	filename := uparams.LiteralParamsStringView(name)
	m, e := NewSourceASTManager(ASTManagerConfig{Symbols: sy, Numerals: q, Budget: b}, &filename)
	paramsMust(t, e)
	defer m.Close()
	paramsMust(t, m.TraceStream.Flush())
	prefix, e := os.ReadFile(name)
	paramsMust(t, e)
	paramsMust(t, m.TraceStream.SetShowPos(true))
	bs, e := m.MkBoolSort()
	paramsMust(t, e)
	v, e := m.MkVar(0, bs)
	paramsMust(t, e)
	paramsMust(t, m.IncRef(v))
	defer func() { paramsMust(t, m.DecRef(v)) }()
	again, e := m.MkVar(0, bs)
	paramsMust(t, e)
	if again != v {
		t.Fatal("source interned var")
	}
	d, e := m.MkFuncDecl(usymbol.Numerical(17), []ASTHandle{bs}, bs, ASTFuncDeclInfo{})
	paramsMust(t, e)
	app, e := m.MkApp(d, []ASTHandle{v})
	paramsMust(t, e)
	paramsMust(t, m.IncRef(app))
	defer func() { paramsMust(t, m.DecRef(app)) }()
	again, e = m.MkApp(d, []ASTHandle{v})
	paramsMust(t, e)
	if again != app {
		t.Fatal("source interned app")
	}
	pat, e := m.MkFamilyApp(2, 0, nil, []ASTHandle{app})
	paramsMust(t, e)
	paramsMust(t, m.IncRef(pat))
	defer func() { paramsMust(t, m.DecRef(pat)) }()
	bound, e := sy.Intern("x")
	paramsMust(t, e)
	spec := ASTQuantifierSpec{Forall: true, Sorts: []ASTHandle{bs}, Names: []usymbol.Symbol{bound}, Body: app, QID: usymbol.Numerical(23), Patterns: []ASTHandle{pat}}
	quant, e := m.MkQuantifier(spec)
	paramsMust(t, e)
	paramsMust(t, m.IncRef(quant))
	defer func() { paramsMust(t, m.DecRef(quant)) }()
	again, e = m.MkQuantifier(spec)
	paramsMust(t, e)
	if again != quant {
		t.Fatal("source interned quantifier")
	}
	id := func(h ASTHandle) string {
		v, e := m.GetASTID(h)
		paramsMust(t, e)
		return strconv.FormatUint(uint64(v), 10)
	}
	spec.Patterns = nil
	spec.NoPatterns = []ASTHandle{app}
	spec.QID = usymbol.Numerical(24)
	noPattern, e := m.MkQuantifier(spec)
	paramsMust(t, e)
	paramsMust(t, m.IncRef(noPattern))
	defer func() { paramsMust(t, m.DecRef(noPattern)) }()
	// Pattern creation also emits its actual app, separate from quantifier fields.
	expected := "[mk-var] #" + id(v) + "\n[mk-app] #" + id(app) + " k!17 #" + id(v) + "\n[mk-app] #" + id(pat) + " pattern #" + id(app) + "\n[mk-quant] #" + id(quant) + " k!23 #" + id(pat) + " #" + id(app) + "\n"
	expected += "[mk-quant] #" + id(noPattern) + " k!24 #" + id(app) + "\n"
	paramsMust(t, m.TraceStream.Flush())
	data, e := os.ReadFile(name)
	paramsMust(t, e)
	got := strings.TrimPrefix(string(data), string(prefix))
	if got != expected {
		t.Fatal("literal source-new/unsigned/symbol/pattern/body insertion sequence", got, expected)
	}
	if b.Used() != m.Stats().AllocatedBytes {
		t.Fatal("trace physical/cold owner graph", b.Used(), m.Stats())
	}
}
