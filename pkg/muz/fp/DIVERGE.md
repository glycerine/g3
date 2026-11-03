# Recursive QBMC: C++ and Go divergence audit

This audit follows the live linear, quantified BMC path exercised by
`TestQBMCRecursiveRuleSelection` in `bmc_numeric_proof_test.go`. It compares
`src/muz/bmc/dl_bmc_engine.cpp` with the Go fixedpoint facade and
`pkg/muz/bmc/quantified_linear_*`. It does not claim a complete audit of the
underlying SMT solver or model evaluator.

The fixture has `leaf(0)`, `leaf(x) && x < 3 -> leaf(x+1)`, and
`leaf(3) -> goal`. One native run returned
`bmc: model has no selected quantified rule at level 0` from the query, before
proof validation. The result is not a demonstrated logical counterexample: the
run did not record the model, predicate, selector values, or whether the first
or second Go traversal failed. The failure did not recur in 20 isolated
recursive runs or 30 complete numeric-proof runs. The cause is **unknown**.

The standalone recursive QBMC test and its matrix entry have been re-enabled
to match the corrected C++ regression and independently check the proof.

## Differences that could affect rule selection

1. **FIXED — Solver scope when reading the model.** The public QBMC path now
   selects rules and constructs the proof while the solver scope remains
   active, matching `src/muz/bmc/dl_bmc_engine.cpp:51–70`. The in-scope
   consumer preserves cleanup and model ownership on error; focused native
   and three-tag scope and recursive-proof tests pass.

2. **FIXED — One selection pass versus two.** The public QBMC path now
   selects each rule once and builds its proof step during the same traversal,
   matching `src/muz/bmc/dl_bmc_engine.cpp:269–344`. Trace and proof share the
   selected rules instead of independently evaluating the model twice.

3. **FIXED — Evaluator lifetime and completion.** Trace and binding model
   evaluations now use a fresh evaluator with completion explicitly disabled
   for each term, matching `src/model/model.cpp:79–89`. Focused tests cover
   a nondefault global completion setting and the recursive proof path.

4. **FIXED — Group assertion order.** C++ compiles predicate groups by iterating its
   `decl2rules` map (`src/muz/base/dl_rule_set.h:159,270`,
   `src/muz/bmc/dl_bmc_engine.cpp:86–103`). Go now records the source predicate-group bucket
   traversal using the existing declaration AST hash, including growth and
   groups emptied by removal. Focused RuleSet and QBMC compiler-order tests
   pass; the previous compiler fails the order regression. The effect on the
   intermittent failure remains unproven. Reset capacity retention and sparse
   table shrink behavior also match source and have focused tests.

5. **FIXED — Query-predicate check and failure reporting.** Go now
   performs the source debug assertions for the query predicate at `T` and
   a missing selector, catches the same typed rewriter exception and emits
   source VERIFY diagnostics for an invalid `T`. Focused native and three-tag
   tests pass. If execution continues beyond an invalid source assertion, Go
   reports an explicit error instead of reproducing C++ undefined behavior.

## Differences after a rule has been selected

6. **FIXED — Index used for proof-variable values.** Go evaluates indexed
   rule variables at the trace level, matching the corrected C++ implementation.
   A focused test distinguishes this from the rule ordinal.

7. **FIXED — Uninterpreted binding values.** Go now preserves non-null
   symbolic evaluation results, matching `src/muz/bmc/dl_bmc_engine.cpp:288–297`.
   A direct test checks this with model completion disabled. These source
   semantics can expose the source proof failures described below.

8. **FIXED — Termination of the selected trace.** Go accepts a selected fact
   at any level and verifies that a non-fact has a positive level, matching
   the corrected C++ extractor. Focused tests cover both cases.

9. **FIXED — Proof resolution and normalization.** Both Go QBMC proof paths
   use corrected C++ hyper-resolution positions `(1, 0)`. The re-enabled
   recursive and numeric proof tests check proof structure, side conditions
   and original premises. Symbolic model bindings can still expose a separate
   source-compatible proof-checking failure, described below.

## Next evidence to capture

On the next failure, capture the trace-versus-proof pass, bit width, query
predicate and current predicate, level, rule ordinals and each evaluated
selector (including whether it is false or symbolic), plus `T` and the model's
relevant interpretations. Compare the same model before and after the solver
pop and evaluate selectors with a fresh completion-disabled evaluator. Record
actual group/assertion order. These comparisons can distinguish model content,
evaluator state, and ordering hypotheses without guessing a fix or weakening
the independent proof checks.

## Integration findings

The later C++ fixes use the trace level for bindings, stop at a selected fact
regardless of level, and correct hyper-resolution positions. The Go recursive
proof regression is re-enabled, including independent checks of its proof.
Internal substitution accepts symbolic rule heads as the C++ Release path
does while public rule ingestion still validates heads. The existing
`TestBMCQuantifiedLinearProofWithUninterpretedBinding` remains red: with a
symbolic binding, the source-shaped first proof conclusion stays quantified
while the instantiated rule has no variables, leaving the next proof step
without the substitution needed by the independent checker.
