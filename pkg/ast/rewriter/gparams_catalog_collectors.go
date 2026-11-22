package rewriter

// Parameter collectors remain with the C++ definitions they translate.

import (
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Source: src/ast/rewriter/arith_rewriter_params.pyg
var arithRewriterParamsDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "algebraic_number_evaluator", Kind: uparams.ParamBool, Description: "simplify/evaluate expressions containing (algebraic) irrational numbers.", Default: "true", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "mul_to_power", Kind: uparams.ParamBool, Description: "collpase (* t ... t) into (^ t k), it is ignored if expand_power is true.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "expand_power", Kind: uparams.ParamBool, Description: "expand (^ t k) into (* t ... t) if  1 < k <= max_degree.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "expand_tan", Kind: uparams.ParamBool, Description: "replace (tan x) with (/ (sin x) (cos x)).", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "max_degree", Kind: uparams.ParamUInt, Description: "max degree of algebraic numbers (and power operators) processed by simplifier.", Default: "64", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "sort_sums", Kind: uparams.ParamBool, Description: "sort the arguments of + application.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "gcd_rounding", Kind: uparams.ParamBool, Description: "use gcd rounding on integer arithmetic atoms.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "arith_lhs", Kind: uparams.ParamBool, Description: "all monomials are moved to the left-hand-side, and the right-hand-side is just a constant.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "arith_ineq_lhs", Kind: uparams.ParamBool, Description: "rewrite inequalities so that right-hand-side is a constant.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "elim_to_real", Kind: uparams.ParamBool, Description: "eliminate to_real from arithmetic predicates that contain only integers.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "push_to_real", Kind: uparams.ParamBool, Description: "distribute to_real over * and +.", Default: "true", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "eq2ineq", Kind: uparams.ParamBool, Description: "expand equalities into two inequalities", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "elim_rem", Kind: uparams.ParamBool, Description: "replace (rem x y) with (ite (>= y 0) (mod x y) (- (mod x y))).", Default: "false", Module: "rewriter"},
)

func catalogCollectArithRewriterParams(d *uparams.ParamDescrs) error {
	return d.InsertGroup(arithRewriterParamsDescriptions)
}

// Source: src/ast/rewriter/array_rewriter_params.pyg
var arrayRewriterParamsDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "expand_select_store", Kind: uparams.ParamBool, Description: "replace a (select (store ...) ...) term by an if-then-else term", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "expand_store_eq", Kind: uparams.ParamBool, Description: "reduce (store ...) = (store ...) with a common base into selects", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "sort_store", Kind: uparams.ParamBool, Description: "sort nested stores when the indices are known to be different", Default: "false", Module: "rewriter"},
)

func catalogCollectArrayRewriterParams(d *uparams.ParamDescrs) error {
	return d.InsertGroup(arrayRewriterParamsDescriptions)
}

// Source: src/ast/rewriter/bool_rewriter_params.pyg
var boolRewriterParamsDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "ite_extra_rules", Kind: uparams.ParamBool, Description: "extra ite simplifications, these additional simplifications may reduce size locally but increase globally", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "flat", Kind: uparams.ParamBool, Description: "create nary applications for and,or,+,*,bvadd,bvmul,bvand,bvor,bvxor", Default: "true", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "elim_and", Kind: uparams.ParamBool, Description: "conjunctions are rewritten using negation and disjunctions", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "local_ctx", Kind: uparams.ParamBool, Description: "perform local (i.e., cheap) context simplifications", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "local_ctx_limit", Kind: uparams.ParamUInt, Description: "limit for applying local context simplifier", Default: "4294967295", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "blast_distinct", Kind: uparams.ParamBool, Description: "expand a distinct predicate into a quadratic number of disequalities", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "blast_distinct_threshold", Kind: uparams.ParamUInt, Description: "when blast_distinct is true, only distinct expressions with less than this number of arguments are blasted", Default: "4294967295", Module: "rewriter"},
)

func catalogCollectBoolRewriterParams(d *uparams.ParamDescrs) error {
	return d.InsertGroup(boolRewriterParamsDescriptions)
}

