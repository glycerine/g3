// Package lp ports z3's util/lp linear programming solvers.
//
// Floating-point products that feed an addition or subtraction are wrapped
// in an explicit float64() conversion. The conversion rounds the product,
// which keeps the Go compiler from contracting x*y+z into a fused
// multiply-add (as it does on arm64, ppc64le, s390x, riscv64 and amd64 v3),
// so results match z3 bit for bit on all platforms.
package lp
