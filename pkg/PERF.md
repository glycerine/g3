# g3 performance audit

Audit of the Go port in `pkg/` against the C++ in `src/`, done 2026-11-06/07 on
branch `perf-audit-fixes`, go1.27.3, with `build/z3` and `build/test-z3` freshly
built by `compile.sh`.

**Read B1 first.** It is a soundness bug: g3 returns `unsat` for satisfiable
formulas. Its candidate fix is not committed because of a blocker described there.

## Why is Go so much slower?

Go itself is not the main reason. At the data-structure level the port is not
mechanical, and four differences account for most of the measured gap:

1. **Every access is a checked call.** C++ reads `n->get_decl()` or
   `m_assignment[l]` directly. Go goes through a method that validates the owner
   and handle and returns an error (P5, P6). The SAT solver did this per byte.
2. **Backtracking rebuilds state rather than undoing it.** C++ pops a compact
   trail. Go snapshots on every merge, clones whole maps on every decision, and
   rebuilds the entire congruence table on backjumps (P1, P2, P3).
3. **Hash maps where C++ uses dense arrays.** C++ indexes by expression id or bool
   var. Go uses `map[ast.ASTHandle]...` with a 16-byte key (P4, 1,972 sites).
4. **`big.Rat` for all arithmetic.** C++ `mpq` has a small-integer fast path that
   does not allocate. `math/simplex` and the arithmetic theories allocate and run
   GCD on small values (P8).

GC costs come on top of this and are comparatively cheap to reduce (P7).

## Measurements

All timings are wall clock on the same machine. Workloads were generated for this
audit and are synthetic. Their exact definitions are in "Reproducing" below.

### Go vs C++ before any change

| Workload | C++ | Go | Ratio | Search comparable? |
|---|---:|---:|---:|---|
| `r3sat190.cnf` (SAT core) | 0.21 s | 10.96 s | 52x | yes: 16,592 vs 16,376 conflicts |
| `r3sat250.cnf` (SAT core) | 7.13 s | >60 s | >8x | not measured |
| `lia_big_parse` (4,000 vars) | 1.08 s | 31.05 s | 29x | yes, no search at all |
| `php8` | 0.30 s | 1.49 s | 5x | yes: 17,975 vs 18,263 conflicts |
| `r3sat180.smt2` | 0.05 s | 0.35 s | 7x | roughly: 6,517 vs 4,992 conflicts |
| `quant_f` | 0.00 s | 0.46 s | >100x | no: 528 vs 1,105 bool vars |
| `quant_lra_lia_solver.smt2` | 0.02 s | 6.27 s | ~260x | see P9 |
| `arr_chain` | 0.00 s | 3.24 s | >500x | no: 114 vs 757 decisions |
| `ScopeRestoration/SurvivingAtoms` | 480 ns | 6,842 ns | 14x | same operation |
| `ScopeRestoration/ScopedAtoms` | 13,000 ns | 179,011 ns | 14x | same operation |
| `lra` | >60 s | 0.41 s | Go wins | C++ validated Go's model: real win |
| `ufdiamond`, N=120 | >60 s | 0.87 s | **not a win** | caused by bug B1 |

`lia_big_parse` is quadratic in both implementations (n=4,000/8,000: C++ 1.05/4.19 s,
Go 31.7/107 s), so the algorithm matches and the 25-30x is constant factor.

### Effect of each fix

Best of 3 runs, in seconds, each patch applied alone. The baseline is the "fixed"
build containing the B1 candidate, so these search paths include correct lemmas.

| Workload | baseline | P-a SAT words | P-b AST lock | P-c Propagate | P-d rebuild | all four | `GOGC=400` | all + `GOGC=400` |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `r3sat190.cnf` | 10.94 | **2.02** | | | | 1.98 | | |
| `php8` | 1.55 | | 1.55 | **1.13** | 1.51 | 1.07 | 1.33 | **0.85** |
| `r3sat180.smt2` | 0.38 | | 0.31 | 0.33 | 0.33 | 0.29 | 0.26 | **0.18** |
| `ud16` | 11.79 | | 11.67 | 10.69 | 11.25 | 10.56 | 7.92 | **7.25** |
| `lia_big_parse` | 31.31 | | **26.21** | 31.68 | 31.90 | 27.26 | 24.15 | **20.09** |

