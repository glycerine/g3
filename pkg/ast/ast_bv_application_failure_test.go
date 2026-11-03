package ast

// Source: bv_util::mk_numeral and ast_manager::mk_app. Numeral declarations
// are uncached and must survive exactly until their applications acquire them.

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestASTBitVectorNumeralApplicationFailureReleasesFreshDeclaration(t *testing.T) {
	for _, mode := range []string{"uint64", "unsigned-invocation"} {
		t.Run(mode, func(t *testing.T) {
			failures, successes := 0, 0
			for _, allowance := range []int64{0, 1, 2, 3, 4, 8, 16, 32, 64, 128} {
				t.Run(fmt.Sprint(allowance), func(t *testing.T) {
					m := astTestManager(t)
					_, err := m.RegisterArithmeticPlugin()
					astTestMust(t, err)
					_, err = m.RegisterBitVectorPlugin()
					astTestMust(t, err)
					sort, err := m.BitVectorSort(10)
					astTestMust(t, err)
					astTestPin(t, m, sort)
					// Reserve output slots first so a failed publication cannot
					// obscure the numeral factory's own declaration frontier.
					refs := make([]ASTRef, 512)
					for i := range refs {
						refs[i] = astTestPin(t, m, ASTHandle{})
					}
					before := m.Stats()
					var cause error
					var call ASTBitVectorNumeralInvocation
					m.Budget().FailAfter(allowance)
					for i := range refs {
						var value ASTHandle
						if mode == "uint64" {
							value, cause = m.BitVectorNumeralUint64(uint64(i+1024), 10)
						} else {
							call = ASTBitVectorNumeralInvocation{}
							value, cause = m.BitVectorNumeralUnsignedWithInvocation(uint32(i+1024), 10, &call)
						}
						if cause != nil {
							if !value.IsNull() {
								t.Error("failed numeral factory published an application", value, cause)
							}
							break
						}
						if cause = m.SetRef(refs[i], value); cause != nil {
							break
						}
					}
					m.Budget().FailAfter(-1)
					if mode == "unsigned-invocation" {
						astTestMust(t, m.CleanupBitVectorNumeralInvocation(&call))
						astTestMust(t, m.CleanupBitVectorNumeralInvocation(&call))
					}
					if cause != nil {
						failures++
						if !errors.Is(cause, arena.ErrInjectedFailure) {
							t.Fatal("unexpected numeral allocation failure", cause)
						}
					} else {
						successes++
						out, err := m.numerals.NewValue()
						astTestMust(t, err)
						defer func() { astTestMust(t, m.numerals.Del(&out)) }()
						for i, ref := range refs {
							value, err := m.RefGet(ref)
							astTestMust(t, err)
							width := uint32(0)
							ok, err := m.BitVectorNumeralValue(value, &out, &width)
							astTestMust(t, err)
							text, err := m.numerals.String(&out)
							astTestMust(t, err)
							if !ok || width != 10 || text != strconv.Itoa(i) {
								t.Fatal("numeral application changed normalization", i, ok, width, text)
							}
						}
						// Rebuilding a shared numeral must leave its existing
						// application and sole declaration reference intact.
						existing, err := m.RefGet(refs[7])
						astTestMust(t, err)
						decl, err := m.GetDecl(existing)
						astTestMust(t, err)
						count, err := m.GetRefCount(decl)
						astTestMust(t, err)
						again, err := m.BitVectorNumeralUint64(1031, 10)
						astTestMust(t, err)
						afterCount, err := m.GetRefCount(decl)
						astTestMust(t, err)
						if again != existing || count != 1 || afterCount != count {
							t.Fatal("shared numeral declaration ownership", again, existing, count, afterCount)
						}
					}
					for i := len(refs) - 1; i >= 0; i-- {
						astTestMust(t, m.SetRef(refs[i], ASTHandle{}))
					}
					after := m.Stats()
					if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
						t.Fatalf("numeral frontier left uncached declaration storage: nodes %d/%d refs %d/%d infos %d/%d parameters %d/%d; cause %v", after.LiveNodes, before.LiveNodes, after.LiveReferences, before.LiveReferences, after.LiveInfos, before.LiveInfos, after.LiveParameters, before.LiveParameters, cause)
					}
				})
			}
			if failures == 0 || successes == 0 {
				t.Fatal("numeral allocation campaign missed interruption or completion", failures, successes)
			}
		})
	}
}
