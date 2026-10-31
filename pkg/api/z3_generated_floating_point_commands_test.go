package api

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestGeneratedFloatingPointReplayConstructors(t *testing.T) {
	var log strings.Builder
	log.WriteString("R\nP 0\nC 6\n= 0x41\n")
	for id := 571; id <= 590; id++ {
		log.WriteString("R\nP 0x41\n")
		if id == 582 {
			log.WriteString("U 8\nU 24\n")
		}
		fmt.Fprintf(&log, "C %d\n= 0x%x\n", id, id)
	}
	log.WriteString("R\nP 0x41\nP 0x246\nC 591\n= 0x301\n") // sort 582
	log.WriteString("R\nP 0x41\nP 0x246\nI 1\nC 592\n= 0x302\n")
	log.WriteString("R\nP 0x41\nP 0x246\nI 1\nC 593\n= 0x303\n")
	log.WriteString("R\nP 0x41\nF 1.5\nP 0x246\nC 595\n= 0x304\n")
	log.WriteString("R\nP 0x41\nD 1.5\nP 0x246\nC 596\n= 0x305\n")
	log.WriteString("R\nP 0x41\nI 1\nP 0x246\nC 597\n= 0x306\n")
	log.WriteString("R\nP 0x41\nI 0\nI 0\nU 0\nP 0x246\nC 598\n= 0x307\n")
	log.WriteString("R\nP 0x41\nI 0\nI 0\nU 0\nP 0x246\nC 599\n= 0x308\n")
	log.WriteString("R\nP 0x41\nU 1\nC 35\n= 0x309\nR\nP 0x41\nU 8\nC 35\n= 0x30a\nR\nP 0x41\nU 23\nC 35\n= 0x30b\n")
	log.WriteString("R\nP 0x41\nU 0\nP 0x309\nC 151\n= 0x30c\nR\nP 0x41\nU 127\nP 0x30a\nC 151\n= 0x30d\nR\nP 0x41\nU 0\nP 0x30b\nC 151\n= 0x30e\n")
	log.WriteString("R\nP 0x41\nP 0x30c\nP 0x30d\nP 0x30e\nC 594\n= 0x30f\n")
	log.WriteString("R\nP 0x41\n")
	for id := 571; id <= 590; id++ {
		fmt.Fprintf(&log, "P 0x%x\n", id)
	}
	for id := 0x301; id <= 0x308; id++ {
		fmt.Fprintf(&log, "P 0x%x\n", id)
	}
	log.WriteString("P 0x30f\n")
	log.WriteString("C 900\n")
	r, err := NewZ3Replayer(strings.NewReader(log.String()), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(900, "check-floating-point-constructors", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		handles := make([]ast.ASTHandle, 29)
		for i := range handles {
			v, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			handles[i] = v.handle
		}
		for i := 1; i < 11; i += 2 {
			if handles[i] != handles[i+1] {
				t.Fatalf("rounding aliases %d", i)
			}
		}
		for i := 12; i < 20; i += 2 {
			if handles[i] != handles[i+1] {
				t.Fatalf("sort aliases %d", i)
			}
		}
		if handles[11] != handles[14] {
			t.Fatal("explicit and single precision sorts differ")
		}
		for i, kind := range []ast.FPAClassificationKind{ast.FPAIsNaN, ast.FPAIsInfinite, ast.FPAIsZero} {
			found, yes, e := owner.results.manager.ClassifyFPANumeral(handles[20+i], kind)
			if e != nil {
				return e
			}
			if !found || !yes {
				t.Fatal("special value", i)
			}
		}
		if handles[23] != handles[24] {
			t.Fatal("float and double values differ")
		}
		if handles[26] != handles[27] {
			t.Fatal("component constructors differ")
		}
		kind, e := owner.results.manager.GetDeclKind(handles[28])
		if e != nil {
			return e
		}
		if kind != ast.FPAOpFP {
			t.Fatal("floating-point constructor kind", kind)
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("floating-point constructors not checked")
	}
}

func TestGeneratedFloatingPointReplayOperations(t *testing.T) {
	var log strings.Builder
	log.WriteString("R\nP 0\nC 6\n= 0x41\nR\nP 0x41\nC 585\n= 0x42\nR\nP 0x41\nC 572\n= 0x43\n")
	log.WriteString("R\nP 0x41\nI 1\nP 0x42\nC 597\n= 0x44\nR\nP 0x41\nI 2\nP 0x42\nC 597\n= 0x45\n")
	for id := 600; id <= 623; id++ {
		log.WriteString("R\nP 0x41\n")
		if id >= 602 && id <= 607 || id == 609 {
			log.WriteString("P 0x43\n")
		}
		log.WriteString("P 0x44\n")
		if id >= 602 && id <= 606 || id == 608 || id >= 610 && id <= 616 {
			log.WriteString("P 0x45\n")
		}
		if id == 606 {
			log.WriteString("P 0x44\n")
		}
		fmt.Fprintf(&log, "C %d\n= 0x%x\n", id, id)
	}
	log.WriteString("R\nP 0x41\nP 0x25a\nC 287\n= 0x501\nR\nP 0x41\nI 3\nP 0x42\nC 597\n= 0x502\n")
	log.WriteString("R\nP 0x41\n")
	for id := 600; id <= 623; id++ {
		fmt.Fprintf(&log, "P 0x%x\n", id)
	}
	log.WriteString("P 0x501\nP 0x502\n")
	log.WriteString("C 900\n")
	r, err := NewZ3Replayer(strings.NewReader(log.String()), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := r.Close(); e != nil {
			t.Error(e)
		}
	}()
	if err = RegisterZ3GeneratedContextCommands(r); err != nil {
		t.Fatal(err)
	}
	if err = RegisterZ3GeneratedASTCommands(r); err != nil {
		t.Fatal(err)
	}
	checked := false
	if err = r.RegisterCommand(900, "check-floating-point-operations", func(r *Z3Replayer) error {
		owner, e := replayContext(r, 0)
		if e != nil {
			return e
		}
		kinds := []int32{ast.FPAOpAbs, ast.FPAOpNeg, ast.FPAOpAdd, ast.FPAOpSub, ast.FPAOpMul, ast.FPAOpDiv, ast.FPAOpFMA, ast.FPAOpSqrt, ast.FPAOpRem, ast.FPAOpRoundToIntegral, ast.FPAOpMin, ast.FPAOpMax, ast.FPAOpLe, ast.FPAOpLt, ast.FPAOpGe, ast.FPAOpGt, ast.FPAOpEq, int32(ast.FPAIsNormal), int32(ast.FPAIsSubnormal), int32(ast.FPAIsZero), int32(ast.FPAIsInfinite), int32(ast.FPAIsNaN), int32(ast.FPAIsNegative), int32(ast.FPAIsPositive)}
		for i, want := range kinds {
			value, e := r.astObject(owner, uint32(i+1))
			if e != nil {
				return e
			}
			got, e := owner.results.manager.GetDeclKind(value.handle)
			if e != nil {
				return e
			}
			if got != want {
				t.Fatalf("operation %d: got %d, want %d", 600+i, got, want)
			}
		}
		sum, e := r.astObject(owner, 25)
		if e != nil {
			return e
		}
		three, e := r.astObject(owner, 26)
		if e != nil {
			return e
		}
		if sum.handle != three.handle {
			t.Fatal("floating-point addition did not simplify to three")
		}
		checked = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = r.Parse(); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("floating-point operations not checked")
	}
}
