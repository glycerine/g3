package lp

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func lpCheckConstraintVector(t *testing.T, h *lpPairHarness, v *SparseRationalVector, indices []uint32, values []*big.Rat) {
	t.Helper()
	if int(v.Size()) != len(indices) {
		t.Fatal("coefficient count", v.Size(), len(indices))
	}
	for i, j := range indices {
		if v.Data[i].Index != j {
			t.Fatal("coefficient index/order", i, v.Data[i].Index, j)
		}
		lpSparseRationalCheck(t, h, &v.Data[i].Value, values[i])
	}
}

func TestLARConstraintsIndependentExactCopiesAndBorrows(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	h := newLPPairHarness(t)
	p := h.pair("0", "0")
	r := rand.New(rand.NewPCG(66391, 129))
	kinds := []LConstraintKind{LConstraintLE, LConstraintLT, LConstraintEQ, LConstraintGT, LConstraintGE}
	for world := 0; world < 120; world++ {
		rhs := new(big.Rat).SetFrac(new(big.Int).Lsh(big.NewInt(int64(r.IntN(2001)-1000)), uint(r.IntN(180))), big.NewInt(int64(r.IntN(97)+1)))
		h.must(h.m.SetString(&p.X, rhs.RatString()))
		j := r.Uint32()
		kind := kinds[world%len(kinds)]
		variable, err := NewLARVarConstraint(h.m, j, kind, &p.X)
		h.must(err)
		term := NewLARTerm(h.m)
		constant := big.NewRat(int64(r.IntN(21)-10), 13)
		h.must(h.m.SetString(&p.Y, constant.RatString()))
		h.must(term.SetValue(&p.Y))
		input := NewSparseRationalVector(h.m)
		var indices []uint32
		var values []*big.Rat
		want := make(map[uint32]*big.Rat)
		orderOracle := newLPTermOrder()
		for i := 0; i < world%13; i++ {
			column := uint32(r.IntN(5))
			x := new(big.Rat).SetFrac(new(big.Int).Lsh(big.NewInt(int64(r.IntN(19)-9)), uint(r.IntN(130))), big.NewInt(int64(r.IntN(41)+1)))
			h.must(h.m.SetString(&p.Y, x.RatString()))
			h.must(input.Append(column, &p.Y))
			h.must(term.AddCoefficient(column, &p.Y))
			indices = append(indices, column)
			values = append(values, x)
			orderOracle = lpTermOracleAdd(want, orderOracle, column, x)
		}
		order := orderOracle.Keys()
		borrowed, err := NewLARTermConstraint(h.m, term, kind, &p.X)
		h.must(err)
		owned, err := NewLARConstraint(h.m, input.Data, kind, &p.X)
		h.must(err)
		if variable.Size() != 1 || owned.Size() != input.Size() || borrowed.Size() != term.Size() {
			t.Fatal("sizes")
		}
		if variable.Kind != kind || borrowed.Kind != kind || owned.Kind != kind {
			t.Fatal("relation kind")
		}
		h.must(h.m.SetInt32(&p.X, 23))
		for _, base := range []*LARBaseConstraint{&variable.LARBaseConstraint, &borrowed.LARBaseConstraint, &owned.LARBaseConstraint} {
			lpSparseRationalCheck(t, h, &base.RightSide, rhs)
		}
		for i := range input.Data {
			h.must(h.m.SetInt32(&input.Data[i].Value, 29))
		}
		v, err := variable.CoefficientsAsVector()
		h.must(err)
		lpCheckConstraintVector(t, h, v, []uint32{j}, []*big.Rat{big.NewRat(1, 1)})
		h.must(v.Close())
		h.must(variable.FreeCoefficient(&p.X))
		lpSparseRationalCheck(t, h, &p.X, new(big.Rat))
		h.must(owned.FreeCoefficient(&p.X))
		lpSparseRationalCheck(t, h, &p.X, new(big.Rat))
		v, err = owned.CoefficientsAsVector()
		h.must(err)
		lpCheckConstraintVector(t, h, v, indices, values)
		for i := range v.Data {
			h.must(h.m.SetInt32(&v.Data[i].Value, -7))
		}
		h.must(v.Close())
		v, err = owned.CoefficientsAsVector()
		h.must(err)
		lpCheckConstraintVector(t, h, v, indices, values)
		h.must(v.Close())
		v, err = borrowed.CoefficientsAsVector()
		h.must(err)
		var termValues []*big.Rat
		for _, index := range order {
			termValues = append(termValues, want[index])
		}
		lpCheckConstraintVector(t, h, v, order, termValues)
		// A term export is an independent value, but future constraint queries
		// observe actual mutations to the borrowed term.
		h.must(term.AddCoefficient(101, &p.X))
		lpCheckConstraintVector(t, h, v, order, termValues)
		h.must(v.Close())
		if borrowed.Size() != uint32(len(order)+1) {
			t.Fatal("term was copied")
		}
		h.must(borrowed.FreeCoefficient(&p.X))
		lpSparseRationalCheck(t, h, &p.X, constant)
		h.must(h.m.SetInt32(&p.Y, 31))
		h.must(term.SetValue(&p.Y))
		lpSparseRationalCheck(t, h, &p.X, constant)
		h.must(borrowed.FreeCoefficient(&p.X))
		lpSparseRationalCheck(t, h, &p.X, big.NewRat(31, 1))
		h.must(borrowed.Close())
		if !term.Contains(101) {
			t.Fatal("constraint destroyed its borrowed term")
		}
		lpSparseRationalCheck(t, h, &term.Value, big.NewRat(31, 1))
		h.must(variable.Close())
		h.must(owned.Close())
		h.must(input.Close())
		h.must(term.Close())
		lpExactVectorNamedCensus(t, h, nil)
	}
}