// Source: src/ast/rewriter/bv_rewriter_params.pyg
var bvRewriterParamsDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "udiv2mul", Kind: uparams.ParamBool, Description: "convert constant udiv to mul", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "split_concat_eq", Kind: uparams.ParamBool, Description: "split equalities of the form (= (concat t1 t2) t3)", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "bit2bool", Kind: uparams.ParamBool, Description: "try to convert bit-vector terms of size 1 into Boolean terms", Default: "true", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "blast_eq_value", Kind: uparams.ParamBool, Description: "blast (some) Bit-vector equalities into bits", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "elim_sign_ext", Kind: uparams.ParamBool, Description: "expand sign-ext operator using concat and extract", Default: "true", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "hi_div0", Kind: uparams.ParamBool, Description: "use the 'hardware interpretation' for division by zero (for bit-vector terms)", Default: "true", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "mul2concat", Kind: uparams.ParamBool, Description: "replace multiplication by a power of two into a concatenation", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "bvnot2arith", Kind: uparams.ParamBool, Description: "replace (bvnot x) with (bvsub -1 x)", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "bv_sort_ac", Kind: uparams.ParamBool, Description: "sort the arguments of all AC operators", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "bv_trailing", Kind: uparams.ParamBool, Description: "lean removal of trailing zeros", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "bv_extract_prop", Kind: uparams.ParamBool, Description: "attempt to partially propagate extraction inwards", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "bv_not_simpl", Kind: uparams.ParamBool, Description: "apply simplifications for bvnot", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "bv_ite2id", Kind: uparams.ParamBool, Description: "rewrite ite that can be simplified to identity", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "bv_le_extra", Kind: uparams.ParamBool, Description: "additional bu_(u/s)le simplifications", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "bv_urem_simpl", Kind: uparams.ParamBool, Description: "additional simplification for bvurem", Default: "false", Module: "rewriter"},
)

func catalogCollectBvRewriterParams(d *uparams.ParamDescrs) error {
	return d.InsertGroup(bvRewriterParamsDescriptions)
}

// Source: src/ast/rewriter/fpa_rewriter_params.pyg
var fpaRewriterParamsDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "hi_fp_unspecified", Kind: uparams.ParamBool, Description: "use the 'hardware interpretation' for unspecified values in fp.to_ubv, fp.to_sbv, fp.to_real, and fp.to_ieee_bv", Default: "false", Module: "rewriter"},
)

func catalogCollectFpaRewriterParams(d *uparams.ParamDescrs) error {
	return d.InsertGroup(fpaRewriterParamsDescriptions)
}

// Source: src/ast/rewriter/poly_rewriter_params.pyg
var polyRewriterParamsDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "som", Kind: uparams.ParamBool, Description: "put polynomials in som-of-monomials form", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "som_blowup", Kind: uparams.ParamUInt, Description: "maximum number of monomials generated when putting a polynomial in sum-of-monomials normal form", Default: "4294967295", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "hoist_mul", Kind: uparams.ParamBool, Description: "hoist multiplication over summation to minimize number of multiplications", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "hoist_cmul", Kind: uparams.ParamBool, Description: "hoist constant multiplication over summation to minimize number of multiplications", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "flat", Kind: uparams.ParamBool, Description: "create nary applications for and,or,+,*,bvadd,bvmul,bvand,bvor,bvxor", Default: "true", Module: "rewriter"},
)

func catalogCollectPolyRewriterParams(d *uparams.ParamDescrs) error {
	return d.InsertGroup(polyRewriterParamsDescriptions)
}

// Source: src/ast/rewriter/rewriter_params.pyg
var rewriterParamsDescriptions = uparams.NewParamDescriptionGroup(
	uparams.NamedParamDescription{Name: "max_memory", Kind: uparams.ParamUInt, Description: "maximum amount of memory in megabytes", Default: "4294967295", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "max_steps", Kind: uparams.ParamUInt, Description: "maximum number of steps", Default: "4294967295", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "flat", Kind: uparams.ParamBool, Description: "create nary applications for and,or,+,*,bvadd,bvmul,bvand,bvor,bvxor", Default: "true", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "push_ite_arith", Kind: uparams.ParamBool, Description: "push if-then-else over arithmetic terms.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "push_ite_bv", Kind: uparams.ParamBool, Description: "push if-then-else over bit-vector terms.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "pull_cheap_ite", Kind: uparams.ParamBool, Description: "pull if-then-else terms when cheap.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "bv_ineq_consistency_test_max", Kind: uparams.ParamUInt, Description: "max size of conjunctions on which to perform consistency test based on inequalities on bitvectors.", Default: "0", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "cache_all", Kind: uparams.ParamBool, Description: "cache all intermediate results.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "rewrite_patterns", Kind: uparams.ParamBool, Description: "rewrite patterns.", Default: "false", Module: "rewriter"},
	uparams.NamedParamDescription{Name: "ignore_patterns_on_ground_qbody", Kind: uparams.ParamBool, Description: "ignores patterns on quantifiers that don't mention their bound variables.", Default: "true", Module: "rewriter"},
)

func catalogCollectRewriterParams(d *uparams.ParamDescrs) error {
	return d.InsertGroup(rewriterParamsDescriptions)
}

func init() {
	uparams.RegisterGlobalParamsCollector("catalogCollectArithRewriterParams", catalogCollectArithRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectArrayRewriterParams", catalogCollectArrayRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectBoolRewriterParams", catalogCollectBoolRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectBvRewriterParams", catalogCollectBvRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectFpaRewriterParams", catalogCollectFpaRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectPolyRewriterParams", catalogCollectPolyRewriterParams)
	uparams.RegisterGlobalParamsCollector("catalogCollectRewriterParams", catalogCollectRewriterParams)
}
