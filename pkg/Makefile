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

bench:
	go test ./smtcore -run '^$$' \
	-bench '^BenchmarkSMTScopeRestoration$$' \
	-benchtime=10000x

cppbench:
	cd ../build && make -j 48 && \
	make test -j 48 && \
	./test-z3 smt_scope_restoration

#BenchmarkSMTScopeRestoration/SurvivingAtoms	10000	458.37 ns/op	128 atoms/op	64 clauses/op
#BenchmarkSMTScopeRestoration/ScopedAtoms	10000	12295.14 ns/op	128 atoms/op	64 clauses/op
#PASS
#(test smt_scope_restoration :time 0.39 :before-memory 0.00 :after-memory 0.12)


#-bench '^BenchmarkSMT(BooleanPropagation|RelevanceCallbacks|ConflictResolution|ScopeRestoration)$$' \
#-benchmem -benchtime=2s -count=5

# To profile one component, select its exact name:
#
# go test ./smtcore -run '^$' \
#   -bench '^BenchmarkSMTBooleanPropagation$' \
#   -benchmem -benchtime=5s \
#   -cpuprofile=/tmp/boolean-propagation.cpu
#
# go tool pprof -http=localhost:8080 /tmp/boolean-propagation.cpu
#
# The four benchmark names are:
#
# - `BenchmarkSMTBooleanPropagation`
# - `BenchmarkSMTRelevanceCallbacks`
# - `BenchmarkSMTConflictResolution`
# - `BenchmarkSMTScopeRestoration` # the slow one
#
# `-run '^$'` skips unit tests. Benchmark timings exclude setup and resets; CPU profiles also capture
# that untimed work, so filter pprof by the component function when analyzing it.
#
# starting point, 2026 Nov 06:
# at 9c41fa9ad (HEAD -> master, origin/master, origin/HEAD) Restore source BV equality scheduling to fix optimization regressions
#
# | CPU activity        | C++    |  Go     |
# |---------------------|-------:|--------:|
# | Boolean propagation | 2.22 s | 12.48 s |
# | Relevance callbacks | 0.36 s |  3.85 s |
# | Conflict resolution | 0.92 s |  4.28 s |
# | Scope restoration   | 0.24 s |  7.35 s |
# | Background GC       | —      |  4.01 s |
#
# These are inclusive CPU times; rows can overlap.
#
# Relevance callback cost fell from **8.96 to 3.85 CPU seconds**. The largest remaining relative
# gap is scope restoration, roughly 31× the native cost.
