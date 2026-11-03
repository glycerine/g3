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

1. **Solver scope when reading the model.** C++ `qlinear::check` gets the
   model, selects rules and builds the proof before popping the solver scope
   (`src/muz/bmc/dl_bmc_engine.cpp:51–70,247–285`). Go acquires a counted
   `ModelCore`, closes the attempt and pops the scope before returning from
   `SearchQuantifiedLinear` (`pkg/muz/bmc/quantified_linear_search.go:44–51,103–114`,
   `pkg/muz/bmc/quantified_linear_search_cleanup.go:38–68,121–129`). The facade
   then evaluates the model (`pkg/muz/fp/bmc_engine.go:280–316`). The owner
   reference keeps the model alive, but whether a pop changes any relevant
   model interpretation or evaluator input has not been established. This is
   a timing difference and a candidate, not an identified cause.

2. **One selection pass versus two.** C++ selects each rule, records the
   trace and assembles the proof in one `get_model` loop
   (`src/muz/bmc/dl_bmc_engine.cpp:269–344`). Go first calls
   `QuantifiedLinearTrace`, then `QuantifiedLinearProof`; each calls
   `quantifiedLinearSteps`, reevaluating `T` and the rule selectors with a new
   evaluator (`pkg/muz/fp/bmc_engine.go:307–317`,
   `pkg/muz/bmc/quantified_linear_trace.go:16–17,34–48`,
   `pkg/muz/bmc/quantified_linear_proof.go:40–46`). The reported error does
   not say which pass failed. Repeated evaluation could matter if model or
   evaluator state changes, but no such change has been demonstrated.

3. **Evaluator lifetime and completion.** C++ `model::eval` creates a fresh
   evaluator for each call and explicitly sets model completion to its
   default argument, false (`src/model/model.cpp:79–89`,
   `src/model/model.h:45`, `src/muz/bmc/dl_bmc_engine.cpp:229–244`). Go creates
   one evaluator per trace pass and reuses it for `T` and all selectors
   (`pkg/muz/bmc/quantified_linear_trace.go:48–107`). Its completion setting
   comes from the model-evaluator parameter view, whose default is also false
   (`pkg/model/model_evaluator.go:459–468`); a nondefault completion setting
   can therefore differ from the C++ call. Evaluator cache or state effects
   with the default fixture are unproven.

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

8. **Termination of the selected trace.** C++ stops at level zero, asserting
   a fact there, and otherwise assumes a single predecessor
   (`src/muz/bmc/dl_bmc_engine.cpp:347–354`). Go stops as soon as a selected
   rule has no uninterpreted tails, even above zero; it reports an error for a
   non-fact at level zero or multiple tails
   (`pkg/muz/bmc/quantified_linear_trace.go:138–152`). A fact selected at a
   positive level can therefore lead to different trace/proof behavior. This
   difference alone does not explain failure before a level-zero rule is
   selected.

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