Differences of a few percent are within noise on individual cells. Search statistics
for `php8`, `r3sat180` and `ud12` were identical with and without all four patches.

Result on the committed code is recorded in "After the commits" below.

## Fixes committed on this branch

- **P-a. SAT clause words** (`sat/sat_clause_storage.go`, `arena/small_bytes.go`).
  `readWord` called `validate` and then read four bytes separately, each through a
  full descriptor lookup. `Literal` additionally re-read size and capacity. So one
  literal read cost three word reads, each with five validations. It now does one
  validated `ReadUint32LE`. About 5x. Done, tests pass.
- **P-b. AST mutex removed** (`ast/ast_core_storage.go`). `lock()` took a mutex on
  every accessor, which was 23% of `lia_big_parse`. The field is now a no-op type, so
  the existing `defer m.mu.Unlock()` call sites are unchanged for now. I checked the
  asynchronous paths (`shell/*` signal goroutines, `control.ScopedTimer`,
  `runCombinedTimed`): the ones I read cancel through `Reslimit`, whose cancel count
  is atomic, and do not take this mutex. **Consequence:** `ASTManager` is now strictly
  confined to one goroutine, including for library users of `pkg/api`.
- **P-c. `Propagate`** (`smtcore/smt_boolean_propagation.go`). Reads `s.assignment`
  directly rather than through `Assignment`, which re-checked the owner per literal,
  and checks `clause.open()` only in debug builds. An invalid literal now panics
  rather than returning an error.
- **P-d. `rebuildCongruence`** (`smtcore/smt_equality_propagation.go`). Replaced a
  per-enode `map` allocation with a scan over the argument list. Small gain. P1
  replaces this function altogether.

Rejected:

- Removing only the `GetKind` validation from `SMTCheckerContext.SetAssignment`
  gave no measurable gain. The map write is the cost, not the check.
- Removing the `assignments` map write entirely made `quant_f` 8x faster (0.46 to
  0.06 s), but other code reads that map (`smt_quantifier_manager.go`,
  `smt_relevancy.go`), so behavior changed. Do it properly as part of P4.

## Open performance items, highest value first

### P1. Replace `rebuildCongruence` with a trail undo

`popTo` rebuilds parent lists and the whole congruence table from every enode
whenever a popped scope contained a merge, so each backjump is O(all enodes). It was
27% of `ud16`. C++ `context::undo_add_eq` (`src/smt/smt_context.cpp`) restores only
the affected class, using `add_eq_trail(r1, n1, r2_num_parents)`.

- **Fix:** port the C++ trail. Record `r1`, `n1` and `r2`'s parent count, then in
  undo restore roots, truncate `r2.parents`, and reinsert only `r1`'s parents.
- **Test:** benchmark a pop over K merges with N unrelated enodes, and assert the
  cost does not grow with N. Existing `SCOPE_RESTORATION_PLAN.md` tests cover the
  semantics. Also check the congruence table contents match a full rebuild.

### P2. Replace per-merge `equalitySnapshot`

`addEquality` allocates a snapshot of the whole smaller class on every merge and
clones each node's theory-variable list, then clones again on restore. C++ stores a
fixed-size trail entry and walks the class to undo. This goes together with P1.

- **Test:** `testing.AllocsPerRun` on a merge/pop cycle should be near 0.

### P3. Remove O(D) disequality handling

- `PushScope` copies both `disequalities` and `disequalityReasons` on **every
  decision**.
- `remapDisequalities` scans the whole map on every merge.
- `SMTArrayTheory.hasRelevantDisequality` scans **all Boolean variables** per
  candidate pair.

