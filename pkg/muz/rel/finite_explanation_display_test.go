package rel

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFiniteExplanationProductDisplayEstimatesAndEmptyCollection(t *testing.T) {
	f := newTableRelationFixture(t)
	ruleSort, err := f.ast.MkDatalogRuleSort()
	astTestMust(t, err)
	symbol, err := f.ast.Symbols.Intern("finite-explanation-display")
	astTestMust(t, err)
	explanation, err := f.ast.MkConst(symbol, ruleSort)
	astTestMust(t, err)
	p, err := NewFiniteExplanationProduct(f.relations, []ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	defer func() { astTestMust(t, p.Close()) }()
	astTestMust(t, p.AddFact([]ast.ASTHandle{f.truth}, explanation))

	// Install a valid but empty inner row to exercise source
	// garbage_collect(true), including disposal of the nested AST owner.
	empty, err := NewExplanationRelation(f.ast, []ast.ASTHandle{ruleSort})
	astTestMust(t, err)
	p.inner = append(p.inner, empty)
	astTestMust(t, p.table.EnsureFact(Fact{0, 1}))
	if rows, e := p.SizeEstimateRows(); e != nil || rows != 2 {
		t.Fatalf("outer row estimate = %d, want 2 (%v)", rows, e)
	}
	sparse, ok := p.table.(*SparseTable)
	if !ok {
		t.Fatalf("default explanation index is %T, want SparseTable", p.table)
	}
	wantBytes := uint32(len(sparse.data.entries)) * sparse.layout.EntryBytes()
	if size, e := p.SizeEstimateBytes(); e != nil || size != wantBytes {
		t.Fatalf("outer byte estimate = %d, want %d (%v)", size, wantBytes, e)
	}

	var output bytes.Buffer
	astTestMust(t, p.Display(&output))
	text := output.String()
	if !strings.Contains(text, "finite_product_relation:\n table:\n") || !strings.Contains(text, " inner relation 0:\n") || strings.Contains(text, " inner relation 1:\n") || !strings.Contains(text, "finite-explanation-display") {
		t.Fatalf("explanation product display = %q", text)
	}
	if rows, e := p.SizeEstimateRows(); e != nil || rows != 1 {
		t.Fatalf("post-display row estimate = %d, want 1 (%v)", rows, e)
	}
	if _, err = empty.Empty(); err == nil {
		t.Fatal("display did not close collected empty explanation owner")
	}

	refusal := errors.New("explanation product display refusal")
	if err = p.Display(productDisplayRefusingWriter{err: refusal}); !errors.Is(err, refusal) {
		t.Fatalf("display writer error = %v, want %v", err, refusal)
	}
	if err = p.Display(nil); !errors.Is(err, ErrTableRelation) {
		t.Fatalf("nil writer error = %v", err)
	}
}
