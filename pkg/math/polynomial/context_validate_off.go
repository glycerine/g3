//go:build !g3checked && !g3debug

package polynomial

// polynomialValidateOwnership selects whether the hot allocation-context
// validators (henselCallerFrame, henselPairReceipt, the arithmetic roots,
// trialFrame, knuthFrame) re-test the pure ownership and
// identity links of a frame: manager self pointers, the integer manager a
// frame was built for, and which polynomial managers a frame owns. Those
// links are fixed when a frame is constructed and can only disagree after a
// caller bug, which checked/debug builds still report. Every state test
// (invocation numbers, active/executing/constructed, scope and frame-list
// membership, callback depth) runs in all builds, as numericValidateCells
// does for digit access.
const polynomialValidateOwnership = false
