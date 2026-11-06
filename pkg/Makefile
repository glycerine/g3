.PHONY: fast tags slow

fast:
	time go test -timeout=0 -count=1 ./...

tags:
	time go test -timeout=0 -count=1 -short -tags g3checked,g3debug,g3trace $$( go list ./... | grep -v math/polynomial | grep -v util/lp |grep -v pkg/qe)

slow:
	time go test -timeout=0 -count=1 -short -tags g3checked,g3debug,g3trace github.com/glycerine/g3/pkg/qe  github.com/glycerine/g3/pkg/math/polynomial   github.com/glycerine/g3/pkg/util/lp


## Q: which are the very slow tests under the 3 tags?
##
## A: Under `-tags g3checked,g3debug,g3trace`, the slow tests measured were:
## 
## - `pkg/muz/pdr`: `TestModelSearchSourceRandomTrees` took about **36 seconds** for its bounded 100-round run.
## - `pkg/muz/pdr`: `TestPDRNestedArrayEvaluationColdFailureCleanupAndRetry` took about **33 seconds** on its own.
## - `pkg/smtcore`: `TestPseudoBooleanTheorySourceNestedFuzz` took about **22 seconds** for the 1,000-round prefix.
## 
## The full tagged package runs took **147 seconds for PDR** and **60 seconds for smtcore**.
## The full 100,000-round PB fuzzer is opt-in; the C++ `theory_pb` test also did not finish within two minutes.
##
## Follow-up: yeah we should probably skip these.
