package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTArrayParentsPrivateMergesDelayedUpwardAndRollback(t *testing.T) {
	for _, simple := range []bool{false, true} {
		for _, relevant := range []bool{false, true} {
			for _, weak := range []bool{false, true} {
				t.Run(fmt.Sprint(simple, "/", relevant, "/", weak), func(t *testing.T) {
					m := astTestManager(t)
					family, err := m.RegisterArrayPlugin()
					astTestMust(t, err)
					_, _, s := smtInternalizerFixture(t, m, nil)
					s.relevancy.enabled = relevant
					var plugin SMTTheoryInternalizer
					if simple {
						plugin, err = NewSMTSimpleArrayTheory(family)
					} else {
						plugin, err = NewSMTArrayTheory(family)
					}
					astTestMust(t, err)
					astTestMust(t, s.RegisterTheory(plugin))
					theory := smtArrayCore(plugin)
					config := theory.arrayConfig
					config.Weak = weak
					astTestMust(t, theory.ConfigureArray(config))
					astTestMust(t, s.SetupTheories())
					if (theory.arrayConfig.Laziness == 0) == relevant {
						t.Fatal("source relevance/laziness initialization was not consumed")
					}
					indexSort, indices := smtArrayQueueIndices(t, m, "parent-index-", 2)
					boolean, err := m.MkBoolSort()
					astTestMust(t, err)
					sort, err := m.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexSort}), boolean)
					astTestMust(t, err)
					a, err := m.MkConst(astTestSymbol(t, m, "parent-base"), sort)
					astTestMust(t, err)
					b, err := m.MkConst(astTestSymbol(t, m, "parent-alias"), sort)
					astTestMust(t, err)
					astTestMust(t, s.Internalize(b, false))
					value := smtRelevancyBool(t, m, "parent-value")
					store, err := m.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{a, indices[0], value})
					astTestMust(t, err)
					astTestMust(t, s.Internalize(store, false))
					read, err := m.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{a, indices[1]})
					astTestMust(t, err)
					astTestMust(t, s.Internalize(read, true))
					parentData := theory.arrayData[theory.arrayUnion.Find(uint32(s.context.enodes.get(a).TheoryVar(family)))]
					if relevant && len(parentData.parentSelects) != 0 {
						t.Fatal("irrelevant read entered parent list")
					}
					astTestMust(t, s.markRelevant(read))
					astTestMust(t, s.markRelevant(store))
					_, err = s.PropagateCore()
					astTestMust(t, err)
					baseVariable := s.context.enodes.get(a).TheoryVar(family)
					storeVariable := s.context.enodes.get(store).TheoryVar(family)
					aliasVariable := s.context.enodes.get(b).TheoryVar(family)
					base := theory.arrayData[theory.arrayUnion.Find(uint32(baseVariable))]
					if base.propUpward || len(base.parentStores) != 1 || len(base.parentSelects) == 0 {
						t.Fatal("initial parent lists or upward flag", base)
					}
					for attempt := uint32(1); attempt <= 2; attempt++ {
						astTestMust(t, s.PushScope())
						// Context event order merges the private store class into
						// the alias class; both partitions must undo on pop.
						astTestMust(t, s.addEquality(s.context.enodes.get(b), s.context.enodes.get(store), NewSMTEqualityAxiom()))
						astTestMust(t, s.PropagateTheoryEvents())
						if theory.arrayUnion.Find(uint32(storeVariable)) != theory.arrayUnion.Find(uint32(aliasVariable)) || base.propUpward == weak {
							t.Fatal("private merge/upward flag", weak, base.propUpward)
						}
						if len(theory.readTodo) != 0 {
							t.Fatal("expensive upward axiom was emitted before final check")
						}
						status, err := theory.FinalCheck()
						astTestMust(t, err)
						want := SMTFinalCheckContinue
						if weak {
							want = SMTFinalCheckDone
						}
						if status != want {
							t.Fatal("delayed upward final check", status, want)
						}
						if !weak {
							if len(theory.readTodo) != 1 || theory.statistics.expandedAxiom2 != attempt {
								t.Fatal("missing delayed upward axiom", theory.statistics.expandedAxiom2)
							}
							astTestMust(t, theory.Propagate())
						}
						astTestMust(t, s.PopScopes(1))
						if base.propUpward || theory.arrayUnion.Find(uint32(storeVariable)) == theory.arrayUnion.Find(uint32(aliasVariable)) || theory.CanPropagate() {
							t.Fatal("array partition, flags or work survived pop")
						}
						if int(theory.arrayUnion.GetNumVars()) != theory.NumVariables() || len(theory.arrayData) != theory.NumVariables() {
							t.Fatal("private array variable count diverged after pop")
						}
					}
				})
			}
		}
	}
}