func TestLARConstraintsConstructionAndExportPhysicalRefusals(t *testing.T) {
	if !testing.Short() {
		t.Skip("lp: test too slow, run with -short for full suite.")
	}

	for _, op := range []string{"variable", "term", "general", "export", "stored-append", "term-export", "free-coefficient"} {
		failures := 0
		succeeded := false
		for ordinal := int64(0); ordinal < 180; ordinal++ {
			done := false
			t.Run(fmt.Sprintf("%s/%d", op, ordinal), func(t *testing.T) {
				h := newLPPairHarness(t)
				value := strings.Repeat("1234567890", 100) + "/7"
				p := h.pair(value, "0")
				input := NewSparseRationalVector(h.m)
				term := NewLARTerm(h.m)
				h.must(term.SetValue(&p.X))
				for _, j := range []uint32{3, 3, 7, 11} {
					h.must(input.Append(j, &p.X))
					h.must(term.AddCoefficient(j, &p.X))
				}
				var variable *LARVarConstraint
				var borrowed *LARTermConstraint
				var owned *LARConstraint
				var exported *SparseRationalVector
				var err error
				if op == "export" || op == "stored-append" {
					owned, err = NewLARConstraint(h.m, input.Data, LConstraintGE, &p.Y)
					h.must(err)
				}
				if op == "term-export" || op == "free-coefficient" {
					borrowed, err = NewLARTermConstraint(h.m, term, LConstraintGT, &p.Y)
					h.must(err)
				}
				h.budget.FailAfter(ordinal)
				switch op {
				case "variable":
					variable, err = NewLARVarConstraint(h.m, 81, LConstraintLT, &p.X)
				case "term":
					borrowed, err = NewLARTermConstraint(h.m, term, LConstraintLT, &p.X)
				case "general":
					owned, err = NewLARConstraint(h.m, input.Data, LConstraintLT, &p.X)
				case "export":
					exported, err = owned.CoefficientsAsVector()
				case "stored-append":
					err = owned.Coefficients.Append(31, &p.X)
				case "term-export":
					exported, err = borrowed.CoefficientsAsVector()
				case "free-coefficient":
					err = borrowed.FreeCoefficient(&p.Y)
				}
				h.budget.FailAfter(-1)
				if err != nil {
					if !lpExactPriorityPhysicalRefusal(err) {
						t.Fatal(err)
					}
					failures++
					switch op {
					case "variable":
						if variable == nil || variable.Kind != LConstraintLT || variable.Column != 0 {
							t.Fatal("variable construction prefix")
						}
						if _, err = variable.CoefficientsAsVector(); err != ErrExactVectorPending {
							t.Fatal(err)
						}
						if err = variable.FreeCoefficient(&p.Y); err != ErrExactVectorPending {
							t.Fatal(err)
						}
					case "term":
						if borrowed == nil || borrowed.Kind != LConstraintLT || borrowed.Term != nil {
							t.Fatal("term construction prefix")
						}
						if _, err = borrowed.CoefficientsAsVector(); err != ErrExactVectorPending {
							t.Fatal(err)
						}
						if err = borrowed.FreeCoefficient(&p.Y); err != ErrExactVectorPending {
							t.Fatal(err)
						}
					case "general":
						if owned == nil || owned.Kind != LConstraintLT {
							t.Fatal("general construction prefix")
						}
						if _, err = owned.CoefficientsAsVector(); err != ErrExactVectorPending {
							t.Fatal(err)
						}
						if owned.Coefficients != nil {
							want, _ := new(big.Rat).SetString(value)
							lpSparseRationalCheck(t, h, &owned.RightSide, want)
							for i := range owned.Coefficients.Data {
								if owned.Coefficients.Data[i].Index != input.Data[i].Index {
									t.Fatal("changed constructor prefix")
								}
								lpSparseRationalCheck(t, h, &owned.Coefficients.Data[i].Value, want)
							}
						}
					case "export", "term-export":
						if exported == nil {
							t.Fatal("missing returned owner")
						}
						if owned != nil && owned.ready() != nil || borrowed != nil && borrowed.Term.ready() != nil {
							t.Fatal("export poisoned its source")
						}
						for i := range exported.Data {
							want, _ := new(big.Rat).SetString(value)
							if op == "term-export" && exported.Data[i].Index == 3 {
								want.Mul(want, big.NewRat(2, 1))
							}
							lpSparseRationalCheck(t, h, &exported.Data[i].Value, want)
						}
					case "stored-append":
						if _, err = owned.CoefficientsAsVector(); err != ErrExactVectorPending {
							t.Fatal(err)
						}
					case "free-coefficient":
						if err = borrowed.FreeCoefficient(&p.Y); err != ErrExactVectorPending {
							t.Fatal(err)
						}
					}
				} else {
					done = true
					want, _ := new(big.Rat).SetString(value)
					switch op {
					case "variable":
						lpSparseRationalCheck(t, h, &variable.RightSide, want)
						if variable.Column != 81 {
							t.Fatal(variable.Column)
						}
					case "term":
						lpSparseRationalCheck(t, h, &borrowed.RightSide, want)
					case "general", "export":
						v := exported
						if v == nil {
							v = owned.Coefficients
						}
						lpCheckConstraintVector(t, h, v, []uint32{3, 3, 7, 11}, []*big.Rat{want, want, want, want})
					case "stored-append":
						lpCheckConstraintVector(t, h, owned.Coefficients, []uint32{3, 3, 7, 11, 31}, []*big.Rat{want, want, want, want, want})
					case "term-export":
						// The term's coefficients follow libstdc++ unordered_map order.
						termOrder := newLPTermOrder()
						var termValues []*big.Rat
						for _, j := range []uint32{3, 3, 7, 11} {
							termOrder.keys.Put(j, lpTermOrderValue)
						}
						for _, j := range termOrder.Keys() {
							if j == 3 {
								termValues = append(termValues, new(big.Rat).Mul(want, big.NewRat(2, 1)))
							} else {
								termValues = append(termValues, want)
							}
						}
						lpCheckConstraintVector(t, h, exported, termOrder.Keys(), termValues)
					case "free-coefficient":
						lpSparseRationalCheck(t, h, &p.Y, want)
					}
				}
				if exported != nil {
					h.must(exported.Close())
				}
				if variable != nil {
					h.must(variable.Close())
				}
				if borrowed != nil {
					h.must(borrowed.Close())
				}
				if owned != nil {
					h.must(owned.Close())
				}
				h.must(h.m.Del(&p.Y))
				p.Y = numeral.NewRational()
				h.must(input.Close())
				h.must(term.Close())
				lpExactVectorNamedCensus(t, h, nil)
			})
			if done {
				succeeded = true
				break
			}
		}
		if failures == 0 || !succeeded {
			t.Fatal("incomplete physical refusal campaign", op, failures, succeeded)
		}
	}
}