C++ has no such table. `context::is_diseq` is an O(1) congruence-table lookup of the
equality atom. Porting that would also remove the code containing bug B1.

- **Test:** an N-disequalities benchmark where push and merge cost do not grow with
  N, plus the B1 regression test below.

### P4. Dense arrays in place of handle-keyed maps

C++ indexes by `expr->get_id()`. These Go maps are on hot paths, with 16-byte keys:
`boolVar`, `context.enodes`, `context.assignments`, `context.relevant`,
`cachedGeneration`, and the relevancy watch maps. `SetAssignment` does a map write on
every assignment and every undo. There are 1,972 `map[ast.ASTHandle]` sites overall,
343 in `smtcore`, but start with those listed.

- **Fix:** slices indexed by `h.Slot()`, with a generation check. A handle's slot is
  stable while its generation is live.
- **Test:** compare `-st` statistics on the workloads before and after. Search must
  not change. `BenchmarkSMTRelevanceCallbacks` should show the gain.

### P5. Batched AST reads

`lia_big_parse` spends 43% under `rewriterEngine.processApp`, nearly all in small
accessors. `MkBoolVar` makes 5 or more such calls per expression, and each repeats
`lock`, handle validation, and `defer`. Accessor call sites: `muz` 1,631, `tactic`
1,356, `smtcore` 1,244, `qe` 639, `rewriter` 616.

- **Fix:** add a method returning one struct with kind, decl, family, decl kind and
  arg count. Add `Args(h, buf)` to read all args at once. `readInternal` makes two
  validated arena reads per argument, so arguments should come out as a block.
- **Fix:** `rewriterCore.open()` calls `ProofsEnabled()`, taking the lock path on each
  operation. Proof mode is fixed at construction, so cache it. Same in
  `makeClause` and `resolve`.
- **Test:** `lia_big_parse` timing, and equivalence tests against the single accessors.

### P6. Remaining SAT core overhead

Still about 10x C++ after P-a (2.1 s vs 0.21 s). `propagateCore` still does a
descriptor lookup per clause word, and validates the watch cursor (`store.open`,
backing liveness, bounds) on every read and write. `valueLiteral` and `assignCore`
call `open()`.

- **Fix:** take a `[]uint32` view of the clause once per visit, and iterate
  `backing.data` as a local slice, revalidating only after `assignCore`-type calls
  that can reallocate. Check the view is not retained across compaction.
- **Test:** `-dimacs -st` statistics must be unchanged. Target `r3sat190` under 0.5 s.

### P7. GC tuning

No code sets `GOGC` or a memory limit. `GOGC=400` gave 14-33% across the table above.

- **Fix:** `debug.SetGCPercent(400)` in `cmd/g3/main.go`, respecting a user-set
  `GOGC`. Not committed because it was only evaluated through the environment variable,
  and **its memory cost was not measured**. Measure RSS first, and do not set it in
  library code.
- Longer term: `SMTEnode`, `SMTClause` and `SMTBooleanJustification` hold many
  pointers, and fewer pointers would reduce mark work.

### P8. Small-value arithmetic

- `math/simplex.Infinitesimal` is three `big.Rat`s. `smtLinearForm` is
  `map[uint32]*big.Rat`. There are 234 `big` uses in `math/simplex` and 745 in
  `smtcore`. C++ uses `mpq` with an inline small representation. Go has that design
  in `util/numeral` (`Int{Val, Cell}`), but simplex does not use it.
- `smtArithmeticCoefficientKey` builds a sorted string key with `RatString` per atom
  and was 17% of `arr_chain`. Replace it with a hash over sorted column/numerator/
  denominator values, or intern linear forms.
- `assignedArithmeticBoundCandidates` scans all assertions for each column.
- `IntegerManager.Add`/`Mul` run `check` on three operands, plus a `defer` closure
  for tracing, per operation. Put tracing behind a build tag, and fast-path small
  operands before validation.
- **Test:** an arithmetic-heavy benchmark. Results need to agree exactly with
  existing `big.Rat` values in a differential test, especially at int32 overflow.

