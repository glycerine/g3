package pattern

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPatternCollectorSparseBinders(t *testing.T) {
	m := patternManager(t)
	pin := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		patternMust(t, err)
		patternMust(t, m.IncRef(h))
		t.Cleanup(func() { patternMust(t, m.DecRef(h)) })
		return h
	}
	b := pin(m.MkBoolSort())
	f := pin(m.MkFuncDecl(symbol.Numerical(710), []ast.ASTHandle{b, b}, b, ast.ASTFuncDeclInfo{}))
	x := pin(m.MkVar(63, b))
	y := pin(m.MkVar(64, b))
	z := pin(m.MkVar(1023, b))
	xy := pin(m.MkApp(f, []ast.ASTHandle{x, y}))
	xyz := pin(m.MkApp(f, []ast.ASTHandle{xy, z}))
	c, err := newPatternCandidates(m, 1024, nil, false)
	patternMust(t, err)
	t.Cleanup(func() { patternMust(t, c.close()) })
	patternMust(t, c.collect(xyz))
	pair, triple := c.info[xy].freeVars, c.info[xyz].freeVars
	if pair.count() != 2 || triple.count() != 3 || !pair.subset(triple) || triple.subset(pair) || sameFreeVars(pair, triple) {
		t.Fatalf("unexpected candidate sets %v and %v", pair, triple)
	}
	if _, ok := pair.high[64]; !ok || pair.low != uint64(1)<<63 {
		t.Fatal(pair)
	}
	if _, ok := triple.high[1023]; !ok {
		t.Fatal(triple)
	}
}