func TestLARConstraintsForeignManagerAndStoppedTerm(t *testing.T) {
	h := newLPPairHarness(t)
	foreign := newLPPairHarness(t)
	p := h.pair("17/19", "0")
	term := NewLARTerm(foreign.m)
	c, err := NewLARTermConstraint(h.m, term, LConstraintGE, &p.X)
	if err != ErrExactVectorOwner || c.Term != nil {
		t.Fatal(err)
	}
	h.must(c.Close())
	h.must(term.Close())
	term = NewLARTerm(h.m)
	c, err = NewLARTermConstraint(h.m, term, LConstraintEQ, &p.X)
	h.must(err)
	// A stopped real substitution retains a lexical owner even when its
	// erased live pivot yields no outstanding arithmetic frame.
	v := NewIndexedRationalVector(h.m, 1)
	h.must(term.AddCoefficient(0, &p.X))
	h.must(h.m.SetInt32(&p.Y, 1))
	h.must(v.SetValue(&p.Y, 0))
	if err = term.Substitute(0, v); err != ErrLARTermPivotErased {
		t.Fatal(err)
	}
	if _, err = c.CoefficientsAsVector(); err != ErrExactVectorPending {
		t.Fatal(err)
	}
	if err = c.FreeCoefficient(&p.Y); err != ErrExactVectorPending {
		t.Fatal(err)
	}
	h.must(c.Close())
	h.must(term.Close())
	h.must(v.Close())
	lpExactVectorNamedCensus(t, h, nil)
}