### P9. Search and instantiation divergence

Some gaps are about doing different work, not doing it slower:

- `quant_f`: 1,105 vs 528 bool vars, 369 vs 159 arithmetic bounds, and Go lacks the
  `arith-fixed-eqs` that C++ reports.
- `arr_chain`: 757 vs 114 decisions, 7 vs 2 conflicts.
- `(forall ((x Real)) (exists ((y Real)) (> y x)))`: C++ 0.00 s, Go 3.82 s, with Go's
  time in nested MBQI solving.

Root cause not determined. The way to find it is to diff `-st` output and then instance
traces on the smallest file. Matching C++ is not a goal in itself: `lra` shows Go can be
legitimately faster. But each difference should be checked with model validation.

### P10. Conflict resolution allocations

`resolve` allocates O(V) per conflict: `levels()` copies a level for every Boolean,
`marks` is a fresh slice, and `syncENodeLiterals` walks every variable with a map
write. C++ keeps persistent mark arrays. `TransReaches` and `smtCommonTransAncestor`
allocate maps per equality, and `cleanup` allocates a new `processed` map per expansion.
It was about 15% of `ud16`.

- **Fix:** persistent slices reused between calls, using `booleanData[v].ScopeLevel`
  directly and marks on the enode.
- **Test:** `BenchmarkSMTConflictResolution` should reach 0 allocs/op.

### P11. Smaller items

- `SMTClause`: pointer to struct to slice to array is one more cache miss than C++'s
  inline literals. `SMTBooleanJustification` is about 40 bytes (C++ `b_justification`
  is one tagged word), inflating `SMTBoolVarData` and the assignment trail.
- `detachCongruenceParents` allocates a map per merge. `mergeTheoryVariables` allocates
  closures per merge.
- `smtCongruenceSignature` builds a string key for 3+ argument terms, and `validateNode`
  makes accessor calls per insert and find.
- `satClauseObjectBytes` calls `reflect.TypeOf` on every clause allocation. Use
  `unsafe.Sizeof` or a constant. There are 705 `reflect` uses, mostly in cold paths.
- `elim_unconstrained` is quadratic on `lia_big_parse` in C++ too. An algorithmic fix
  would beat both, but would diverge from source.

## Benchmark harness problems

The Makefile's CPU table overstates some costs:

- `b.StopTimer`/`b.StartTimer` with `b.ReportAllocs` calls `ReadMemStats`, which stops
  the world. That was 30% of total samples in `BenchmarkSMTBooleanPropagation`.
- `benchmarkMust` calls `b.Helper()` in loops, which was 26.7% of samples in
  `BenchmarkSMTScopeRestoration` through stack walking.
- So process-wide profile rows include a lot of untimed work, and only
  `BenchmarkSMTScopeRestoration` has a C++ twin that times the same thing. Use
  `-focus` when reading profiles, and drop `b.Helper()` from `benchmarkMust`.

## Bugs

### B1. False `unsat`: disequality conflicts omit antecedents (critical, open)

```smt2
(declare-const a (Array Int Int))
(declare-const b (Array Int Int))
(declare-const i0 Int)
(declare-const i1 Int)
(declare-const i2 Int)
(assert (not (= (store b i2 i0) (store a i1 i0))))
(assert (= (store b i2 i1) a))
(check-sat)
```

C++ says `sat`, g3 says `unsat`. It is plainly satisfiable. On the larger source formula, I checked C++'s model by
hand, and pinning its key values still gives `unsat` in g3. In 500
random array formulas, 3 were false `unsat`. `(get-proof)` ends in `false` with
undischarged `hypothesis` terms.

**Cause.** `addEquality` checks `isDisequal(r1, r2)` eagerly, which is a Go addition
with no C++ equivalent. `SMTDisequalityConflictJustification.MarkAntecedents` then marks
only the final equality. It omits the proof paths `First -> r1`, `Second -> r2`, and
those from the disequality atom's terms to their roots. The conflict therefore appears
to be at a lower level than it is. At the base level, `resolve` reports `unsat`. Above
it, the learned lemma is too strong.

