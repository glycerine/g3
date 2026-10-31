package opt

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func TestObjectivePurifierPrintedDefinitionNames(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	x := decompositionIntegerVariable(t, f, "x with space")
	product, err := f.manager.MkArithmeticMul(x, x)
	if err != nil {
		t.Fatal(err)
	}
	condition, err := f.manager.MkArithmeticGT(x, decompositionInteger(t, f, 0))
	if err != nil {
		t.Fatal(err)
	}
	purifier, err := NewObjectivePurifier(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := purifier.Close(); err != nil {
			t.Error(err)
		}
	}()
	var hard []ast.ASTHandle
	for i, test := range []struct {
		term   ast.ASTHandle
		prefix string
		soft   bool
	}{
		{product, "(* |x with space| |x with space|)", false},
		{condition, "(> |x with space| 0)", true},
		{product, "(* |x with space| |x with space|)", false},
	} {
		fresh, err := purifier.definition(test.term, &hard, test.soft)
		if err != nil {
			t.Fatal(err)
		}
		decl, err := f.manager.GetDecl(fresh)
		if err != nil {
			t.Fatal(err)
		}
		name, err := f.manager.GetASTName(decl)
		if err != nil {
			t.Fatal(err)
		}
		text, err := f.symbols.String(name)
		if err != nil || text != fmt.Sprintf("%s!%d", test.prefix, i) {
			t.Fatalf("fresh name %q: %v", text, err)
		}
		left, err := f.manager.GetArg(hard[i], 0)
		if err != nil || left != fresh {
			t.Fatal("definition left side", left, err)
		}
		right, err := f.manager.GetArg(hard[i], 1)
		if err != nil || right != test.term {
			t.Fatal("definition right side", right, err)
		}
	}
	stats, err := purifier.Statistics()
	if err != nil || stats.FreshDeclarations != 3 || stats.ArithmeticDefinitions != 2 || stats.SoftDefinitions != 1 {
		t.Fatal(stats, err)
	}
}

func TestObjectivePurifierDefinitionPublishesEqualityBeforeFilter(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	purifier, err := NewObjectivePurifier(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := purifier.Close(); err != nil {
			t.Error(err)
		}
	}()
	// Make filter insertion refuse deterministically, after the equality has
	// been constructed and appended. Earlier source effects must survive.
	if err := purifier.filter.Close(); err != nil {
		t.Fatal(err)
	}
	var hard []ast.ASTHandle
	if _, err = purifier.definition(f.p, &hard, true); !errors.Is(err, tactic.ErrModelConverterOwner) {
		t.Fatal("filter insertion error", err)
	}
	if len(hard) != 1 || purifier.stats.FreshDeclarations != 0 {
		t.Fatal("publication prefix", len(hard), purifier.stats)
	}
	right, err := f.manager.GetArg(hard[0], 1)
	if err != nil || right != f.p {
		t.Fatal("published defining equality", right, err)
	}
}

func TestObjectivePurifierPrefixReleasesFormatDocuments(t *testing.T) {
	f := newObjectiveFormulaFixture(t)
	purifier, err := NewObjectivePurifier(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := purifier.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err = purifier.definitionPrefix(f.p); err != nil {
		t.Fatal(err)
	}
	formats, err := f.manager.GetFormatManager()
	if err != nil {
		t.Fatal(err)
	}
	before := formats.Stats()
	for _, term := range []ast.ASTHandle{f.q, f.p, f.q} {
		if _, err = purifier.definitionPrefix(term); err != nil {
			t.Fatal(err)
		}
	}
	after := formats.Stats()
	if before.LiveNodes != after.LiveNodes || before.LiveReferences != after.LiveReferences {
		t.Fatal("temporary format trees survived", before, after)
	}
}

func TestObjectivePurifierPrefixFailureDoesNotConsumeFreshName(t *testing.T) {
	success, failures := false, 0
	for after := int64(0); after < 128; after++ {
		f := newObjectiveFormulaFixture(t)
		purifier, err := NewObjectivePurifier(f.manager)
		if err != nil {
			t.Fatal(err)
		}
		f.budget.FailAfter(after)
		prefix, err := purifier.definitionPrefix(f.p)
		f.budget.FailAfter(-1)
		if err != nil {
			if !errors.Is(err, arena.ErrInjectedFailure) {
				t.Fatal(after, err)
			}
			failures++
		} else {
			if prefix != "codec-p" {
				t.Fatal(prefix)
			}
			success = true
		}
		// Formatting precedes mk_fresh_const: even a failed print must leave
		// the source fresh identifier sequence untouched.
		sort, e := f.manager.GetSort(f.p)
		if e != nil {
			t.Fatal(e)
		}
		probe, e := f.manager.MkFreshConst("probe", sort)
		if e != nil {
			t.Fatal(e)
		}
		ref, e := f.manager.NewRef(probe)
		if e != nil {
			t.Fatal(e)
		}
		f.refs = append(f.refs, ref)
		decl, e := f.manager.GetDecl(probe)
		if e != nil {
			t.Fatal(e)
		}
		name, e := f.manager.GetASTName(decl)
		if e != nil {
			t.Fatal(e)
		}
		text, e := f.symbols.String(name)
		if e != nil || text != "probe!0" {
			t.Fatal(after, text, e)
		}
		if e := purifier.Close(); e != nil {
			t.Fatal(e)
		}
		if success {
			break
		}
	}
	if !success || failures == 0 {
		t.Fatal("allocation frontier", failures, success)
	}
}
