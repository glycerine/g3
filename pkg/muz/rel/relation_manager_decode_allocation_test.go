package rel

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestRelationManagerDecodeAllocationFailureReleasesFreshNumerals(t *testing.T) {
	for _, mode := range []string{"value", "fact"} {
		t.Run(mode, func(t *testing.T) {
			failures, successes := 0, 0
			for _, allowance := range []int64{0, 1, 2, 3, 4, 8, 16, 32, 64, 128} {
				t.Run(fmt.Sprintf("reservations-%d", allowance), func(t *testing.T) {
					// Each attempt starts with fresh physical AST pages. Holding all
					// decoded values crosses both numeral and reference page boundaries.
					f := newTableRelationFixture(t)
					sort, err := f.ast.BitVectorSort(10)
					astTestMust(t, err)
					astTestPin(t, f.ast, sort)
					sorts := make([]ast.ASTHandle, 512)
					fact := make(Fact, len(sorts))
					for i := range sorts {
						sorts[i], fact[i] = sort, Element(i)
					}
					before := f.ast.Stats()
					var refs []ast.ASTRef
					var cause error
					f.ast.Budget().FailAfter(allowance)
					if mode == "fact" {
						refs, cause = f.relations.DecodeFact(sorts, fact)
					} else {
						for _, value := range fact {
							var ref ast.ASTRef
							ref, cause = f.relations.DecodeValue(sort, value)
							if cause != nil {
								if ref != (ast.ASTRef{}) {
									t.Error("failed value decoder published a partial owner", ref)
									refs = append(refs, ref)
								}
								break
							}
							refs = append(refs, ref)
						}
					}
					f.ast.Budget().FailAfter(-1)
					var receipt memory.ConstructionFailure
					if errors.As(cause, &receipt) {
						if receipt.RuntimeOwner() != f.ast.NumeralService().RuntimeOwner() {
							t.Fatal("decoder cleanup receipt lost its native runtime")
						}
						shown, exposed := receipt.Error(), receipt.Unwrap()
						astTestMust(t, receipt.Cleanup())
						astTestMust(t, receipt.Cleanup())
						if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil {
							t.Fatal("decoder cleanup changed its stable receipt")
						}
					}
					if cause != nil {
						failures++
						if !errors.Is(cause, arena.ErrInjectedFailure) {
							t.Error("unexpected decoder failure", cause)
						}
						if mode == "fact" && refs != nil {
							t.Error("failed fact decoder published a partial row")
						}
					} else {
						successes++
						if len(refs) != len(fact) {
							t.Fatal("completed decoder returned an incomplete row", len(refs))
						}
						for i, ref := range refs {
							value, err := f.ast.RefGet(ref)
							astTestMust(t, err)
							roundtrip, err := f.relations.EncodeValue(sort, value)
							astTestMust(t, err)
							if roundtrip != fact[i] {
								t.Fatal("decoded numeral changed its finite value", i, roundtrip)
							}
						}
					}
					for i := len(refs) - 1; i >= 0; i-- {
						astTestMust(t, f.ast.DelRef(&refs[i]))
					}
					after := f.ast.Stats()
					if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences || after.LiveInfos != before.LiveInfos || after.LiveParameters != before.LiveParameters {
						t.Fatalf("decoder leaked fresh numeral state: nodes %d/%d refs %d/%d infos %d/%d parameters %d/%d; cause %v", after.LiveNodes, before.LiveNodes, after.LiveReferences, before.LiveReferences, after.LiveInfos, before.LiveInfos, after.LiveParameters, before.LiveParameters, cause)
					}
				})
			}
			if failures == 0 || successes == 0 {
				t.Fatal("decoder campaign missed interruption or completion", failures, successes)
			}
		})
	}
}