**This is not confined to arrays.** Over-strong lemmas also occur in pure QF_UF, which
explains the diamond result: g3 used 23 conflicts at N=20 where C++ used 1,050,964.
With the candidate fix g3 tracks C++ (N=8: 264 vs 264; N=12: 4,275 vs 4,200; N=16:
66,332 vs 65,868). **So that speedup is the bug and any `unsat` from g3 on a formula
with equalities should be treated with suspicion until this is fixed.** My fuzzing found
no false `unsat` in 500 UF formulas and 1,500 near-diamonds, which shows only that they
are hard to hit with those generators. Any `sat` model is unaffected.

**Candidate fix.** It fixes all 3 array cases, leaves the rest of the differential fuzz
unchanged except as listed under B2, and passes `go test -short ./smtcore`:

```go
// smt_justification.go: add these fields to the struct
DiseqLeft, DiseqRight *SMTEnode

// SMTDisequalityConflictJustification.MarkAntecedents
if j.First.root == nil || j.Second.root == nil {
	return ast.ErrASTDomain
}
for _, node := range []*SMTEnode{j.First, j.Second, j.DiseqLeft, j.DiseqRight} {
	if node == nil {
		continue
	}
	if err := marker.MarkEquality(node, node.root); err != nil {
		return err
	}
}
return marker.MarkJustifiedEquality(j.First, j.Second, j.Equality)

// smt_equality_propagation.go, in addEquality's disequal branch
key, _ := smtCheckerPair(r1, r2)
reason := s.disequalityReasons[key]
if proof.DiseqLeft, proof.DiseqRight, err = s.disequalityAtomTerms(reason); err != nil {
	return err
}
return s.setEqualityConflict(proof, reason)

// smt_equality_propagation.go, new helper
func (s *SMTInternalizerState) disequalityAtomTerms(reason SMTLiteral) (*SMTEnode, *SMTEnode, error) {
	variable := reason.Var()
	if variable < SMTFirstBoolVar || int(variable) >= len(s.boolExpr) {
		return nil, nil, nil
	}
	atom := s.boolExpr[variable]
	count, err := s.manager.GetNumArgs(atom)
	if err != nil || count != 2 {
		return nil, nil, err
	}
	first, err := s.manager.GetArg(atom, 0)
	if err != nil {
		return nil, nil, err
	}
	second, err := s.manager.GetArg(atom, 1)
	if err != nil {
		return nil, nil, err
	}
	return s.context.enodes[first], s.context.enodes[second], nil
}
```

**Blocker: why this is not committed.** With this patch
`cmd/g3/test_vectors/quant_lra_lia_solver.smt2` does not terminate (still running after
22 minutes in the test suite, against 6.3 s before and 0.02 s in C++). The minimal case:

```smt2
(set-logic LRA)
(assert (forall ((x Real)) (exists ((y Real)) (> y x))))
(check-sat)
```

The stack is `CheckModel -> checkQuantifier -> auxiliaryCounterexamples -> nested
search -> propagateArithmeticBoundAtoms`. I did not determine the cause. The likely
reading is that MBQI on this file was only terminating because of the too-strong
lemmas, but that is unverified. Start by comparing with what C++ does in 1 conflict and
0.00 s (see P9), since fixing that would likely remove both the 6 s and the hang.

Also worth considering: fix B1 by removing the eager check entirely and using C++'s
`is_diseq` design (P3). That removes this code path, rather than repairing it.

**Regression test.** Put the formula above and these in `cmd/g3/test_vectors/`, and
assert `sat` through `run()`:

```smt2
(declare-const a (Array Int Int))
(declare-const b (Array Int Int))
(declare-const i0 Int)
(declare-const i1 Int)
(declare-const i2 Int)
(declare-const i3 Int)
(assert (or (= (select b i0) i3) (not (= (store a i0 i0) (store (store a i0 i3) i2 i3)))))
(assert (or (= (store (store b i1 i1) i0 i2) (store b i2 i1))))
(assert (or (= (select a i0) i3)))
(assert (or (not (= (store b i0 i1) b))))
(check-sat)
```

