package smt2

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
)

func TestSMT2ConcreteDatatypeFormsAndDeclarationScopes(t *testing.T) {
	for _, declaration := range []string{
		"(declare-datatype Nat ((zero) (succ (pred Nat))))",
		"(declare-datatypes () ((Nat zero (succ (pred Nat)))))",
		"(declare-datatypes ((Nat 0)) ((zero (succ (pred Nat)))))",
	} {
		t.Run(declaration, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer context.Close()
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader("(push 1)"+declaration+"(declare-const n Nat)(assert (= (pred (succ n)) n))(check-sat-using (then simplify smt))"), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if output.String() != "sat\n" {
				t.Fatal(output.String())
			}
			if err = ParseCommands(context, strings.NewReader("(pop 1)"), &output); err != nil {
				t.Fatal(err)
			}
			if _, err = context.LookupSort("Nat"); err == nil {
				t.Fatal("popped datatype sort remained visible")
			}
			if _, err = context.MakeApp("zero", nil, nil, ast.ASTHandle{}); err == nil {
				t.Fatal("popped constructor remained visible")
			}
		})
	}
}

func TestSMT2DatatypeRefusalsBeforeDeclarationPublication(t *testing.T) {
	for _, declaration := range []string{
		"(declare-datatype D ())",
		"(declare-datatype D ((make (field Missing))))",
		"(declare-datatype D ((a (field Bool)) (b (field Bool))))",
		"(declare-datatype D ((make)) extra)",
		"(declare-datatypes ((D 0) (E 0)) (((make))))",
		"(declare-datatypes ((D 0)) (((make)) ((other))))",
		"(declare-datatypes ((D 2)) ((par (T) ((make (field T))))))",
	} {
		t.Run(declaration, func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer context.Close()
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(declaration), &output); err == nil {
				t.Fatal("invalid/unported declaration accepted")
			}
			if _, err = context.LookupSort("D"); err == nil {
				t.Fatal("rejected declaration published its sort")
			}
			if _, err = context.MakeApp("make", nil, nil, ast.ASTHandle{}); err == nil {
				t.Fatal("rejected declaration published its constructor")
			}
		})
	}
}

func TestSMT2ParametricDatatypeInstantiationAndRecursiveFields(t *testing.T) {
	// Not List: with no logic set, List is predeclared (cmd_context.cpp:725).
	for _, script := range []string{
		"(declare-datatype Box (par (T) ((box (unbox T)))))" +
			"(declare-const b (Box Bool))" +
			"(assert (= (unbox (box true)) true))(check-sat)",
		"(declare-datatypes ((Seqs 1)) ((par (T) ((nil) (cons (head T) (tail (Seqs T)))))))" +
			"(declare-const xs (Seqs Bool))" +
			"(assert (= (head (cons true nil)) true))(check-sat)",
		"(declare-datatypes (T) ((Seqs (nil) (cons (head T) (tail (Seqs T))))))" +
			"(declare-const xs (Seqs Bool))" +
			"(assert (= (head (cons true nil)) true))(check-sat)",
	} {
		t.Run(script[:32], func(t *testing.T) {
			context, err := cmd_context.NewStandaloneContext()
			if err != nil {
				t.Fatal(err)
			}
			defer context.Close()
			var output bytes.Buffer
			if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
				t.Fatal(err, output.String())
			}
			if output.String() != "sat\n" {
				t.Fatal(output.String())
			}
		})
	}
}

func TestSMT2ParametricDatatypeSortArityRefusals(t *testing.T) {
	for _, suffix := range []string{
		"(declare-const x Box)",
		"(declare-const x (Box Bool Int))",
	} {
		context, err := cmd_context.NewStandaloneContext()
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		err = ParseCommands(context, strings.NewReader("(declare-datatype Box (par (T) ((box (unbox T)))))"+suffix), &output)
		_ = context.Close()
		if err == nil {
			t.Fatal("accepted invalid parametric datatype sort", suffix)
		}
	}
}

func TestSMT2ParametricDatatypeInstantiationFunctionsFollowScopes(t *testing.T) {
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	defer context.Close()
	var output bytes.Buffer
	script := "(declare-datatype Box (par (T) ((box (unbox T)))))" +
		"(push 1)(declare-const scoped (Box Bool))(pop 1)" +
		"(declare-const retained (Box Bool))" +
		"(assert (= (unbox (box true)) true))(check-sat)"
	if err = ParseCommands(context, strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if output.String() != "sat\n" {
		t.Fatal(output.String())
	}
}

// AUDIT.md parsers_shell_01-F3: push_match_frame compiles a match into an
// ite chain over the constructor recognizers; check_patterns rejects
// non-datatype scrutinees and missing constructors.
func TestSMT2MatchExpression(t *testing.T) {
	for _, tc := range []struct{ script, want string }{
		{"(declare-datatypes ((L 0)) (((nil) (cons (hd Int) (tl L)))))(declare-const l L)(assert (= 3 (match l ((nil 0) ((cons h t) h)))))(check-sat)(get-value ((hd l)))",
			"sat\n(((hd l) 3))\n"},
		{"(declare-datatypes ((L 0)) (((nil) (cons (hd Int) (tl L)))))(declare-const l L)(simplify (match l ((nil 0) ((cons h t) (+ h 1)))))",
			"(ite ((_ is nil) l) 0 (+ 1 (hd l)))\n"},
		{"(simplify (match 3 ((x 1))))",
			"(error \"line 1 column 27: pattern matching is only supported for algebraic datatypes\")\n"},
		{"(declare-datatypes ((L 0)) (((nil) (cons (hd Int) (tl L)))))(declare-const l L)(simplify (match l ((nil 1) (nil 2))))",
			"(error \"line 1 column 116: a constructor is missing from pattern match\")\n"},
	} {
		c, err := cmd_context.NewStandaloneContext()
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		err = ParseShellCommands(c, strings.NewReader(tc.script), &out)
		if closeErr := c.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if strings.HasPrefix(tc.want, "(error") != (err != nil) {
			t.Fatal("parse error", tc.script, err)
		}
		if out.String() != tc.want {
			t.Fatalf("%s: got %q want %q", tc.script, out.String(), tc.want)
		}
	}
}
