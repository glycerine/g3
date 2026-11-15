(declare-const a (_ FloatingPoint 3 3))
(declare-const b (_ FloatingPoint 3 3))
(assert (fp.lt (fp.add RNE a b) b))
(apply (using-params (then fpa2bv (or-else skip simplify)) :max_steps 1))
