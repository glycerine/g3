package duality

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func dualityContextFixture(t *testing.T) (*Context, *Config, *ast.ASTManager) {
	t.Helper()
	p, budget := dualityParamsFixture(t)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{
		Symbols: p.SymbolsField, Numerals: p.NumeralsField, Budget: budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	config, err := NewConfig(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	closeConfig(t, config)
	context, err := NewContext(m, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !context.closed {
			if err := context.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	return context, config, m
}

func TestContextFamilyOrderAndConfigCopy(t *testing.T) {
	p, budget := dualityParamsFixture(t)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{
		Symbols: p.SymbolsField, Numerals: p.NumeralsField, Budget: budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	config, err := NewConfig(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	closeConfig(t, config)
	if err = config.SetBool("mbqi", true); err != nil {
		t.Fatal(err)
	}
	context, err := NewContext(m, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !context.closed {
			if err := context.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	families, err := context.Families()
	if err != nil {
		t.Fatal(err)
	}
	want := ContextFamilies{Basic: 0, Arithmetic: 5, BitVector: 6, Array: 7, Datatype: 8, Datalog: 9}
	if families != want {
		t.Fatal("source family registration order", families, want)
	}
	for name, id := range map[string]ast.FamilyID{
		"arith": 5, "bv": 6, "array": 7, "datatype": 8, "datalog_relation": 9,
	} {
		symbol, err := m.Symbols.Intern(name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := m.FamilyIDForSymbol(symbol)
		if err != nil || got != id {
			t.Fatal("family", name, got, err)
		}
	}
	// Mutating the caller's config must detach from the context's copy.
	if err = config.SetBool("mbqi", false); err != nil {
		t.Fatal(err)
	}
	owned, err := context.Config()
	if err != nil {
		t.Fatal(err)
	}
	ownedRef, err := owned.Ref()
	if err != nil {
		t.Fatal(err)
	}
	name := "mbqi"
	if got, err := p.GetBoolName(ownedRef, &name, false); err != nil || !got {
		t.Fatal("context config was not copy-constructed", got, err)
	}
	if err = context.SetInt("limit", -1); err != nil {
		t.Fatal(err)
	}
	name = "limit"
	if got, err := p.GetUIntName(ownedRef, &name, 0); err != nil || got != ^uint32(0) {
		t.Fatal("context setter", got, err)
	}
}

func TestContextConstructorFailureReleasesConfigCopy(t *testing.T) {
	p, budget := dualityParamsFixture(t)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{
		Symbols: p.SymbolsField, Numerals: p.NumeralsField, Budget: budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	config, err := NewConfig(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	closeConfig(t, config)
	if err = config.SetBool("live", true); err != nil {
		t.Fatal(err)
	}
	before := p.Stats()
	if before.References != 1 {
		t.Fatal("fixture reference count", before)
	}
	if err = m.Close(); err != nil {
		t.Fatal(err)
	}
	if context, err := NewContext(m, config); context != nil || err == nil {
		t.Fatal("closed borrowed manager accepted", context, err)
	}
	after := p.Stats()
	if after.References != before.References || after.LiveBlocks != before.LiveBlocks || after.Entries != before.Entries {
		t.Fatal("failed context retained config copy", before, after)
	}
}

func TestContextBorrowsManagerAndRejectsShallowCopy(t *testing.T) {
	context, _, manager := dualityContextFixture(t)
	copyContext := *context
	if _, err := copyContext.Families(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("shallow context copy", err)
	}
	if err := context.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := context.Manager(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("closed context", err)
	}
	if err := context.Close(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("double context close", err)
	}
	// Context destruction does not adopt the borrowed manager.
	if sort, err := manager.MkBoolSort(); err != nil || sort.IsNull() {
		t.Fatal("borrowed manager closed", sort, err)
	}
}

func TestObjectCopyAndContextChecks(t *testing.T) {
	context, config, manager := dualityContextFixture(t)
	a, err := NewObject(context)
	if err != nil {
		t.Fatal(err)
	}
	b, err := a.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckContext(a, b); err != nil {
		t.Fatal(err)
	}
	copyObject := *a
	if _, err := copyObject.Context(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("shallow object copy", err)
	}
	other, err := NewContext(manager, config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := other.Close(); err != nil {
			t.Error(err)
		}
	}()
	c, err := NewObject(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckContext(a, c); !errors.Is(err, ErrContextMismatch) {
		t.Fatal("context mismatch", err)
	}
	if _, err := (*Object)(nil).Manager(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("nil object", err)
	}
}

func TestSymbolNamesSignedNumbersAndAssignment(t *testing.T) {
	context, config, manager := dualityContextFixture(t)
	name, err := context.StringSymbol("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := name.String(); err != nil || got != "alpha" {
		t.Fatal("named symbol", got, err)
	}
	for _, value := range []int{0, 17, -1, -2147483648, 2147483647} {
		symbol, err := context.IntSymbol(value)
		if err != nil {
			t.Fatal(err)
		}
		got, err := symbol.String()
		if err != nil {
			t.Fatal(err)
		}
		want := map[int]string{0: "0", 17: "17", -1: "-1", -2147483648: "-2147483648", 2147483647: "2147483647"}[value]
		if got != want {
			t.Fatal("signed numerical symbol", value, got, want)
		}
		var out bytes.Buffer
		if err := symbol.Write(&out); err != nil || out.String() != want {
			t.Fatal("symbol writer", value, out.String(), err)
		}
	}
	otherContext, err := NewContext(manager, config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := otherContext.Close(); err != nil {
			t.Error(err)
		}
	}()
	other, err := otherContext.StringSymbol("beta")
	if err != nil {
		t.Fatal(err)
	}
	if err := name.Assign(other); err != nil {
		t.Fatal(err)
	}
	if got, err := name.String(); err != nil || got != "beta" {
		t.Fatal("symbol assignment", got, err)
	}
	assignedObject, err := name.Object()
	if err != nil {
		t.Fatal(err)
	}
	otherObject, _ := other.Object()
	if err := CheckContext(assignedObject, otherObject); err != nil {
		t.Fatal("assignment did not transfer context", err)
	}
	equal, err := name.Equal(other)
	if err != nil || !equal {
		t.Fatal("assigned equality", equal, err)
	}
	copySymbol := *name
	if _, err := copySymbol.String(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("shallow symbol copy", err)
	}
}

var errDualityWriter = errors.New("duality writer refused")

type dualityRefusingWriter struct{}

func (dualityRefusingWriter) Write([]byte) (int, error) { return 0, errDualityWriter }

type dualityShortWriter struct{}

func (dualityShortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return len(p) - 1, nil
}

func TestSymbolWriterFailureForeignOwnerAndClosedContext(t *testing.T) {
	context, _, _ := dualityContextFixture(t)
	symbol, err := context.StringSymbol("payload")
	if err != nil {
		t.Fatal(err)
	}
	if err := symbol.Write(dualityRefusingWriter{}); !errors.Is(err, errDualityWriter) {
		t.Fatal("writer error", err)
	}
	if err := symbol.Write(dualityShortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal("short writer", err)
	}
	if err := symbol.Write(nil); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("nil writer", err)
	}

	foreignParams, _ := dualityParamsFixture(t)
	foreign, err := foreignParams.SymbolsField.Intern("foreign")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newSymbol(context, foreign); !errors.Is(err, usymbol.ErrOwner) {
		t.Fatal("foreign symbol owner", err)
	}
	if err := context.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := symbol.String(); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("symbol escaped context", err)
	}
	if _, err := context.StringSymbol("closed"); !errors.Is(err, ErrWrapperOwner) {
		t.Fatal("closed context created symbol", err)
	}
}