I confirmed these fail without the patch and pass with it. Also add a test that UF
diamond conflict counts grow exponentially, as a guard against over-strong lemmas
returning, and validate learned lemmas in a debug build: each must be implied by the
conflict's full antecedent set.

### B2. `sat` with an invalid quantifier model (open)

```smt2
(set-logic UFLIA)
(declare-fun f (Int) Int)
(declare-fun g (Int Int) Int)
(declare-const c Int)
(declare-const d Int)
(assert (forall ((x Int)) (! (= (f x) (g x x)) :pattern ((f x)))))
(assert (forall ((x Int)) (! (<= (f x) (+ x 0)) :pattern ((f x)))))
(assert (forall ((x Int)) (! (>= (g x x) (g x d)) :pattern ((f x)))))
(assert (distinct (g c d) (f 1)))
(assert (<= (g d c) c))
(assert (distinct d (+ 1 2)))
(check-sat)
(eval (f 10))
(eval (g 10 10))
```

g3 says `sat`, then evaluates `(f 10)` to 10 and `(g 10 10)` to 1, violating the first
assertion. C++ says `unknown`. `sat` is the right verdict (for example `g(x,y)=x-1`,
`c=5`, `d=0`), but the model is wrong, and `model_validate=true` does not catch it. `f`'s
printed interpretation has an `x!0` default without the projection `g` has. Cause not
determined; start in `smt_model_finder*.go`. With the B1 patch this input instead runs
past the 8 s limit used in fuzzing.

- **Test:** evaluate each assertion under the returned model on random `x`. Make this
  a general check on MBQI models in debug builds.

### B3. `-st` memory figures are wrong (minor, open)

`:memory` and `:max-memory` always show 0.01 MB against 34-77 MB RSS, and `:num-allocs`
shows 2 versus C++'s 3,245,211. `cmd_context/apply_tactic_statistics.go` reports a
`runtime` allocator that hardly any allocation passes through. **Fix:** use
`runtime.ReadMemStats` for these fields at statistics time. **Test:** allocate a known
amount and check the reported figure reflects it.

### Differences checked and not treated as bugs

- **Fuzz `dt238`: C++ `unsat`, g3 `sat`.** I checked g3's result by hand, with
  `t0 = nil`, `tl(nil) = nil`, `hd(nil) = 5`, `n0 = n1 = n2 = 0`,
  `t1 = cons 1 (cons 2437 nil)`: all five assertions hold given selectors unconstrained
  on the wrong constructor. So g3 looks right under SMT-LIB semantics and C++ 4.7.1 wrong.
- **`mk-bool-var` differs by 2** on purely Boolean inputs. Seen, not investigated. Probably
  reserved variables.
- **Search differs slightly** between implementations even on the SAT core (16,592 vs
  16,376 conflicts). Not traced. Candidates are sort stability and hash iteration order.
  It limits how exactly per-conflict costs can be compared.

## Differential fuzz results

3,500 random small formulas (500 each of UF, LIA, LRA, BV, arrays, datatypes,
quantifiers), comparing `check-sat` against C++, 8 s limit each:

| | mismatches | which |
|---|---:|---|
| unpatched | 5 | 3 false `unsat` on arrays (B1), `dt238`, `q433` (B2) |
| with B1 patch | 2 | `dt238`, `q433` (now times out) |

Plus 1,500 near-diamonds with no mismatches on either build. This compared verdicts, not
models, and does not establish correctness. BV and LIA instances were small.

## Reproducing

Build a profiling g3 without touching the tree, using `go build -overlay` to wrap `main`
with `pprof.StartCPUProfile`. Generators:

