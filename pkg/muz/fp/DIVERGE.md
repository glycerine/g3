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

The standalone recursive QBMC test and its corresponding matrix entry are
currently commented out as requested; the test body and independent proof
checks remain in `pkg/muz/fp/bmc_numeric_proof_test.go` with the reason.

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

6. **FIXED — Index used for proof-variable values.** Go now evaluates
   indexed rule-variable terms at the selected rule ordinal, as C++ does
   (`src/muz/bmc/dl_bmc_engine.cpp:273–290`). A direct test distinguishes
   the ordinal from the derivation level under native and three-tag builds.

7. **FIXED — Uninterpreted binding values.** Go now preserves non-null
   symbolic evaluation results, matching `src/muz/bmc/dl_bmc_engine.cpp:288–297`.
   A direct test checks this with model completion disabled. These source
   semantics can expose the source proof failures described below.

8. **FIXED — Termination of the selected trace.** Go now stops at level zero
   and requires a single predecessor at positive levels, matching the source
   assumptions (`src/muz/bmc/dl_bmc_engine.cpp:347–354`). Go reports an AST
   precondition error when those assumptions are violated. A focused test
   exercises facts selected at levels zero and one. The recursive QBMC proof
   test has been commented out by request; other proof tests also fail when
   the solver selects a source-valid positive-level fact.

9. **FIXED — Proof resolution and normalization.** QBMC now follows the
   C++ proof sequence: normalize before hyper-resolution, use the original
   first-rule conclusion and source resolution position, and omit the extra
   variable-bank substitution composition. Proof mode starts after
   substitution and unification and is restored after retryable cleanup.
   Focused source-conclusion and native cleanup tests pass; source-compatible
   proof failures remain in the existing integration tests.

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

The source encoding permits facts at positive levels, but source trace
extraction assumes every positive-level rule has a predecessor. An actual Go
model had T=10 and reached the base fact at level 5; bundled Z3 verified that
model against all source constraints. Source termination has been restored by
request; the recursive proof case is commented out and other proof tests can
fail on positive-level facts. No extra encoding constraint was added.
The bundled Z3 4.7.1 also fails its rule-unifier verification on the recursive
and linear fixtures. Continuing beyond a failed source assertion can invoke undefined C++ behavior; Go reports an explicit error at that boundary.

After these changes, a focused three-tag integration run still fails
`TestBMCQuantifiedLinearProofWithRuleVariables`,
`TestBMCQuantifiedLinearProofWithUninterpretedBinding`, and
`TestBMCNumericProofIndependentValidation/qbmc/linear`. These tests remain
enabled; the user explicitly authorized matching source despite red proofs.
The separate `TestPDRInputTrace` regression from `pkg/log.red` passes with
all three tags after its context-ownership accounting fix.
