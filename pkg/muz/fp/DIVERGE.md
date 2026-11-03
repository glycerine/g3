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

From `pkg/`, repeat the exact fixture and its independent proof checks with:

```sh
go test ./muz/fp -run '^TestQBMCRecursiveRuleSelection$' -count=100
```

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
   `src/muz/bmc/dl_bmc_engine.cpp:86–103`). Go scans the rule set and compiles
   each head at its first appearance (`pkg/muz/bmc/quantified_linear_compile.go:37–58`).
   The group traversal order can differ even when the resulting implications
   are logically equivalent. Go now records the source predicate-group bucket
   traversal using the existing declaration AST hash, including growth and
   groups emptied by removal. Focused RuleSet and QBMC compiler-order tests
   pass; the previous compiler fails the order regression. The effect on the
   intermittent failure remains unproven. Reset capacity retention and sparse
   table shrink behavior also match source and have focused tests.

5. **Query-predicate check and failure reporting.** C++ has a debug assertion
   that the indexed query predicate evaluates true at the chosen level before
   it walks rules, followed by `SASSERT(r)` for a missing selected rule
   (`src/muz/bmc/dl_bmc_engine.cpp:260–265,275–284`). Go checks that `T` is a
   representable bit-vector numeral but does not make that same predicate
   assertion; it returns the observed explicit error if no rule selector is
   true (`pkg/muz/bmc/quantified_linear_trace.go:61–77,95–133`). C++ also
   ignores the boolean return from `model::eval` in `eval_q`, whereas Go
   propagates evaluator errors. This explains the difference in diagnostics,
   not why the Go selector was non-true. A source assertion or subsequent
   dereference is not evidence that C++ would return the same error.

## Differences after a rule has been selected

6. **Index used for proof-variable values.** C++ passes the selected rule's
   ordinal `i` to `eval_q` for every rule-variable term
   (`src/muz/bmc/dl_bmc_engine.cpp:273–290`). Go substitutes the selected
   trace step's **level** (`pkg/muz/bmc/quantified_linear_proof.go:40–46`,
   `pkg/muz/bmc/quantified_linear_bind.go:44–64`). These differ when the
   ordinal and level differ. This can change proof bindings; the C++ choice
   may itself be a defect and should not be silently copied without checking
   proof validity. It happens after selecting a rule, so it does not directly
   explain the reported missing selector at that step.

7. **Uninterpreted binding values.** C++ keeps a non-null evaluation result
   and falls back to the original variable only for a null result
   (`src/muz/bmc/dl_bmc_engine.cpp:288–297`). Go falls back when the result is
   not an AST value (`pkg/muz/bmc/quantified_linear_bind.go:70–96`). This may
   affect proof substitutions for partially interpreted models; it occurs
   after rule selection and is not established as the reported cause.

8. **FIXED — Termination of the selected trace.** Go now stops at level zero
   and requires a single predecessor at positive levels, matching the source
   assumptions (`src/muz/bmc/dl_bmc_engine.cpp:347–354`). Go reports an AST
   precondition error when those assumptions are violated. A focused test
   exercises facts selected at levels zero and one. The recursive QBMC proof
   test has been commented out by request; other proof tests also fail when
   the solver selects a source-valid positive-level fact.

9. **Proof resolution and normalization.** C++ applies the rule unifier and
   builds a hyper-resolution proof for the resulting formula directly
   (`src/muz/bmc/dl_bmc_engine.cpp:303–327`). Go resolves a raw rule, composes
   its variable-bank renaming into the premise substitutions, proves that raw
   resolvent and then normalizes the rule with explicit proof steps
   (`pkg/muz/bmc/linear_proof.go:212–245,265–285`). This can change proof
   structure and normalization behavior after selection. It is not evidence
   for the earlier missing-selector error.

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
and linear fixtures. Items 6, 7 and 9 remain open pending resolution of these source
inconsistencies and proof-normalization dependencies. Go enters proof mode
before the whole proof step; C++ does so after substitution and unification.