- `php8`: pigeonhole, 9 into 8, QF_UF.
- `r3satN`: random 3-SAT, ratio 4.26, `random.seed(42)`, as `.smt2` and DIMACS.
- `udN`, `ufdiamond`: chain of `(or (and (= ai bi) (= bi ai+1)) (and (= ai ci) (= ci ai+1)))`
  with `(not (= a0 aN))`, plus side constraints on `f` and `g`.
- `lia_big_parse`: 4,000 ints, `(<= (+ vi (i%17)) (+ vi+1 (i%13)))`.
- `lra`: 120 reals, 260 clauses `(or (<= t c1) (>= t c2))`, each `t` having 5 random
  coefficients in [-9, 9], `random.seed(7)` after the scheduling workload.
- `arr_chain`: 60 nested `store`s with index-equality clauses.
- `quant_f`: `f(x+1)=f(x)+2` with pattern `(f x)`, and 50 array reads.

Only the formulas quoted above are in this file. The generator scripts were temporary.

## Status on branch `perf-plan` (2026-11-07)

Done and committed, each with unit tests. Unless noted, search statistics were checked unchanged.

- **B1 fixed** (`a031acd63`). The hang that blocked it was a strategy gap: LRA, LIA, NIA and
  logic-free input now reach QSAT. **B2** no longer occurs (it was a consequence of B1).
  **B3** fixed: real heap figures.
- **B4 (new) fixed:** identical inputs gave different searches across runs, from Go's random
  map order in the model finder, Ackermannization and reference-release loops. 669/669 test
  vectors now repeat at 8 runs. Not swept: about 80 similar release loops in `muz` and
  `duality`.
- **P1/P2** trail undo; **P3** per-root disequalities; **P4** dense expression table;
  **P5** `NodeHeader`; **P6** SAT clause views; **P7** `GOGC=400` in the CLI;
  **P10/P11** allocation-free conflict resolution and arity-2 congruence keys.
- **P8:** cached atom keys and bounds, column index, zero-skipping `Infinitesimal`, endpoint
  sign check, and small-integer fast path. Arithmetic now adds atom rows and undoes scopes on
  the live tableau, and this one *does* change search: checked against C++ on 1,200
  push/pop scripts with model validation. Not done: replacing `big.Rat` in simplex.

Measured: `r3sat190` 10.96 s → 0.95 s (C++ 0.21); `ud16` 11.8 s → 1.9 s, with B1's correct
lemmas (C++ 0.48); `arr_chain` 3.3 s → 0.29 s; MBQI stress `qbi` 32.5 s → 5.6 s.

### Open

- **P9, new bug B5:** `(assert (forall ((x Int)) (! (= (f (+ x 1)) (+ (f x) 2)) :pattern ((f x)))))`
  with `(assert (= (f 0) 0))`, `(assert (not (= (f 3) 6)))` under `(set-logic AUFLIA)` never
  terminates. This predates the audit, and C++ solves it instantly. Cause: C++
  `theory_arith::fixed_var_eh` propagates equalities between fixed variables while bounds are
  asserted. Go only does it in `FinalCheck` (`propagateFixedSharedValues`), so eager E-matching
  runs forever. Calling `propagateFixedSharedValues` from `Propagate` fixes it (verified for
  N=3..20) but makes `quant_f` 16x and `qbi` 6x slower, because it replays a separate solver
  and its derived bounds disable the incremental tableau. Fix properly by detecting newly fixed
  columns on the live solver as C++ does. Not committed.
- The full `make` suite has not run since commit `9a52bf219`. Run it before merging.
- Remaining P11: `reflect.TypeOf` per clause allocation, `elim_unconstrained`'s quadratic rounds
  (`lia_big_parse` is still about 27 s against 1 s in C++).

### B5 update

Fixed by the `theory_arith` port (ARITH_PORT.md): with `G3_THEORY_ARITH=port`
the AUFLIA goal is refuted at once, as in C++, because `fixed_var_eh` runs
from `set_bound`. The replay theory, still the default, does not terminate.
Regression test: `cmd/g3/theory_arith_port_test.go`.
