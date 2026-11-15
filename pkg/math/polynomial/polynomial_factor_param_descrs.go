package polynomial

import uparams "github.com/glycerine/g3/pkg/util/params"

// FactorParamsGetParamDescrs is polynomial::factor_params::get_param_descrs
// (src/math/polynomial/polynomial.cpp:58-62).
func FactorParamsGetParamDescrs(r *uparams.ParamDescrs) error {
	for _, d := range [3]struct{ name, descr string }{
		{"max_search_size", "(default: infty) Z3 polynomial factorization is composed of three steps: factorization in GF(p), lifting and search. This parameter can be used to limit the search space."},
		{"max_prime", "(default: infty) Z3 polynomial factorization is composed of three steps: factorization in GF(p), lifting and search. This parameter limits the maximum prime number p to be used in the first step."},
		{"num_primes", "(default: 1) Z3 polynomial factorization is composed of three steps: factorization in GF(p), lifting and search. The search space may be reduced by factoring the polynomial in different GF(p)'s. This parameter specify the maximum number of finite factorizations to be considered, before lifiting and searching."},
	} {
		descr := d.descr
		if err := r.InsertName(d.name, uparams.ParamUInt, &descr, nil, nil); err != nil {
			return err
		}
	}
	return nil
}
